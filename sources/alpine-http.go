package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type alpineLinux struct {
	common
}

func (s *alpineLinux) Run() error {
	releaseFull := s.definition.Image.Release
	releaseShort := releaseFull

	if s.definition.Image.Release == "edge" {
		if s.definition.Source.SameAs == "" {
			return errors.New("You can't use Alpine edge without setting same_as")
		}

		releaseFull = s.definition.Source.SameAs
		releaseShort = releaseFull
	}

	releaseField := strings.Split(releaseFull, ".")
	if len(releaseField) == 2 {
		releaseShort = fmt.Sprintf("v%s", releaseFull)
		releaseFull = fmt.Sprintf("%s.0", releaseFull)
	} else if len(releaseField) == 3 {
		releaseShort = fmt.Sprintf("v%s.%s", releaseField[0], releaseField[1])
	} else {
		return fmt.Errorf("Bad Alpine release: %s", releaseFull)
	}

	fname := fmt.Sprintf("alpine-minirootfs-%s-%s.tar.gz", releaseFull, s.definition.Image.ArchitectureMapped)

	tarball := fmt.Sprintf("%s/%s/releases/%s/%s", s.definition.Source.URL, releaseShort, s.definition.Image.ArchitectureMapped, fname)

	url, err := url.Parse(tarball)
	if err != nil {
		return fmt.Errorf("Failed to parse URL %q: %w", tarball, err)
	}

	if !s.definition.Source.SkipVerification && url.Scheme != "https" &&
		len(s.definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	var fpath string

	if s.definition.Source.SkipVerification {
		fpath, err = s.DownloadHash(s.definition.Image, tarball, "", nil)
	} else {
		fpath, err = s.DownloadHash(s.definition.Image, tarball, tarball+".sha256", sha256.New())
	}
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", tarball, err)
	}

	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		s.DownloadHash(s.definition.Image, tarball+".asc", "", nil)
		valid, err := s.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".asc"),
			s.definition.Source.Keys,
			s.definition.Source.Keyserver)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", tarball+".asc", err)
		}
		if !valid {
			return fmt.Errorf("Invalid signature for %q", filepath.Join(fpath, fname))
		}
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", fname, err)
	}

	// Handle edge builds
	if s.definition.Image.Release == "edge" {
		// Upgrade to edge
		exitChroot, err := shared.SetupChroot(s.rootfsDir, s.definition.Environment, nil)
		if err != nil {
			return fmt.Errorf("Failed to set up chroot: %w", err)
		}

		err = shared.RunCommand("sed", "-i", "-e", "s/v[[:digit:]]\\.[[:digit:]]\\+/edge/g", "/etc/apk/repositories")
		if err != nil {
			exitChroot()
			return fmt.Errorf("Failed to edit apk repositories: %w", err)
		}

		err = shared.RunCommand("apk", "upgrade", "--update-cache", "--available")
		if err != nil {
			exitChroot()
			return fmt.Errorf("Failed to upgrade edge build: %w", err)
		}

		exitChroot()
	}

	// Fix bad permissions in Alpine tarballs
	err = os.Chmod(s.rootfsDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to chmod %q: %w", s.rootfsDir, err)
	}

	return nil
}
