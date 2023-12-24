package managers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	incus "github.com/lxc/incus/shared/util"

	"github.com/lxc/distrobuilder/shared"
)

type anise struct {
	common
}

func (m *anise) load() error {
	m.commands = managerCommands{
		clean:   "anise",
		install: "anise",
		refresh: "anise",
		remove:  "anise",
		update:  "anise",
	}

	m.flags = managerFlags{
		global: []string{},
		clean: []string{
			"cleanup", "--purge-repos",
		},
		install: []string{
			// Forcing always override of the protected
			// files. Not needed on image creation.
			"install", "--skip-config-protect",
		},
		refresh: []string{
			"repo", "update", "--force",
		},
		remove: []string{
			"uninstall", "--skip-config-protect",
		},
		update: []string{
			"upgrade", "--sync-repos", "--skip-config-protect",
		},
	}

	return nil
}

func (m *anise) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	var targetFile string

	if repoAction.Name == "" {
		return errors.New("Invalid repository name")
	}

	if repoAction.URL == "" {
		return errors.New("Invalid repository url")
	}

	if strings.HasSuffix(repoAction.Name, ".yml") {
		targetFile = filepath.Join("/etc/anise/repos.conf.d", repoAction.Name)
	} else {
		targetFile = filepath.Join("/etc/anise/repos.conf.d", repoAction.Name+".yml")
	}

	if !incus.PathExists(filepath.Dir(targetFile)) {
		err := os.MkdirAll(filepath.Dir(targetFile), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", filepath.Dir(targetFile), err)
		}
	}

	f, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("Failed to open file %q: %w", targetFile, err)
	}

	defer f.Close()

	// NOTE: repo.URL is not an URL but the content of the file.
	_, err = f.WriteString(repoAction.URL)
	if err != nil {
		return fmt.Errorf("Failed to write string to %q: %w", targetFile, err)
	}

	return nil
}
