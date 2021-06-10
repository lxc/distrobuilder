package sources

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type altLinux struct {
	common
}

func (s *altLinux) Run() error {
	baseURL := fmt.Sprintf("%s/%s/cloud/", s.definition.Source.URL, s.definition.Image.Release)
	fname := fmt.Sprintf("alt-%s-rootfs-systemd-%s.tar.xz", strings.ToLower(s.definition.Image.Release), s.definition.Image.ArchitectureMapped)

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""

	if !s.definition.Source.SkipVerification {
		if len(s.definition.Source.Keys) != 0 {

			checksumFile = baseURL + "SHA256SUM"

			fpath, err := shared.DownloadHash(s.definition.Image, checksumFile+".asc", "", nil)
			if err != nil {
				return err
			}

			shared.DownloadHash(s.definition.Image, checksumFile, "", nil)

			valid, err := shared.VerifyFile(
				filepath.Join(fpath, "SHA256SUM"),
				filepath.Join(fpath, "SHA256SUM.asc"),
				s.definition.Source.Keys,
				s.definition.Source.Keyserver)
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

	fpath, err := shared.DownloadHash(s.definition.Image, baseURL+fname, checksumFile, sha256.New())
	if err != nil {
		return err
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	return nil
}
