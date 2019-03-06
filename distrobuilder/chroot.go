package main

import (
	"fmt"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

func managePackages(def shared.DefinitionPackages, actions []shared.DefinitionAction,
	release string, architecture string) error {
	var err error

	manager := managers.Get(def.Manager)
	if manager == nil {
		return fmt.Errorf("Couldn't get manager")
	}

	// Handle repositories actions
	if def.Repositories != nil && len(def.Repositories) > 0 {
		if manager.RepoHandler == nil {
			return fmt.Errorf("No repository handler present")
		}

		for _, repo := range def.Repositories {
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

	for _, set := range def.Sets {
		if len(set.Releases) > 0 && !lxd.StringInSlice(release, set.Releases) {
			continue
		}

		if len(set.Architectures) > 0 && !lxd.StringInSlice(architecture, set.Architectures) {
			continue
		}

		if set.Action == "install" {
			err = manager.Install(set.Packages)
		} else if set.Action == "remove" {
			err = manager.Remove(set.Packages)
		}
		if err != nil {
			return err
		}
	}

	// TODO: Remove this once openSUSE builds properly without it. OpenSUSE 42.3 doesn't support this flag.
	if release != "42.3" {
		manager.SetInstallFlags("install", "--allow-downgrade")
	}

	if def.Cleanup {
		err = manager.Clean()
		if err != nil {
			return err
		}
	}

	return nil
}
