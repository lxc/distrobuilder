package managers

// NewEgoPortage creates a new Manager instance.
func NewEgoPortage() *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   "emerge",
			install: "emerge",
			refresh: "ego",
			remove:  "emerge",
			update:  "emerge",
		},
		flags: ManagerFlags{
			global: []string{},
			clean:  []string{},
			install: []string{
				"--autounmask-continue",
				"--quiet-build=y",
			},
			remove: []string{
				"--unmerge",
			},
			refresh: []string{
				"sync",
			},
			update: []string{
				"--update", "@world",
			},
		},
	}
}
