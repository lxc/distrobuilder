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
	baseURL := fmt.Sprintf(
		"%s/%s/cloud/%s/",
		s.definition.Source.URL,
		s.definition.Image.Release,
		s.definition.Image.ArchitectureMapped,
	)
	fname := fmt.Sprintf("alt-%s-rootfs-systemd-%s.tar.xz", strings.ToLower(s.definition.Image.Release), s.definition.Image.ArchitectureMapped)

	url, err := url.Parse(baseURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse URL %q", baseURL)
	}

	checksumFile := ""

	if !s.definition.Source.SkipVerification {
		if len(s.definition.Source.Keys) != 0 {
			checksumFile = baseURL + "SHA256SUMS"

			fpath, err := shared.DownloadHash(s.definition.Image, checksumFile+".gpg", "", nil)
			if err != nil {
				return errors.Wrapf(err, "Failed to download %q", checksumFile+".gpg")
			}

			_, err = shared.DownloadHash(s.definition.Image, checksumFile, "", nil)
			if err != nil {
				return errors.Wrapf(err, "Failed to download %q", checksumFile)
			}

			valid, err := shared.VerifyFile(
				filepath.Join(fpath, "SHA256SUMS"),
				filepath.Join(fpath, "SHA256SUMS.gpg"),
				s.definition.Source.Keys,
				s.definition.Source.Keyserver)
			if err != nil {
				return errors.Wrap(err, "Failed to verify file")
			}
			if !valid {
				return errors.Errorf("Invalid signature for %q", "SHA256SUMS")
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
		return errors.Wrapf(err, "Failed to download %q", baseURL+fname)
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", fname)
	}

	return nil
}
