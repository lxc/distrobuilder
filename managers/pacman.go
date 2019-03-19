package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
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
		commands: ManagerCommands{
			clean:   "pacman",
			install: "pacman",
			refresh: "pacman",
			remove:  "pacman",
			update:  "pacman",
		},
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

	var keyring string

	if lxd.StringInSlice(runtime.GOARCH, []string{"arm", "arm64"}) {
		keyring = "archlinuxarm"
	} else {
		keyring = "archlinux"
	}

	err = shared.RunCommand("pacman-key", "--populate", keyring)
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

	var mirror string

	if lxd.StringInSlice(runtime.GOARCH, []string{"arm", "arm64"}) {
		mirror = "Server = http://mirror.archlinuxarm.org/$arch/$repo"
	} else {
		mirror = "Server = http://mirrors.kernel.org/archlinux/$repo/os/$arch"
	}

	_, err = f.WriteString(mirror)
	if err != nil {
		return err
	}

	return nil
}
