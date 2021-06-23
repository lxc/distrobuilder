package managers

type custom struct {
	common
}

func (m *custom) load() error {
	m.commands = managerCommands{
		clean:   m.definition.Packages.CustomManager.Clean.Command,
		install: m.definition.Packages.CustomManager.Install.Command,
		refresh: m.definition.Packages.CustomManager.Refresh.Command,
		remove:  m.definition.Packages.CustomManager.Remove.Command,
		update:  m.definition.Packages.CustomManager.Update.Command,
	}

	m.flags = managerFlags{
		clean:   m.definition.Packages.CustomManager.Clean.Flags,
		install: m.definition.Packages.CustomManager.Install.Flags,
		refresh: m.definition.Packages.CustomManager.Refresh.Flags,
		remove:  m.definition.Packages.CustomManager.Remove.Flags,
		update:  m.definition.Packages.CustomManager.Update.Flags,
		global:  m.definition.Packages.CustomManager.Flags,
	}

	return nil
}
