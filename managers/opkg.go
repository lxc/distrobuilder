package managers

import "os"

type opkg struct {
	common
}

func (m *opkg) load() error {
	m.commands = managerCommands{
		clean:   "rm",
		install: "opkg",
		refresh: "opkg",
		remove:  "opkg",
		update:  "echo",
	}

	m.flags = managerFlags{
		clean: []string{
			"-rf", "/tmp/opkg-lists/",
		},
		global: []string{},
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
			"Not supported",
		},
	}

	m.hooks = managerHooks{
		preRefresh: func() error {
			return os.MkdirAll("/var/lock", 0755)
		},
	}

	return nil
}
