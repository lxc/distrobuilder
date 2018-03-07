package sources

import (
	"os"
	"path"
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
func (s *Debootstrap) Run(source shared.DefinitionSource, release, arch, rootfsDir string) error {
	var args []string

	os.RemoveAll(rootfsDir)

	if source.Variant != "" {
		args = append(args, "--variant", source.Variant)
	}

	if arch != "" {
		args = append(args, "--arch", arch)
	}

	if len(source.Keys) > 0 {
		keyring, err := shared.CreateGPGKeyring(source.Keyserver, source.Keys)
		if err != nil {
			return err
		}
		defer os.RemoveAll(path.Base(keyring))

		args = append(args, "--keyring", keyring)
	}

	args = append(args, release, rootfsDir)

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
