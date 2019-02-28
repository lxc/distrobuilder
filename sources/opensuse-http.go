package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// OpenSUSEHTTP represents the OpenSUSE HTTP downloader.
type OpenSUSEHTTP struct{}

// NewOpenSUSEHTTP creates a new OpenSUSEHTTP instance.
func NewOpenSUSEHTTP() *OpenSUSEHTTP {
	return &OpenSUSEHTTP{}
}

// Run downloads an OpenSUSE tarball.
func (s *OpenSUSEHTTP) Run(definition shared.Definition, rootfsDir string) error {
	var baseURL string
	var fname string

	tarballPath := s.getPathToTarball(definition.Source.URL, definition.Image.Release,
		definition.Image.ArchitectureMapped)

	resp, err := http.Head(tarballPath)
	if err != nil {
		return fmt.Errorf("Couldn't resolve URL: %v", err)
	}

	baseURL, fname = path.Split(resp.Request.URL.String())

	url, err := url.Parse(fmt.Sprintf("%s/%s", baseURL, fname))
	if err != nil {
		return err
	}

	fpath, err := shared.DownloadHash(definition.Image, url.String(), "", nil)
	if err != nil {
		return fmt.Errorf("Error downloading openSUSE image: %s", err)
	}

	if definition.Source.SkipVerification {
		// Unpack
		return lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	}

	checksumPath := fmt.Sprintf("%s.sha256", tarballPath)
	checksumFile := path.Base(checksumPath)

	shared.DownloadHash(definition.Image, checksumPath, "", nil)
	valid, err := shared.VerifyFile(filepath.Join(fpath, checksumFile), "",
		definition.Source.Keys, definition.Source.Keyserver)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("Failed to verify tarball")
	}

	// Manually verify the checksum
	checksum, err := shared.GetSignedContent(filepath.Join(fpath, checksumFile),
		definition.Source.Keys, definition.Source.Keyserver)
	if err != nil {
		return fmt.Errorf("Failed to read signed file: %v", err)
	}

	imagePath := filepath.Join(fpath, fname)

	image, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("Failed to verify image: %v", err)
	}

	hash := sha256.New()
	_, err = io.Copy(hash, image)
	if err != nil {
		image.Close()
		return fmt.Errorf("Failed to verify image: %v", err)
	}

	image.Close()

	result := fmt.Sprintf("%x", hash.Sum(nil))
	checksumStr := strings.TrimSpace(string(checksum))

	if result != checksumStr {
		return fmt.Errorf("Hash mismatch for %s: %s != %s", imagePath, result, checksumStr)
	}

	// Unpack
	return lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
}

func (s *OpenSUSEHTTP) getPathToTarball(baseURL string, release string, arch string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	u.Path = path.Join(u.Path, "repositories", "Virtualization:", "containers:", "images:")

	if strings.ToLower(release) == "tumbleweed" {
		u.Path = path.Join(u.Path, "openSUSE-Tumbleweed")

		switch arch {
		case "i686", "x86_64":
			u.Path = path.Join(u.Path, "container")
		case "aarch64":
			u.Path = path.Join(u.Path, "container_ARM")
		case "ppc64le":
			u.Path = path.Join(u.Path, "container_PowerPC")
		case "s390x":
			u.Path = path.Join(u.Path, "container_zSystems")
		default:
			return ""
		}

		u.Path = path.Join(u.Path, fmt.Sprintf("opensuse-tumbleweed-image.%s-lxc.tar.xz",
			arch))
	} else {
		u.Path = path.Join(u.Path, fmt.Sprintf("openSUSE-Leap-%s", release))

		switch arch {
		case "x86_64":
			u.Path = path.Join(u.Path, "containers")
		case "aarch64", "ppc64le":
			u.Path = path.Join(u.Path, "containers_ports")
		}

		u.Path = path.Join(u.Path, fmt.Sprintf("opensuse-leap-%s-image.%s-lxc.tar.xz", release,
			arch))
	}

	return u.String()
}
