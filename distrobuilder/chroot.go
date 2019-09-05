package main

import (
	"fmt"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

func managePackages(def shared.DefinitionPackages, actions []shared.DefinitionAction,
	release string, architecture string, variant string) error {
	var err error
	var manager *managers.Manager

	if def.Manager != "" {
		manager = managers.Get(def.Manager)
		if manager == nil {
			return fmt.Errorf("Couldn't get manager")
		}
	} else {
		manager = managers.GetCustom(*def.CustomManager)
	}

	// Handle repositories actions
	if def.Repositories != nil && len(def.Repositories) > 0 {
		if manager.RepoHandler == nil {
			return fmt.Errorf("No repository handler present")
		}

		for _, repo := range def.Repositories {
			if len(repo.Releases) > 0 && !lxd.StringInSlice(release, repo.Releases) {
				continue
			}

			if len(repo.Architectures) > 0 && !lxd.StringInSlice(architecture, repo.Architectures) {
				continue
			}

			if len(repo.Variants) > 0 && !lxd.StringInSlice(variant, repo.Variants) {
				continue
			}

			err = manager.RepoHandler(repo)
			if err != nil {
				return fmt.Errorf("Error for repository %s: %s", repo.Name, err)
			}
		}
	}

	err = manager.Refresh()
	if err != nil {
		return err
	}

	if def.Update {
		err = manager.Update()
		if err != nil {
			return err
		}

		// Run post update hook
		for _, action := range actions {
			err = shared.RunScript(action.Action)
			if err != nil {
				return fmt.Errorf("Failed to run post-update: %s", err)
			}
		}
	}

	var validSets []shared.DefinitionPackagesSet

	for _, set := range def.Sets {
		if len(set.Releases) > 0 && !lxd.StringInSlice(release, set.Releases) {
			continue
		}

		if len(set.Architectures) > 0 && !lxd.StringInSlice(architecture, set.Architectures) {
			continue
		}

		if len(set.Variants) > 0 && !lxd.StringInSlice(variant, set.Variants) {
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

	if def.Cleanup {
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
