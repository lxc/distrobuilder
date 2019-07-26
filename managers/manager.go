package managers

import "github.com/lxc/distrobuilder/shared"

// ManagerFlags represents flags for all subcommands of a package manager.
type ManagerFlags struct {
	global  []string
	install []string
	remove  []string
	clean   []string
	update  []string
	refresh []string
}

// ManagerHooks represents custom hooks.
type ManagerHooks struct {
	clean func() error
}

// ManagerCommands represents all commands.
type ManagerCommands struct {
	clean   string
	install string
	refresh string
	remove  string
	update  string
}

// A Manager represents a package manager.
type Manager struct {
	commands    ManagerCommands
	flags       ManagerFlags
	hooks       ManagerHooks
	RepoHandler func(repoAction shared.DefinitionPackagesRepository) error
}

// Get returns a Manager specified by name.
func Get(name string) *Manager {
	switch name {
	case "apk":
		return NewApk()
	case "apt":
		return NewApt()
	case "dnf":
		return NewDnf()
	case "egoportage":
		return NewEgoPortage()
	case "opkg":
		return NewOpkg()
	case "pacman":
		return NewPacman()
	case "portage":
		return NewPortage()
	case "xbps":
		return NewXbps()
	case "yum":
		return NewYum()
	case "equo":
		return NewEquo()
	case "zypper":
		return NewZypper()
	}

	return nil
}

// GetCustom returns a custom Manager specified by a Definition.
func GetCustom(def shared.DefinitionPackagesCustomManager) *Manager {
	return &Manager{
		commands: ManagerCommands{
			clean:   def.Clean.Command,
			install: def.Install.Command,
			refresh: def.Refresh.Command,
			remove:  def.Remove.Command,
			update:  def.Update.Command,
		},
		flags: ManagerFlags{
			clean:   def.Clean.Flags,
			install: def.Install.Flags,
			refresh: def.Refresh.Flags,
			remove:  def.Remove.Flags,
			update:  def.Update.Flags,
			global:  def.Flags,
		},
	}
}

// Install installs packages to the rootfs.
func (m Manager) Install(pkgs []string) error {
	if len(m.flags.install) == 0 || pkgs == nil || len(pkgs) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.install...)
	args = append(args, pkgs...)

	return shared.RunCommand(m.commands.install, args...)
}

// Remove removes packages from the rootfs.
func (m Manager) Remove(pkgs []string) error {
	if len(m.flags.remove) == 0 || pkgs == nil || len(pkgs) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.remove...)
	args = append(args, pkgs...)

	return shared.RunCommand(m.commands.remove, args...)
}

// Clean cleans up cached files used by the package managers.
func (m Manager) Clean() error {
	var err error

	if len(m.flags.clean) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.clean...)

	err = shared.RunCommand(m.commands.clean, args...)
	if err != nil {
		return err
	}

	if m.hooks.clean != nil {
		err = m.hooks.clean()
	}

	return err
}

// Refresh refreshes the local package database.
func (m Manager) Refresh() error {
	if len(m.flags.refresh) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.refresh...)

	return shared.RunCommand(m.commands.refresh, args...)
}

// Update updates all packages.
func (m Manager) Update() error {
	if len(m.flags.update) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.update...)

	return shared.RunCommand(m.commands.update, args...)
}

// SetInstallFlags overrides the default install flags.
func (m *Manager) SetInstallFlags(flags ...string) {
	m.flags.install = flags
}
