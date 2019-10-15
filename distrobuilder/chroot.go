package main

import (
	"fmt"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

func manageRepositories(def *shared.Definition, manager *managers.Manager) error {
	var err error

	if def.Packages.Repositories == nil || len(def.Packages.Repositories) == 0 {
		return nil
	}

	// Handle repositories actions
	if manager.RepoHandler == nil {
		return fmt.Errorf("No repository handler present")
	}

	for _, repo := range def.Packages.Repositories {
		if !shared.ApplyFilter(&repo, def.Image.Release, def.Image.ArchitectureMapped, def.Image.Variant) {
			continue
		}

		// Run template on repo.URL
		repo.URL, err = shared.RenderTemplate(repo.URL, def)
		if err != nil {
			return err
		}

		err = manager.RepoHandler(repo)
		if err != nil {
			return fmt.Errorf("Error for repository %s: %s", repo.Name, err)
		}
	}

	return nil
}

func managePackages(def *shared.Definition, manager *managers.Manager) error {
	var err error

	err = manager.Refresh()
	if err != nil {
		return err
	}

	if def.Packages.Update {
		err = manager.Update()
		if err != nil {
			return err
		}

		// Run post update hook
		for _, action := range def.GetRunnableActions("post-update") {
			err = shared.RunScript(action.Action)
			if err != nil {
				return fmt.Errorf("Failed to run post-update: %s", err)
			}
		}
	}

	var validSets []shared.DefinitionPackagesSet

	for _, set := range def.Packages.Sets {
		if !shared.ApplyFilter(&set, def.Image.Release, def.Image.ArchitectureMapped, def.Image.Variant) {
			continue
		}

		validSets = append(validSets, set)
	}

	for _, set := range optimizePackageSets(validSets) {
		if set.Action == "install" {
			err = manager.Install(set.Packages)
		} else if set.Action == "remove" {
			err = manager.Remove(set.Packages)
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

	for i := 1; i < len(sets); i++ {
		if sets[i].Action == sets[i-1].Action {
			packages = append(packages, sets[i].Packages...)
		} else {
			newSets = append(newSets, shared.DefinitionPackagesSet{
				Action:   action,
				Packages: packages,
			})

			action = sets[i].Action
			packages = sets[i].Packages
		}
	}

	newSets = append(newSets, shared.DefinitionPackagesSet{
		Action:   action,
		Packages: packages,
	})

	return newSets
}
