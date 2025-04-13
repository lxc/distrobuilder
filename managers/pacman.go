package managers

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	"github.com/lxc/distrobuilder/shared"
)

type pacman struct {
	common
}

func (m *pacman) load() error {
	err := m.setMirrorlist()
	if err != nil {
		return fmt.Errorf("Failed to set mirrorlist: %w", err)
	}

	err = m.setupTrustedKeys()
	if err != nil {
		return fmt.Errorf("Failed to setup trusted keys: %w", err)
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
			entries, err := os.ReadDir(path)
			if err != nil {
				if os.IsNotExist(err) {
					return nil
				}

				return fmt.Errorf("Failed to list directory '%s': %w", path, err)
			}

			// Individually wipe all entries.
			for _, entry := range entries {
				entryPath := filepath.Join(path, entry.Name())
				err := os.RemoveAll(entryPath)
				if err != nil && !os.IsNotExist(err) {
					return fmt.Errorf("Failed to remove '%s': %w", entryPath, err)
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

	err = shared.RunCommand(m.ctx, nil, nil, "pacman-key", "--init")
	if err != nil {
		return fmt.Errorf("Error initializing with pacman-key: %w", err)
	}

	var keyring string

	if slices.Contains([]string{"arm", "arm64"}, runtime.GOARCH) {
		keyring = "archlinuxarm"
	} else {
		keyring = "archlinux"
	}

	err = shared.RunCommand(m.ctx, nil, nil, "pacman-key", "--populate", keyring)
	if err != nil {
		return fmt.Errorf("Error populating with pacman-key: %w", err)
	}

	return nil
}

func (m *pacman) setMirrorlist() error {
	f, err := os.Create(filepath.Join("etc", "pacman.d", "mirrorlist"))
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", filepath.Join("etc", "pacman.d", "mirrorlist"), err)
	}

	defer f.Close()

	var mirror string

	if slices.Contains([]string{"arm", "arm64"}, runtime.GOARCH) {
		mirror = "Server = http://mirror.archlinuxarm.org/$arch/$repo"
	else if slices.Contains([]string{"riscv64"}, runtime.GOARCH) {
		mirror = "Server = https://archriscv.felixc.at/repo/$repo"
	} else {
		mirror = "Server = http://mirrors.kernel.org/archlinux/$repo/os/$arch"
	}

	_, err = f.WriteString(mirror)
	if err != nil {
		return fmt.Errorf("Failed to write to %q: %w", filepath.Join("etc", "pacman.d", "mirrorlist"), err)
	}

	return nil
}
