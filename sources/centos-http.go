package sources

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
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
func (s *CentOSHTTP) Run(definition shared.Definition, rootfsDir string) error {
	baseURL := fmt.Sprintf("%s/%s/isos/%s/", definition.Source.URL,
		strings.Split(definition.Image.Release, ".")[0],
		definition.Image.ArchitectureMapped)

	s.fname = s.getRelease(definition.Source.URL, definition.Image.Release,
		definition.Source.Variant, definition.Image.ArchitectureMapped)
	if s.fname == "" {
		return fmt.Errorf("Couldn't get name of iso")
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	// Force gpg checks when using http
	if url.Scheme != "https" {
		if len(definition.Source.Keys) == 0 {
			return errors.New("GPG keys are required if downloading from HTTP")
		}

		checksumFile = "sha256sum.txt.asc"
		shared.Download(baseURL+checksumFile, "")
		valid, err := shared.VerifyFile(filepath.Join(os.TempDir(), checksumFile), "",
			definition.Source.Keys, definition.Source.Keyserver)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("Failed to verify tarball")
		}
	}

	err = shared.Download(baseURL+s.fname, checksumFile)
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
	os.MkdirAll(squashfsDir, 0755)
	os.MkdirAll(tempRootDir, 0755)
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))

	// this is easier than doing the whole loop thing ourselves
	err := shared.RunCommand("mount", filePath, isoDir)
	if err != nil {
		return err
	}
	defer syscall.Unmount(isoDir, 0)

	err = shared.RunCommand("mount", filepath.Join(isoDir, "LiveOS",
		"squashfs.img"), squashfsDir)
	if err != nil {
		return err
	}
	defer syscall.Unmount(squashfsDir, 0)

	err = shared.RunCommand("mount", filepath.Join(squashfsDir, "LiveOS",
		"rootfs.img"), tempRootDir)
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

	// Create cdrom repo for yum
	err = os.MkdirAll(filepath.Join(rootfsDir, "mnt", "cdrom"), 0755)
	if err != nil {
		return err
	}

	// Copy repo relevant files to the cdrom
	err = shared.RunCommand("rsync", "-qa",
		filepath.Join(isoDir, "Packages"),
		filepath.Join(isoDir, "repodata"),
		filepath.Join(rootfsDir, "mnt", "cdrom"))
	if err != nil {
		return err
	}

	// Find all relevant GPG keys
	gpgKeys, err := filepath.Glob(filepath.Join(isoDir, "RPM-GPG-KEY-*"))
	if err != nil {
		return err
	}

	// Copy the keys to the cdrom
	for _, key := range gpgKeys {
		err = shared.RunCommand("rsync", "-qa", key,
			filepath.Join(rootfsDir, "mnt", "cdrom"))
		if err != nil {
			return err
		}
	}

	return nil
}

func (s CentOSHTTP) getRelease(URL, release, variant, arch string) string {
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
