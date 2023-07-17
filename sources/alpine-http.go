package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

type alpineLinux struct {
	common
}

func (s *alpineLinux) Run() error {
	var releaseShort string

	releaseFull := s.definition.Image.Release

	if s.definition.Image.Release == "edge" {
		if s.definition.Source.SameAs == "" {
			return errors.New("You can't use Alpine edge without setting same_as")
		}

		releaseFull = s.definition.Source.SameAs
	}

	releaseField := strings.Split(releaseFull, ".")
	if len(releaseField) == 2 {
		releaseShort = fmt.Sprintf("v%s", releaseFull)
	} else if len(releaseField) == 3 {
		releaseShort = fmt.Sprintf("v%s.%s", releaseField[0], releaseField[1])
	} else {
		return fmt.Errorf("Bad Alpine release: %s", releaseFull)
	}

	baseURL := fmt.Sprintf("%s/%s/releases/%s", s.definition.Source.URL, releaseShort, s.definition.Image.ArchitectureMapped)

	if len(releaseField) == 2 {
		var err error

		releaseFull, err = s.getLatestRelease(baseURL, releaseFull, s.definition.Image.ArchitectureMapped)
		if err != nil {
			return fmt.Errorf("Failed to find latest release: %w", err)
		}
	}

	fname := fmt.Sprintf("alpine-minirootfs-%s-%s.tar.gz", releaseFull, s.definition.Image.ArchitectureMapped)

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
		_, err = s.DownloadHash(s.definition.Image, tarball+".asc", "", nil)
		if err != nil {
			return fmt.Errorf("Failed downloading %q: %w", tarball+".asc", err)
		}

		valid, err := s.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".asc"))
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", tarball+".asc", err)
		}

		if !valid {
			return fmt.Errorf("Invalid signature for %q", filepath.Join(fpath, fname))
		}
	}

	s.logger.WithField("file", filepath.Join(fpath, fname)).Info("Unpacking image")

	// Unpack
	err = shared.Unpack(filepath.Join(fpath, fname), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", fname, err)
	}

	// Handle edge builds
	if s.definition.Image.Release == "edge" {
		// Upgrade to edge
		exitChroot, err := shared.SetupChroot(s.rootfsDir, s.definition, nil)
		if err != nil {
			return fmt.Errorf("Failed to set up chroot: %w", err)
		}

		err = shared.RunCommand(s.ctx, nil, nil, "sed", "-i", "-e", "s/v[[:digit:]]\\.[[:digit:]]\\+/edge/g", "/etc/apk/repositories")
		if err != nil {
			{
				err := exitChroot()
				if err != nil {
					s.logger.WithField("err", err).Warn("Failed exiting chroot")
				}
			}

			return fmt.Errorf("Failed to edit apk repositories: %w", err)
		}

		err = shared.RunCommand(s.ctx, nil, nil, "apk", "upgrade", "--update-cache", "--available")
		if err != nil {
			{
				err := exitChroot()
				if err != nil {
					s.logger.WithField("err", err).Warn("Failed exiting chroot")
				}
			}

			return fmt.Errorf("Failed to upgrade edge build: %w", err)
		}

		err = exitChroot()
		if err != nil {
			return fmt.Errorf("Failed exiting chroot: %w", err)
		}
	}

	// Fix bad permissions in Alpine tarballs
	err = os.Chmod(s.rootfsDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to chmod %q: %w", s.rootfsDir, err)
	}

	return nil
}

func (s *alpineLinux) getLatestRelease(baseURL, release string, arch string) (string, error) {
	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = http.Get(baseURL)
		if err != nil {
			return fmt.Errorf("Failed to GET %q: %w", baseURL, err)
		}

		return nil
	}, 3)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to ready body: %w", err)
	}

	regex := regexp.MustCompile(fmt.Sprintf(">alpine-minirootfs-(%s\\.\\d+)-%s.tar.gz<", release, arch))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", nil
}
