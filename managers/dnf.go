package managers

// NewDnf creates a new Manager instance.
func NewDnf() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "dnf",
			install: "dnf",
			refresh: "dnf",
			remove:  "dnf",
			update:  "dnf",
		},
		flags: ManagerFlags{
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
				"upgrade",
			},
			clean: []string{
				"clean", "all",
			},
		},
	}
}
