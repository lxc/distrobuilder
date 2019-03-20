package managers

// NewApt creates a new Manager instance.
func NewApt() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "apt-get",
			install: "apt-get",
			refresh: "apt-get",
			remove:  "apt-get",
			update:  "apt-get",
		},
		flags: ManagerFlags{
			clean: []string{
				"clean",
			},
			global: []string{
				"-y",
			},
			install: []string{
				"install",
			},
			remove: []string{
				"remove", "--auto-remove",
			},
			refresh: []string{
				"update",
			},
			update: []string{
				"dist-upgrade",
			},
		},
	}
}
