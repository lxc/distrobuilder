package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// OpenWrtHTTP represents the ALT Linux downloader.
type OpenWrtHTTP struct{}

// NewOpenWrtHTTP creates a new OpenWrtHTTP instance.
func NewOpenWrtHTTP() *OpenWrtHTTP {
	return &OpenWrtHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *OpenWrtHTTP) Run(definition shared.Definition, rootfsDir string) error {
	release := definition.Image.Release

	architecturePath := strings.Replace(definition.Image.ArchitectureMapped, "_", "/", 1)

	baseURL := fmt.Sprintf("%s/releases/%s/targets/%s/",
		definition.Source.URL, release, architecturePath)
	if release == "snapshot" {
		baseURL = fmt.Sprintf("%s/snapshots/targets/%s/",
			definition.Source.URL, architecturePath)
	}

	releaseInFilename := strings.ToLower(release) + "-"

	if release == "snapshot" {
		releaseInFilename = ""
	}

	fname := fmt.Sprintf("openwrt-%s%s-generic-rootfs.tar.gz", releaseInFilename,
		strings.Replace(definition.Image.ArchitectureMapped, "_", "-", 1))

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	if !definition.Source.SkipVerification {
		if len(definition.Source.Keys) != 0 {
			checksumFile = baseURL + "sha256sums"
			fpath, err := shared.DownloadHash(definition.Image, checksumFile+".asc", "", nil)
			if err != nil {
				return err
			}

			_, err = shared.DownloadHash(definition.Image, checksumFile, "", nil)
			if err != nil {
				return err
			}

			valid, err := shared.VerifyFile(
				filepath.Join(fpath, "sha256sums"),
				filepath.Join(fpath, "sha256sums.asc"),
				definition.Source.Keys,
				definition.Source.Keyserver)
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("Failed to validate archive")
			}
		} else {
			// Force gpg checks when using http
			if url.Scheme != "https" {
				return errors.New("GPG keys are required if downloading from HTTP")
			}
		}
	}

	fpath, err := shared.DownloadHash(definition.Image, baseURL+fname, checksumFile, sha256.New())
	if err != nil {
		return err
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	return nil
}
