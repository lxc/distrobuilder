package managers

import (
	"fmt"

	"github.com/lxc/distrobuilder/shared"
)

func enmanRepoCaller(repo shared.DefinitionPackagesRepository) error {
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

func equoRepoCaller(repo shared.DefinitionPackagesRepository) error {
	if repo.Name == "" {
		return fmt.Errorf("Invalid repository name")
	}

	if repo.URL == "" {
		return fmt.Errorf("Invalid repository url")
	}

	return shared.RunCommand("equo", "repo", "add", "--repo", repo.URL, "--pkg", repo.URL,
		repo.Name)
}

// NewEquo creates a new Manager instance
func NewEquo() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "equo",
			install: "equo",
			refresh: "equo",
			remove:  "equo",
			update:  "equo",
		},
		flags: ManagerFlags{
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
		},
		RepoHandler: func(repoAction shared.DefinitionPackagesRepository) error {
			if repoAction.Type == "" || repoAction.Type == "equo" {
				return equoRepoCaller(repoAction)
			} else if repoAction.Type == "enman" {
				return enmanRepoCaller(repoAction)
			}

			return fmt.Errorf("Invalid repository Type")
		},
	}
}
