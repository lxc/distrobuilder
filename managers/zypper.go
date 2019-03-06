package managers

// NewZypper create a new Manager instance.
func NewZypper() *Manager {
	return &Manager{
		command: "zypper",
		flags: ManagerFlags{
			global: []string{
				"--non-interactive",
				"--gpg-auto-import-keys",
			},
			clean: []string{
				"clean",
				"-a",
			},
			install: []string{
				"install",
			},
			remove: []string{
				"remove",
			},
			refresh: []string{
				"refresh",
			},
			update: []string{
				"update",
			},
		},
	}
}
