package managers

import (
	"fmt"

	"github.com/lxc/distrobuilder/shared"
)

func zypperRepoCaller(repo shared.DefinitionPackagesRepository) error {
	if repo.Name == "" {
		return fmt.Errorf("Invalid repository name")
	}

	if repo.URL == "" {
		return fmt.Errorf("Invalid repository url")
	}

	return shared.RunCommand("zypper", "ar", "--refresh", "--check", repo.URL, repo.Name)
}

// NewZypper create a new Manager instance.
func NewZypper() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "zypper",
			install: "zypper",
			refresh: "zypper",
			remove:  "zypper",
			update:  "zypper",
		},
		flags: ManagerFlags{
			global: []string{
				"--non-interactive",
				"--gpg-auto-import-keys",
			},
			clean: []string{
				"clean",
				"-a",
			},
			install: []string{
				"install",
				"--allow-downgrade",
			},
			remove: []string{
				"remove",
			},
			refresh: []string{
				"refresh",
			},
			update: []string{
				"update",
			},
		},
		RepoHandler: func(repoAction shared.DefinitionPackagesRepository) error {
			if repoAction.Type == "" || repoAction.Type == "zypper" {
				return zypperRepoCaller(repoAction)
			}
			return fmt.Errorf("Invalid repository Type")
		},
	}
}
