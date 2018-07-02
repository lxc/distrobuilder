package sources

import (
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

	if definition.Source.SkipVerification {
		err = shared.DownloadSha256(tarball, "")
	} else {
		err = shared.DownloadSha256(tarball, tarball+".sha256")
	}
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadSha256(tarball+".asc", "")
		valid, err := shared.VerifyFile(
			filepath.Join(os.TempDir(), fname),
			filepath.Join(os.TempDir(), fname+".asc"),
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
	err = lxd.Unpack(filepath.Join(os.TempDir(), fname), rootfsDir, false, false)
	if err != nil {
		return err
	}

	if definition.Image.Release == "edge" {
		// Upgrade to edge
		exitChroot, err := shared.SetupChroot(rootfsDir)
		if err != nil {
			return err
		}

		err = shared.RunCommand("sed", "-i", "-e", "s/v[[:digit:]]\\.[[:digit:]]/edge/g", "/etc/apk/repositories")
		if err != nil {
			return err
		}

		err = shared.RunCommand("apk", "upgrade", "--update-cache", "--available")
		if err != nil {
			return err
		}

		exitChroot()
	}

	return nil
}
