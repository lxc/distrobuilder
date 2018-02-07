package sources

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	"github.com/lxc/distrobuilder/shared"
)

// CentOSHTTP represents the CentOS HTTP downloader.
type CentOSHTTP struct {
	fname    string
	cacheDir string
}

// NewCentOSHTTP creates a new CentOSHTTP instance.
func NewCentOSHTTP() *CentOSHTTP {
	return &CentOSHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *CentOSHTTP) Run(URL, release, variant, arch, cacheDir string) error {
	s.cacheDir = cacheDir

	s.fname = getRelease(URL, release, variant, arch)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
	}

	err := shared.Download(
		URL+path.Join("/", strings.Split(release, ".")[0], "isos", arch, s.fname),
		URL+path.Join("/", strings.Split(release, ".")[0], "isos", arch, "sha256sum.txt"))
	if err != nil {
		return fmt.Errorf("Error downloading CentOS image: %s", err)
	}

	return s.unpack(filepath.Join(os.TempDir(), s.fname), cacheDir)
}

func (s CentOSHTTP) unpack(filePath, cacheDir string) error {
	isoDir := filepath.Join(os.TempDir(), "distrobuilder", "iso")
	squashfsDir := filepath.Join(os.TempDir(), "distrobuilder", "squashfs")
	tempRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs")

	os.RemoveAll(filepath.Join(cacheDir, "rootfs"))

	os.MkdirAll(isoDir, 0755)
	os.MkdirAll(tempRootDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))

	// this is easier than doing the whole loop thing ourselves
	err := shared.RunCommand("mount", filePath, isoDir)
	if err != nil {
		return err
	}
	defer syscall.Unmount(isoDir, 0)

	err = shared.RunCommand("unsquashfs", "-d", squashfsDir,
		filepath.Join(isoDir, "LiveOS/squashfs.img"))
	if err != nil {
		return err
	}

	err = shared.RunCommand("mount", filepath.Join(squashfsDir, "LiveOS", "rootfs.img"), tempRootDir)
	if err != nil {
		return err
	}
	defer syscall.Unmount(tempRootDir, 0)

	err = shared.RunCommand("rsync", "-qa", tempRootDir, cacheDir)
	if err != nil {
		return err
	}

	return nil
}

func getRelease(URL, release, variant, arch string) string {
	resp, err := http.Get(URL + path.Join("/", strings.Split(release, ".")[0], "isos", arch))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)

	regex := regexp.MustCompile(fmt.Sprintf("CentOS-%s-%s-(?i:%s)(-\\d+)?.iso", release, arch, variant))
	return regex.FindString(string(body))
}
