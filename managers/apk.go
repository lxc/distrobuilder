package managers

import (
	"os"

	"github.com/lxc/distrobuilder/shared"
	"github.com/pkg/errors"
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
		return errors.Wrapf(err, "Failed to open %q", repoFile)
	}
	defer f.Close()

	_, err = f.WriteString(repoAction.URL + "\n")
	if err != nil {
		return errors.Wrap(err, "Failed to write string to file")
	}

	return nil
}
