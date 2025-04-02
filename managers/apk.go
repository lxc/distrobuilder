package managers

import (
	"fmt"
	"os"
	"strings"

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
	err := m.appendRepositoryURL(repoAction)
	if err != nil {
		return err
	}

	err = m.writeKeyFile(repoAction)
	if err != nil {
		return err
	}

	return nil
}

func (m *apk) appendRepositoryURL(repoAction shared.DefinitionPackagesRepository) error {
	if repoAction.URL == "" {
		return nil
	}

	repoFile := "/etc/apk/repositories"

	f, err := os.OpenFile(repoFile, os.O_WRONLY|os.O_APPEND, 0o644)
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

func (m *apk) writeKeyFile(repoAction shared.DefinitionPackagesRepository) error {
	if repoAction.Key == "" || repoAction.Name == "" {
		return nil
	}

	if strings.Contains(repoAction.Name, "/") {
		return fmt.Errorf("Invalid key file name: %q", repoAction.Name)
	}

	keyFile := "/etc/apk/keys/" + repoAction.Name
	f, err := os.OpenFile(keyFile, os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("Failed to open %q: %w", keyFile, err)
	}

	defer f.Close()

	_, err = f.WriteString(repoAction.Key + "\n")
	if err != nil {
		return fmt.Errorf("Failed to write %q: %w", keyFile, err)
	}

	return nil
}
