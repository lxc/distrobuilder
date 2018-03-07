package sources

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// ArchLinuxHTTP represents the Arch Linux downloader.
type ArchLinuxHTTP struct{}

// NewArchLinuxHTTP creates a new ArchLinuxHTTP instance.
func NewArchLinuxHTTP() *ArchLinuxHTTP {
	return &ArchLinuxHTTP{}
}

// Run downloads an Arch Linux tarball.
func (s *ArchLinuxHTTP) Run(source shared.DefinitionSource, release, arch, rootfsDir string) error {
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

	// Unpack
	err = lxd.Unpack(filepath.Join(os.TempDir(), fname), rootfsDir, false, false)
	if err != nil {
		return err
	}

	// Move everything inside 'root.x86_64' (which was is the tarball) to its
	// parent directory
	files, err := filepath.Glob(fmt.Sprintf("%s/*", filepath.Join(rootfsDir, "root.x86_64")))
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.Rename(file, filepath.Join(rootfsDir, path.Base(file)))
		if err != nil {
			return err
		}
	}

	return os.RemoveAll(filepath.Join(rootfsDir, "root.x86_64"))
}
