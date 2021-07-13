package managers

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type pacman struct {
	common
}

func (m *pacman) load() error {
	err := m.setMirrorlist()
	if err != nil {
		return errors.Wrap(err, "Failed to set mirrorlist")
	}

	err = m.setupTrustedKeys()
	if err != nil {
		return errors.Wrap(err, "Failed to setup trusted keys")
	}

	m.commands = managerCommands{
		clean:   "pacman",
		install: "pacman",
		refresh: "pacman",
		remove:  "pacman",
		update:  "pacman",
	}

	m.flags = managerFlags{
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
	}

	m.hooks = managerHooks{
		clean: func() error {
			path := "/var/cache/pacman/pkg"

			// List all entries.
			entries, err := ioutil.ReadDir(path)
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}

				return errors.Wrapf(err, "Failed to list directory '%s'", path)
			}

			// Individually wipe all entries.
			for _, entry := range entries {
				entryPath := filepath.Join(path, entry.Name())
				err := os.RemoveAll(entryPath)
				if err != nil && !os.IsNotExist(err) {
					return errors.Wrapf(err, "Failed to remove '%s'", entryPath)
				}
			}

			return nil
		},
	}

	return nil
}

func (m *pacman) setupTrustedKeys() error {
	var err error

	_, err = os.Stat("/etc/pacman.d/gnupg")
	if err == nil {
		return nil
	}

	err = shared.RunCommand("pacman-key", "--init")
	if err != nil {
		return errors.Wrap(err, "Error initializing with pacman-key")
	}

	var keyring string

	if lxd.StringInSlice(runtime.GOARCH, []string{"arm", "arm64"}) {
		keyring = "archlinuxarm"
	} else {
		keyring = "archlinux"
	}

	err = shared.RunCommand("pacman-key", "--populate", keyring)
	if err != nil {
		return errors.Wrap(err, "Error populating with pacman-key")
	}

	return nil
}

func (m *pacman) setMirrorlist() error {
	f, err := os.Create(filepath.Join("etc", "pacman.d", "mirrorlist"))
	if err != nil {
		return errors.Wrapf(err, "Failed to create file %q", filepath.Join("etc", "pacman.d", "mirrorlist"))
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
		return errors.Wrapf(err, "Failed to write to %q", filepath.Join("etc", "pacman.d", "mirrorlist"))
	}

	return nil
}
