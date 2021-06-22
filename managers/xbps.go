package managers

type xbps struct {
	common
}

func (m *xbps) load() error {
	m.commands = managerCommands{
		clean:   "xbps-remove",
		install: "xbps-install",
		refresh: "xbps-install",
		remove:  "xbps-remove",
		update:  "sh",
	}

	m.flags = managerFlags{
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
	}

	return nil
}
