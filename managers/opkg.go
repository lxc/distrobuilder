package managers

import "os"

// NewOpkg creates a new Manager instance.
func NewOpkg() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "rm",
			install: "opkg",
			refresh: "opkg",
			remove:  "opkg",
			update:  "echo",
		},
		flags: ManagerFlags{
			clean: []string{
				"-rf", "/tmp/opkg-lists/",
			},
			global: []string{},
			install: []string{
				"install",
			},
			remove: []string{
				"remove",
			},
			refresh: []string{
				"update",
			},
			update: []string{
				"Not supported",
			},
		},
		hooks: ManagerHooks{
			preRefresh: func() error {
				return os.MkdirAll("/var/lock", 0755)
			},
		},
	}
}
