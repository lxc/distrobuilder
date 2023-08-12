package managers

type slackpkg struct {
	common
}

func (m *slackpkg) load() error {
	m.commands = managerCommands{
		install: "slackpkg",
		remove:  "slackpkg",
		refresh: "slackpkg",
		update:  "true",
		clean:   "true",
	}

	m.flags = managerFlags{
		global: []string{
			"-batch=on", "-default_answer=y",
		},
		install: []string{
			"install",
		},
		remove: []string{
			"remove",
		},
		refresh: []string{
			"update",
		},
		update: []string{
			"upgrade-all",
		},
		clean: []string{
			"clean-system",
		},
	}

	return nil
}
