package sources

import (
	"crypto/sha512"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type gentoo struct {
	common
}

// Run downloads a Gentoo stage3 tarball.
func (s *gentoo) Run() error {
	topLevelArch := s.definition.Image.ArchitectureMapped
	if topLevelArch == "i686" {
		topLevelArch = "x86"
	} else if strings.HasPrefix(topLevelArch, "arm") && topLevelArch != "arm64" {
		topLevelArch = "arm"
	} else if strings.HasPrefix(topLevelArch, "ppc") {
		topLevelArch = "ppc"
	} else if strings.HasPrefix(topLevelArch, "s390") {
		topLevelArch = "s390"
	}

	var baseURL string

	if s.definition.Source.Variant == "systemd" {
		baseURL = fmt.Sprintf("%s/releases/%s/autobuilds/current-stage3-%s-%s",
			s.definition.Source.URL, topLevelArch,
			s.definition.Image.ArchitectureMapped, s.definition.Source.Variant)
	} else {
		baseURL = fmt.Sprintf("%s/releases/%s/autobuilds/current-stage3-%s",
			s.definition.Source.URL, topLevelArch,
			s.definition.Image.ArchitectureMapped)
	}

	fname, err := s.getLatestBuild(baseURL, s.definition.Image.ArchitectureMapped, s.definition.Source.Variant)
	if err != nil {
		return fmt.Errorf("Failed to get latest build: %w", err)
	}

	tarball := fmt.Sprintf("%s/%s", baseURL, fname)

	url, err := url.Parse(tarball)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", tarball, err)
	}

	if !s.definition.Source.SkipVerification && url.Scheme != "https" &&
		len(s.definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	var fpath string

	if s.definition.Source.SkipVerification {
		fpath, err = shared.DownloadHash(s.definition.Image, tarball, "", nil)
	} else {
		fpath, err = shared.DownloadHash(s.definition.Image, tarball, tarball+".DIGESTS", sha512.New())
	}
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", tarball, err)
	}

	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		_, err = shared.DownloadHash(s.definition.Image, tarball+".DIGESTS.asc", "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", tarball+".DIGESTS.asc", err)
		}

		valid, err := shared.VerifyFile(
			filepath.Join(fpath, fname+".DIGESTS.asc"),
			"",
			s.definition.Source.Keys,
			s.definition.Source.Keyserver)
		if err != nil {
			return fmt.Errorf("Failed to verify %q: %w", filepath.Join(fpath, fname+".DIGESTS.asc"), err)
		}
		if !valid {
			return fmt.Errorf("Failed to verify %q", fname+".DIGESTS.asc")
		}
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, fname), err)
	}

	return nil
}

func (s *gentoo) getLatestBuild(baseURL, arch, variant string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", fmt.Errorf("Failed to GET %q: %w", baseURL, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("Failed to read body: %w", err)
	}

	var regex *regexp.Regexp

	// Look for .tar.xz
	if variant != "" {
		regex = regexp.MustCompile(fmt.Sprintf(`"stage3-%s-%s-.*.tar.xz">`, arch, variant))
	} else {
		regex = regexp.MustCompile(fmt.Sprintf(`"stage3-%s-.*.tar.xz">`, arch))
	}

	// Find all stage3 related files
	matches := regex.FindAllString(string(body), -1)
	if len(matches) > 0 {
		// Take the first match since they're all the same anyway
		return strings.Trim(matches[0], `<>"`), nil
	}

	// Look for .tar.bz2
	if variant != "" {
		regex = regexp.MustCompile(fmt.Sprintf(`"stage3-%s-%s-.*.tar.bz2">`, arch, variant))
	} else {
		regex = regexp.MustCompile(fmt.Sprintf(`">stage3-%s-.*.tar.bz2">`, arch))
	}

	// Find all stage3 related files
	matches = regex.FindAllString(string(body), -1)
	if len(matches) > 0 {
		// Take the first match since they're all the same anyway
		return strings.Trim(matches[0], `<>"`), nil
	}

	return "", errors.New("Failed to get match")
}
