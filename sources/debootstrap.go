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
func (s *Debootstrap) Run(source shared.DefinitionSource, release, variant, arch, cacheDir string) error {
	var args []string

	os.RemoveAll(filepath.Join(cacheDir, "rootfs"))

	if variant != "" {
		args = append(args, "--variant", variant)
	}

	if arch != "" {
		args = append(args, "--arch", arch)
	}

	args = append(args, release, filepath.Join(cacheDir, "rootfs"))

	if source.URL != "" {
		args = append(args, source.URL)
	}

	return shared.RunCommand("debootstrap", args...)
}
