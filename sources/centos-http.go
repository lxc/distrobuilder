package sources

import (
	"errors"
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
	fname string
}

// NewCentOSHTTP creates a new CentOSHTTP instance.
func NewCentOSHTTP() *CentOSHTTP {
	return &CentOSHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *CentOSHTTP) Run(source shared.DefinitionSource, release, arch, rootfsDir string) error {
	baseURL := fmt.Sprintf("%s/%s/isos/%s/", source.URL, strings.Split(release, ".")[0], arch)

	s.fname = getRelease(source.URL, release, source.Variant, arch)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
	}

	shared.Download(baseURL+"sha256sum.txt.asc", "")
	valid, err := shared.VerifyFile(filepath.Join(os.TempDir(), "sha256sum.txt.asc"), "",
		source.Keys, source.Keyserver)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("Failed to verify tarball")
	}

	err = shared.Download(baseURL+s.fname, "sha256sum.txt.asc")
	if err != nil {
		return fmt.Errorf("Error downloading CentOS image: %s", err)
	}

	return s.unpack(filepath.Join(os.TempDir(), s.fname), rootfsDir)
}

func (s CentOSHTTP) unpack(filePath, rootfsDir string) error {
	isoDir := filepath.Join(os.TempDir(), "distrobuilder", "iso")
	squashfsDir := filepath.Join(os.TempDir(), "distrobuilder", "squashfs")
	tempRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs")

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

	// Remove rootfsDir otherwise rsync will copy the content into the directory
	// itself
	err = os.RemoveAll(rootfsDir)
	if err != nil {
		return err
	}

	err = shared.RunCommand("rsync", "-qa", tempRootDir+"/", rootfsDir)
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
