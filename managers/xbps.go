package managers

// NewXbps creates a new Manager instance.
func NewXbps() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "xbps-remove",
			install: "xbps-install",
			refresh: "xbps-install",
			remove:  "xbps-remove",
			update:  "sh",
		},
		flags: ManagerFlags{
			global: []string{},
			clean: []string{
				"--yes",
				"--clean-cache",
			},
			install: []string{
				"--yes",
			},
			refresh: []string{
				"--sync",
			},
			remove: []string{
				"--yes",
				"--recursive",
				"--remove-orphans",
			},
			update: []string{
				"-c",
				"xbps-install --yes --update && xbps-install --yes --update",
			},
		},
	}
}
