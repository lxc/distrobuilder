package managers

import (
	"fmt"
	"os"

	"github.com/lxc/distrobuilder/shared"
)

type apk struct {
	common
}

func (m *apk) load() error {
	m.commands = managerCommands{
		clean:   "apk",
		install: "apk",
		refresh: "apk",
		remove:  "apk",
		update:  "apk",
	}

	m.flags = managerFlags{
		global: []string{
			"--no-cache",
		},
		install: []string{
			"add",
		},
		remove: []string{
			"del", "--rdepends",
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

func (m *apk) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	repoFile := "/etc/apk/repositories"

	f, err := os.OpenFile(repoFile, os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("Failed to open %q: %w", repoFile, err)
	}
	defer f.Close()

	_, err = f.WriteString(repoAction.URL + "\n")
	if err != nil {
		return fmt.Errorf("Failed to write string to file: %w", err)
	}

	return nil
}
