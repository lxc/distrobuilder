package managers

// NewApk creates a new Manager instance.
func NewApk() *Manager {
	return &Manager{
		command: "apk",
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
