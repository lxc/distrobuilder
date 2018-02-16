package sources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

// ArchLinuxHTTP represents the Arch Linux downloader.
type ArchLinuxHTTP struct{}

// NewArchLinuxHTTP creates a new ArchLinuxHTTP instance.
func NewArchLinuxHTTP() *ArchLinuxHTTP {
	return &ArchLinuxHTTP{}
}

// Run downloads an Arch Linux tarball.
func (s *ArchLinuxHTTP) Run(source shared.DefinitionSource, release, variant, arch, cacheDir string) error {
	fname := fmt.Sprintf("archlinux-bootstrap-%s-x86_64.tar.gz", release)
	tarball := fmt.Sprintf("%s/%s/%s", source.URL, release, fname)

	err := shared.Download(tarball, "")
	if err != nil {
		return err
	}

	shared.Download(tarball+".sig", "")

	valid, err := shared.VerifyFile(
		filepath.Join(os.TempDir(), fname),
		filepath.Join(os.TempDir(), fname+".sig"),
		source.Keys,
		source.Keyserver)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("Failed to verify tarball")
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
