package managers

type egoportage struct {
	common
}

func (m *egoportage) load() error {
	m.commands = managerCommands{
		clean:   "emerge",
		install: "emerge",
		refresh: "ego",
		remove:  "emerge",
		update:  "emerge",
	}

	m.flags = managerFlags{
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
	}

	return nil
}
