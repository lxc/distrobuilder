package sources

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

// AlpineLinuxHTTP represents the Alpine Linux downloader.
type AlpineLinuxHTTP struct{}

// NewAlpineLinuxHTTP creates a new AlpineLinuxHTTP instance.
func NewAlpineLinuxHTTP() *AlpineLinuxHTTP {
	return &AlpineLinuxHTTP{}
}

// Run downloads an Alpine Linux mini root filesystem.
func (s *AlpineLinuxHTTP) Run(URL, release, variant, arch, cacheDir string) error {
	fname := fmt.Sprintf("alpine-minirootfs-%s-%s.tar.gz", release, arch)
	tarball := fmt.Sprintf("%s/v%s/releases/%s/%s", URL,
		strings.Join(strings.Split(release, ".")[0:2], "."), arch, fname)

	err := shared.Download(tarball, tarball+".sha256")
	if err != nil {
		return err
	}

	shared.Download(tarball+".asc", "")
	valid, err := shared.VerifyFile(
		filepath.Join(os.TempDir(), fname),
		filepath.Join(os.TempDir(), fname+".asc"),
		[]string{"0482D84022F52DF1C4E7CD43293ACD0907D9495A"})
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("Failed to verify tarball")
	}

	err = os.MkdirAll(filepath.Join(cacheDir, "rootfs"), 0755)
	if err != nil {
		return err
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(os.TempDir(), fname), filepath.Join(cacheDir, "rootfs"), false, false)
	if err != nil {
		return err
	}

	return nil
}
