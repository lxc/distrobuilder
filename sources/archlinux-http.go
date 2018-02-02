package sources

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

// ArchLinuxHTTP represents the debootstrap downloader.
type ArchLinuxHTTP struct{}

// NewArchLinuxHTTP creates a new ArchLinuxHTTP instance.
func NewArchLinuxHTTP() *ArchLinuxHTTP {
	return &ArchLinuxHTTP{}
}

// Run runs debootstrap.
func (s *ArchLinuxHTTP) Run(URL, release, variant, arch, cacheDir string) error {
	fname := fmt.Sprintf("archlinux-bootstrap-%s-x86_64.tar.gz", release)

	// Download
	err := shared.Download(URL+path.Join("/", release, fname), "")
	if err != nil {
		return err
	}

	err = os.MkdirAll(cacheDir, 0755)
	if err != nil {
		return err
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(os.TempDir(), fname), cacheDir, false, false)
	if err != nil {
		return err
	}

	os.RemoveAll(filepath.Join(cacheDir, "rootfs"))

	err = os.Rename(filepath.Join(cacheDir, "root.x86_64"), filepath.Join(cacheDir, "rootfs"))
	if err != nil {
		return err
	}

	return nil
}
