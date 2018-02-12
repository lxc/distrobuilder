package managers

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/monstermunchkin/distrobuilder/shared"
)

// NewPacman creates a new Manager instance.
func NewPacman() *Manager {
	err := pacmanSetMirrorlist()
	if err != nil {
		return nil
	}

	// shared.RunCommand("pacman", "-Syy")

	err = pacmanSetupTrustedKeys()
	if err != nil {
		return nil
	}

	return &Manager{
		command: "pacman",
		flags: ManagerFlags{
			clean: []string{
				"-Sc",
			},
			global: []string{
				"--noconfirm",
			},
			install: []string{
				"-S", "--needed",
			},
			remove: []string{
				"-Rcs",
			},
			refresh: []string{
				"-Syy",
			},
			update: []string{
				"-Su",
			},
		},
		hooks: ManagerHooks{
			clean: func() error {
				return os.RemoveAll("/var/cache/pacman/pkg")
			},
		},
	}
}

func pacmanSetupTrustedKeys() error {
	var err error

	_, err = os.Stat("/etc/pacman.d/gnupg")
	if err == nil {
		return nil
	}

	err = shared.RunCommand("pacman-key", "--init")
	if err != nil {
		return fmt.Errorf("Error initializing with pacman-key: %s", err)
	}

	err = shared.RunCommand("pacman-key", "--populate", "archlinux")
	if err != nil {
		return fmt.Errorf("Error populating with pacman-key: %s", err)
	}

	return nil
}

func pacmanSetMirrorlist() error {
	f, err := os.Create(filepath.Join("etc", "pacman.d", "mirrorlist"))
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("Server = http://mirrors.kernel.org/archlinux/$repo/os/$arch")
	if err != nil {
		return err
	}

	return nil
}
