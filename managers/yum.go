package managers

// NewYum creates a new Manager instance.
func NewYum() *Manager {
	return &Manager{
		command: "yum",
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
	}
}
