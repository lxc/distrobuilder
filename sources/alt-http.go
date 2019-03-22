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

// ALTHTTP represents the ALT Linux downloader.
type ALTHTTP struct{}

// NewALTHTTP creates a new ALTHTTP instance.
func NewALTHTTP() *ALTHTTP {
	return &ALTHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *ALTHTTP) Run(definition shared.Definition, rootfsDir string) error {
	release := definition.Image.Release

	baseURL := fmt.Sprintf("%s/%s/cloud/",
		definition.Source.URL, release)

	fname := fmt.Sprintf("alt-%s-rootfs-systemd-%s.tar.xz", strings.ToLower(release),
		definition.Image.ArchitectureMapped)

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	if !definition.Source.SkipVerification {
		if len(definition.Source.Keys) != 0 {

			checksumFile = baseURL + "SHA256SUM"
			fpath, err := shared.DownloadHash(definition.Image, checksumFile+".asc", "", nil)
			if err != nil {
				return err
			}

			shared.DownloadHash(definition.Image, checksumFile, "", nil)

			valid, err := shared.VerifyFile(
				filepath.Join(fpath, "SHA256SUM"),
				filepath.Join(fpath, "SHA256SUM.asc"),
				definition.Source.Keys,
				definition.Source.Keyserver)
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("Failed to validate tarball")
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
