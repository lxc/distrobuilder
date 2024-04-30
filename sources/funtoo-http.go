package sources

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
)

type funtoo struct {
	common
}

// Run downloads a Funtoo stage3 tarball.
func (s *funtoo) Run() error {
	topLevelArch := s.definition.Image.ArchitectureMapped
	if topLevelArch == "generic_32" {
		topLevelArch = "x86-32bit"
	} else if topLevelArch == "generic_64" {
		topLevelArch = "x86-64bit"
	} else if topLevelArch == "armv7a_vfpv3_hardfp" {
		topLevelArch = "arm-32bit"
	} else if topLevelArch == "arm64_generic" {
		topLevelArch = "arm-64bit"
	}

	// Keep release backward compatible to old implementation
	// and to permit to have yet the funtoo/1.4 alias.
	if s.definition.Image.Release == "1.4" {
		s.definition.Image.Release = "1.4-release-std"
	}

	baseURL := fmt.Sprintf("%s/%s/%s/%s",
		s.definition.Source.URL, s.definition.Image.Release,
		topLevelArch, s.definition.Image.ArchitectureMapped)

	// Get the latest release tarball.
	fname := "stage3-latest.tar.xz"
	tarball := fmt.Sprintf("%s/%s", baseURL, fname)

	url, err := url.Parse(tarball)
	if err != nil {
		return fmt.Errorf("Failed to parse URL %q: %w", tarball, err)
	}

	if !s.definition.Source.SkipVerification && url.Scheme != "https" &&
		len(s.definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	var fpath string

	fpath, err = s.DownloadHash(s.definition.Image, tarball, "", nil)
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", tarball, err)
	}

	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		_, err = s.DownloadHash(s.definition.Image, tarball+".gpg", "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", tarball+".gpg", err)
		}

		valid, err := s.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".gpg"))
		if err != nil {
			return fmt.Errorf("Failed to verify file: %w", err)
		}

		if !valid {
			return fmt.Errorf("Invalid signature for %q", filepath.Join(fpath, fname))
		}
	}

	s.logger.WithField("file", filepath.Join(fpath, fname)).Info("Unpacking image")

	// Unpack
	err = shared.Unpack(filepath.Join(fpath, fname), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, fname), err)
	}

	return nil
}
