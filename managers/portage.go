package managers

type portage struct {
	common
}

func (m *portage) load() error {
	m.commands = managerCommands{
		clean:   "emerge",
		install: "emerge",
		refresh: "true",
		remove:  "emerge",
		update:  "emerge",
	}

	m.flags = managerFlags{
		global: []string{},
		clean:  []string{},
		install: []string{
			"--autounmask-continue",
		},
		remove: []string{
			"--unmerge",
		},
		refresh: []string{},
		update: []string{
			"--update", "@world",
		},
	}

	return nil
}
