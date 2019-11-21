package managers

import (
	"os"

	"github.com/lxc/distrobuilder/shared"
)

// NewApk creates a new Manager instance.
func NewApk() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "apk",
			install: "apk",
			refresh: "apk",
			remove:  "apk",
			update:  "apk",
		},
		flags: ManagerFlags{
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
		},
		RepoHandler: func(repoAction shared.DefinitionPackagesRepository) error {
			f, err := os.OpenFile("/etc/apk/repositories", os.O_WRONLY|os.O_APPEND, 0644)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = f.WriteString(repoAction.URL + "\n")
			if err != nil {
				return err
			}

			return nil
		},
	}
}
