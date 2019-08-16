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

// AlpineLinuxHTTP represents the Alpine Linux downloader.
type AlpineLinuxHTTP struct{}

// NewAlpineLinuxHTTP creates a new AlpineLinuxHTTP instance.
func NewAlpineLinuxHTTP() *AlpineLinuxHTTP {
	return &AlpineLinuxHTTP{}
}

// Run downloads an Alpine Linux mini root filesystem.
func (s *AlpineLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {
	releaseFull := definition.Image.Release
	releaseShort := releaseFull

	if definition.Image.Release == "edge" {
		if definition.Source.SameAs == "" {
			return fmt.Errorf("You can't use Alpine edge without setting same_as")
		}

		releaseFull = definition.Source.SameAs
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

	fname := fmt.Sprintf("alpine-minirootfs-%s-%s.tar.gz", releaseFull,
		definition.Image.ArchitectureMapped)

	tarball := fmt.Sprintf("%s/%s/releases/%s/%s", definition.Source.URL,
		releaseShort, definition.Image.ArchitectureMapped, fname)

	url, err := url.Parse(tarball)
	if err != nil {
		return err
	}

	if !definition.Source.SkipVerification && url.Scheme != "https" &&
		len(definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	var fpath string

	if definition.Source.SkipVerification {
		fpath, err = shared.DownloadHash(definition.Image, tarball, "", nil)
	} else {
		fpath, err = shared.DownloadHash(definition.Image, tarball, tarball+".sha256", sha256.New())
	}
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(definition.Image, tarball+".asc", "", nil)
		valid, err := shared.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".asc"),
			definition.Source.Keys,
			definition.Source.Keyserver)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("Failed to verify tarball")
		}
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	// Handle edge builds
	if definition.Image.Release == "edge" {
		// Upgrade to edge
		exitChroot, err := shared.SetupChroot(rootfsDir, definition.Environment)
		if err != nil {
			return err
		}

		err = shared.RunCommand("sed", "-i", "-e", "s/v[[:digit:]]\\.[[:digit:]]/edge/g", "/etc/apk/repositories")
		if err != nil {
			exitChroot()
			return err
		}

		err = shared.RunCommand("apk", "upgrade", "--update-cache", "--available")
		if err != nil {
			exitChroot()
			return err
		}

		exitChroot()
	}

	// Fix bad permissions in Alpine tarballs
	err = os.Chmod(rootfsDir, 0755)
	if err != nil {
		return err
	}

	return nil
}
