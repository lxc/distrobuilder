package managers

import (
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type zypper struct {
	common
}

func (m *zypper) load() error {
	m.commands = managerCommands{
		clean:   "zypper",
		install: "zypper",
		refresh: "zypper",
		remove:  "zypper",
		update:  "zypper",
	}

	m.flags = managerFlags{
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
			"--replacefiles",
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
	}

	return nil
}

func (m *zypper) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	if repoAction.Type != "" && repoAction.Type != "zypper" {
		return errors.New("Invalid repository Type")
	}

	if repoAction.Name == "" {
		return errors.New("Invalid repository name")
	}

	if repoAction.URL == "" {
		return errors.New("Invalid repository url")
	}

	return shared.RunCommand("zypper", "ar", "--refresh", "--check", repoAction.URL, repoAction.Name)
}
