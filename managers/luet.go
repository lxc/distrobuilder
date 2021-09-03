package managers

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type luet struct {
	common
}

func (m *luet) load() error {
	m.commands = managerCommands{
		clean:   "luet",
		install: "luet",
		refresh: "luet",
		remove:  "luet",
		update:  "luet",
	}

	m.flags = managerFlags{
		global: []string{},
		clean: []string{
			"cleanup",
		},
		install: []string{
			"install",
		},
		refresh: []string{
			"repo", "update",
		},
		remove: []string{
			"uninstall",
		},
		update: []string{
			"upgrade",
		},
	}

	return nil
}

func (m *luet) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	var targetFile string

	if repoAction.Name == "" {
		return errors.New("Invalid repository name")
	}

	if repoAction.URL == "" {
		return errors.New("Invalid repository url")
	}

	if strings.HasSuffix(repoAction.Name, ".yml") {
		targetFile = filepath.Join("/etc/luet/repos.conf.d", repoAction.Name)
	} else {
		targetFile = filepath.Join("/etc/luet/repos.conf.d", repoAction.Name+".yml")
	}

	if !lxd.PathExists(filepath.Dir(targetFile)) {
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
