package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

func manageRepositories(def *shared.Definition, manager *managers.Manager, imageTarget shared.ImageTarget) error {
	var err error

	if def.Packages.Repositories == nil || len(def.Packages.Repositories) == 0 {
		return nil
	}

	// Handle repositories actions
	if manager.RepoHandler == nil {
		return fmt.Errorf("No repository handler present")
	}

	for _, repo := range def.Packages.Repositories {
		if !shared.ApplyFilter(&repo, def.Image.Release, def.Image.ArchitectureMapped, def.Image.Variant, def.Targets.Type, imageTarget) {
			continue
		}

		// Run template on repo.URL
		repo.URL, err = shared.RenderTemplate(repo.URL, def)
		if err != nil {
			return err
		}

		// Run template on repo.Key
		repo.Key, err = shared.RenderTemplate(repo.Key, def)
		if err != nil {
			return err
		}

		err = manager.RepoHandler(repo)
		if err != nil {
			return errors.Wrapf(err, "Error for repository %s", repo.Name)
		}
	}

	return nil
}

func managePackages(def *shared.Definition, manager *managers.Manager, imageTarget shared.ImageTarget) error {
	var validSets []shared.DefinitionPackagesSet

	for _, set := range def.Packages.Sets {
		if !shared.ApplyFilter(&set, def.Image.Release, def.Image.ArchitectureMapped, def.Image.Variant, def.Targets.Type, imageTarget) {
			continue
		}

		validSets = append(validSets, set)
	}

	// If there's nothing to install or remove, and no updates need to be performed,
	// we can exit here.
	if len(validSets) == 0 && !def.Packages.Update {
		return nil
	}

	err := manager.Refresh()
	if err != nil {
		return err
	}

	if def.Packages.Update {
		err = manager.Update()
		if err != nil {
			return err
		}

		// Run post update hook
		for _, action := range def.GetRunnableActions("post-update", imageTarget) {
			err = shared.RunScript(action.Action)
			if err != nil {
				return errors.Wrap(err, "Failed to run post-update")
			}
		}
	}

	for _, set := range optimizePackageSets(validSets) {
		if set.Action == "install" {
			err = manager.Install(set.Packages, set.Flags)
		} else if set.Action == "remove" {
			err = manager.Remove(set.Packages, set.Flags)
		}
		if err != nil {
			return err
		}
	}

	if def.Packages.Cleanup {
		err = manager.Clean()
		if err != nil {
			return err
		}
	}

	return nil
}

// optimizePackageSets groups consecutive package sets with the same action to
// reduce the amount of calls to manager.{Install,Remove}(). It still honors the
// order of execution.
func optimizePackageSets(sets []shared.DefinitionPackagesSet) []shared.DefinitionPackagesSet {
	if len(sets) < 2 {
		return sets
	}

	var newSets []shared.DefinitionPackagesSet

	action := sets[0].Action
	packages := sets[0].Packages
	flags := sets[0].Flags

	for i := 1; i < len(sets); i++ {
		if sets[i].Action == sets[i-1].Action && strings.Join(sets[i].Flags, " ") == strings.Join(sets[i-1].Flags, " ") {
			packages = append(packages, sets[i].Packages...)
		} else {
			newSets = append(newSets, shared.DefinitionPackagesSet{
				Action:   action,
				Packages: packages,
				Flags:    flags,
			})

			action = sets[i].Action
			packages = sets[i].Packages
			flags = sets[i].Flags
		}
	}

	newSets = append(newSets, shared.DefinitionPackagesSet{
		Action:   action,
		Packages: packages,
		Flags:    flags,
	})

	return newSets
}

func getOverlay(cacheDir, sourceDir string) (func(), string, error) {
	upperDir := filepath.Join(cacheDir, "upper")
	overlayDir := filepath.Join(cacheDir, "overlay")
	workDir := filepath.Join(cacheDir, "work")

	err := os.Mkdir(upperDir, 0755)
	if err != nil {
		return nil, "", err
	}

	err = os.Mkdir(overlayDir, 0755)
	if err != nil {
		return nil, "", err
	}

	err = os.Mkdir(workDir, 0755)
	if err != nil {
		return nil, "", err
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", sourceDir, upperDir, workDir)

	err = unix.Mount("overlay", overlayDir, "overlay", 0, opts)
	if err != nil {
		return nil, "", err
	}

	cleanup := func() {
		err := unix.Unmount(overlayDir, 0)
		if err != nil {
			fmt.Fprintln(os.Stderr, errors.Wrap(err, "Failed to unmount overlay"))
		}

		err = os.RemoveAll(upperDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, errors.Wrap(err, "Failed to remove upper directory"))
		}

		err = os.RemoveAll(workDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, errors.Wrap(err, "Failed to remove work directory"))
		}

		err = os.Remove(overlayDir)
		if err != nil {
			fmt.Fprintln(os.Stderr, errors.Wrap(err, "Failed to remove overlay directory"))
		}
	}

	return cleanup, overlayDir, nil
}
