package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

func luetRepoCaller(repo shared.DefinitionPackagesRepository) error {
	var targetFile string

	if repo.Name == "" {
		return fmt.Errorf("Invalid repository name")
	}

	if repo.URL == "" {
		return fmt.Errorf("Invalid repository url")
	}

	if strings.HasSuffix(repo.Name, ".yml") {
		targetFile = filepath.Join("/etc/luet/repos.conf.d", repo.Name)
	} else {
		targetFile = filepath.Join("/etc/luet/repos.conf.d", repo.Name+".yml")
	}

	if !lxd.PathExists(filepath.Dir(targetFile)) {
		err := os.MkdirAll(filepath.Dir(targetFile), 0755)
		if err != nil {
			return err
		}
	}

	f, err := os.OpenFile(targetFile, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// NOTE: repo.URL is not an URL but the content of the file.
	_, err = f.WriteString(repo.URL)
	if err != nil {
		return err
	}

	return nil
}

// NewLuet create a new Manager instance
func NewLuet() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "luet",
			install: "luet",
			refresh: "luet",
			remove:  "luet",
			update:  "luet",
		},
		flags: ManagerFlags{
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
		},
		RepoHandler: func(repoAction shared.DefinitionPackagesRepository) error {
			return luetRepoCaller(repoAction)
		},
	}
}
