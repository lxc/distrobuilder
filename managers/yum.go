package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// NewYum creates a new Manager instance.
func NewYum() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "yum",
			install: "yum",
			refresh: "yum",
			remove:  "yum",
			update:  "yum",
		},
		flags: ManagerFlags{
			clean: []string{
				"clean", "all",
			},
			global: []string{
				"-y",
			},
			install: []string{
				"install",
			},
			remove: []string{
				"remove",
			},
			refresh: []string{
				"makecache",
			},
			update: []string{
				"update",
			},
		},
		RepoHandler: yumRepoHandler,
	}
}

func yumRepoHandler(repoAction shared.DefinitionPackagesRepository) error {
	targetFile := filepath.Join("/etc/yum.repos.d", repoAction.Name)

	if !strings.HasSuffix(targetFile, ".repo") {
		targetFile = fmt.Sprintf("%s.repo", targetFile)
	}

	if !lxd.PathExists(filepath.Dir(targetFile)) {
		err := os.MkdirAll(filepath.Dir(targetFile), 0755)
		if err != nil {
			return err
		}
	}

	f, err := os.Create(targetFile)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(repoAction.URL)
	if err != nil {
		return err
	}

	// Append final new line if missing
	if !strings.HasSuffix(repoAction.URL, "\n") {
		_, err = f.WriteString("\n")
		if err != nil {
			return err
		}
	}

	return nil
}
