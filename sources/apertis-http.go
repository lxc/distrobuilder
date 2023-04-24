package sources

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

type apertis struct {
	common
}

// Run downloads the tarball and unpacks it.
func (s *apertis) Run() error {
	release := s.definition.Image.Release
	exactRelease := release

	// https://images.apertis.org/daily/v2020dev0/20190830.0/amd64/minimal/ospack_v2020dev0-amd64-minimal_20190830.0.tar.gz
	baseURL := fmt.Sprintf("%s/%s/%s",
		s.definition.Source.URL, s.definition.Source.Variant, release)

	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = http.Head(baseURL)
		if err != nil {
			return fmt.Errorf("Failed to HEAD %q: %w", baseURL, err)
		}

		return nil
	}, 3)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		// Possibly, release is a specific release (18.12.0 instead of 18.12). Lets trim the prefix and continue.
		re := regexp.MustCompile(`\.\d+$`)
		release = strings.TrimSuffix(release, re.FindString(release))
		baseURL = fmt.Sprintf("%s/%s/%s",
			s.definition.Source.URL, s.definition.Source.Variant, release)
	} else {
		exactRelease, err = s.getLatestRelease(baseURL, release)
		if err != nil {
			return fmt.Errorf("Failed to get latest release: %w", err)
		}
	}

	baseURL = fmt.Sprintf("%s/%s/%s/%s/",
		baseURL, exactRelease, s.definition.Image.ArchitectureMapped, s.definition.Image.Variant)
	fname := fmt.Sprintf("ospack_%s-%s-%s_%s.tar.gz",
		release, s.definition.Image.ArchitectureMapped, s.definition.Image.Variant, exactRelease)

	url, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", baseURL, err)
	}

	// Force gpg checks when using http
	if url.Scheme != "https" {
		return errors.New("Only HTTPS server is supported")
	}

	fpath, err := s.DownloadHash(s.definition.Image, baseURL+fname, "", nil)
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+fname, err)
	}

	s.logger.WithField("file", filepath.Join(fpath, fname)).Info("Unpacking image")

	// Unpack
	err = shared.Unpack(filepath.Join(fpath, fname), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", fname, err)
	}

	return nil
}

func (s *apertis) getLatestRelease(baseURL, release string) (string, error) {
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

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)/<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", nil
}
