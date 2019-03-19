package managers

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
	}
}
