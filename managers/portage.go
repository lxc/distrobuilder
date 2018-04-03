package managers

// NewPortage creates a new Manager instance.
func NewPortage() *Manager {
	return &Manager{
		command: "emerge",
		flags: ManagerFlags{
			global: []string{},
			clean:  []string{},
			install: []string{
				"--autounmask-continue",
			},
			remove: []string{
				"--unmerge",
			},
			refresh: []string{
				"--sync",
			},
			update: []string{
				"--update", "@world",
			},
		},
	}
}
