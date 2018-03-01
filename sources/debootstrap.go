package sources

import (
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
)

// Debootstrap represents the debootstrap downloader.
type Debootstrap struct{}

// NewDebootstrap creates a new Debootstrap instance.
func NewDebootstrap() *Debootstrap {
	return &Debootstrap{}
}

// Run runs debootstrap.
func (s *Debootstrap) Run(source shared.DefinitionSource, release, arch, cacheDir string) error {
	var args []string

	os.RemoveAll(filepath.Join(cacheDir, "rootfs"))

	if source.Variant != "" {
		args = append(args, "--variant", source.Variant)
	}

	if arch != "" {
		args = append(args, "--arch", arch)
	}

	if len(source.Keys) > 0 {
		gpgDir, err := shared.CreateGPGKeyring(source.Keyserver, source.Keys)
		if err != nil {
			return err
		}
		defer os.RemoveAll(gpgDir)

		args = append(args, "--keyring", filepath.Join(gpgDir, "pubring.kbx"))
	}

	args = append(args, release, filepath.Join(cacheDir, "rootfs"))

	if source.URL != "" {
		args = append(args, source.URL)
	}

	// If source.Suite is set, create a symlink in /usr/share/debootstrap/scripts
	// pointing release to source.Suite.
	if source.Suite != "" {
		link := filepath.Join("/usr/share/debootstrap/scripts", release)
		err := os.Symlink(source.Suite, link)
		if err != nil {
			return err
		}
		defer os.Remove(link)
	}

	return shared.RunCommand("debootstrap", args...)
}
