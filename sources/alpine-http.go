package sources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// AlpineLinuxHTTP represents the Alpine Linux downloader.
type AlpineLinuxHTTP struct{}

// NewAlpineLinuxHTTP creates a new AlpineLinuxHTTP instance.
func NewAlpineLinuxHTTP() *AlpineLinuxHTTP {
	return &AlpineLinuxHTTP{}
}

// Run downloads an Alpine Linux mini root filesystem.
func (s *AlpineLinuxHTTP) Run(source shared.DefinitionSource, release, arch, rootfsDir string) error {
	fname := fmt.Sprintf("alpine-minirootfs-%s-%s.tar.gz", release, arch)
	tarball := fmt.Sprintf("%s/v%s/releases/%s/%s", source.URL,
		strings.Join(strings.Split(release, ".")[0:2], "."), arch, fname)

	err := shared.Download(tarball, tarball+".sha256")
	if err != nil {
		return err
	}

	shared.Download(tarball+".asc", "")
	valid, err := shared.VerifyFile(
		filepath.Join(os.TempDir(), fname),
		filepath.Join(os.TempDir(), fname+".asc"),
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

	return nil
}
