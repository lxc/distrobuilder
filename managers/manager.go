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

// A Manager represents a package manager.
type Manager struct {
	command string
	flags   ManagerFlags
	hooks   ManagerHooks
}

// Get returns a Manager specified by name.
func Get(name string) *Manager {
	switch name {
	case "apt":
		return NewApt()
	case "pacman":
		return NewPacman()
	case "yum":
		return NewYum()
	case "apk":
		return NewApk()
	}

	return nil
}

// Install installs packages to the rootfs.
func (m Manager) Install(pkgs []string) error {
	if len(m.flags.install) == 0 || pkgs == nil || len(pkgs) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.install...)
	args = append(args, pkgs...)

	return shared.RunCommand(m.command, args...)
}

// Remove removes packages from the rootfs.
func (m Manager) Remove(pkgs []string) error {
	if len(m.flags.remove) == 0 || pkgs == nil || len(pkgs) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.remove...)
	args = append(args, pkgs...)

	return shared.RunCommand(m.command, args...)
}

// Clean cleans up cached files used by the package managers.
func (m Manager) Clean() error {
	var err error

	if len(m.flags.clean) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.clean...)

	err = shared.RunCommand(m.command, args...)
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

	return shared.RunCommand(m.command, args...)
}

// Update updates all packages.
func (m Manager) Update() error {
	if len(m.flags.update) == 0 {
		return nil
	}

	args := append(m.flags.global, m.flags.update...)

	return shared.RunCommand(m.command, args...)
}
