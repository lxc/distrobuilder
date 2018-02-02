package sources

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

// AlpineLinuxHTTP represents the debootstrap downloader.
type AlpineLinuxHTTP struct{}

// NewAlpineLinuxHTTP creates a new AlpineLinuxHTTP instance.
func NewAlpineLinuxHTTP() *AlpineLinuxHTTP {
	return &AlpineLinuxHTTP{}
}

// Run runs debootstrap.
func (s *AlpineLinuxHTTP) Run(URL, release, variant, arch, cacheDir string) error {
	realArch := arch

	if arch == "amd64" {
		realArch = "x86_64"
	}

	fname := fmt.Sprintf("alpine-minirootfs-%s-%s.tar.gz", release, arch)

	// Download
	parts := strings.Split("3.7.0", ".")
	strings.Join(parts[0:2], ".")
	err := shared.Download(URL+path.Join("/",
		fmt.Sprintf("v%s", strings.Join(strings.Split(release, ".")[0:2], ".")),
		"releases", realArch, fname), "")
	if err != nil {
		return err
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
