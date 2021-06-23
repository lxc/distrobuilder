package managers

import (
	"fmt"

	"github.com/lxc/distrobuilder/shared"
)

type equo struct {
	common
}

func (m *equo) load() error {
	m.commands = managerCommands{
		clean:   "equo",
		install: "equo",
		refresh: "equo",
		remove:  "equo",
		update:  "equo",
	}

	m.flags = managerFlags{
		global: []string{},
		clean: []string{
			"cleanup",
		},
		install: []string{
			"install",
		},
		remove: []string{
			"remove",
		},
		refresh: []string{
			"update",
		},
		update: []string{
			"upgrade",
		},
	}

	return nil
}

func (m *equo) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	if repoAction.Type == "" || repoAction.Type == "equo" {
		return m.equoRepoCaller(repoAction)
	} else if repoAction.Type == "enman" {
		return m.enmanRepoCaller(repoAction)
	}

	return fmt.Errorf("Invalid repository Type")
}

func (m *equo) enmanRepoCaller(repo shared.DefinitionPackagesRepository) error {
	args := []string{
		"add",
	}

	if repo.Name == "" && repo.URL == "" {
		return fmt.Errorf("Missing both repository url and repository name")
	}

	if repo.URL != "" {
		args = append(args, repo.URL)
	} else {
		args = append(args, repo.Name)
	}

	return shared.RunCommand("enman", args...)
}

func (m *equo) equoRepoCaller(repo shared.DefinitionPackagesRepository) error {
	if repo.Name == "" {
		return fmt.Errorf("Invalid repository name")
	}

	if repo.URL == "" {
		return fmt.Errorf("Invalid repository url")
	}

	return shared.RunCommand("equo", "repo", "add", "--repo", repo.URL, "--pkg", repo.URL,
		repo.Name)
}
