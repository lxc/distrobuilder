package managers

import (
	"github.com/lxc/distrobuilder/shared"
)

type dnf struct {
	common
}

// NewDnf creates a new Manager instance.
func (m *dnf) load() error {
	m.commands = managerCommands{
		clean:   "dnf",
		install: "dnf",
		refresh: "dnf",
		remove:  "dnf",
		update:  "dnf",
	}

	m.flags = managerFlags{
		global: []string{
			"-y",
		},
		install: []string{
			"install",
			"--nobest",
		},
		remove: []string{
			"remove",
		},
		refresh: []string{
			"makecache",
		},
		update: []string{
			"upgrade",
			"--nobest",
		},
		clean: []string{
			"clean", "all",
		},
	}

	return nil
}

func (m *dnf) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	return yumManageRepository(repoAction)
}
