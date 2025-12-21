package sources

import (
	"crypto/sha256"
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

type openwrt struct {
	common
}

// Run downloads the tarball and unpacks it.
func (s *openwrt) Run() error {
	var baseURL string

	release := s.definition.Image.Release
	releaseInFilename := strings.ToLower(release) + "-"

	var architecturePath string

	switch s.definition.Image.ArchitectureMapped {
	case "x86_64":
		architecturePath = "x86/64"
	case "armv7l":
		architecturePath = "armsr/armv7"
	case "aarch64":
		architecturePath = "armsr/armv8"
	}

	// Figure out the correct release
	if release == "snapshot" {
		// Build a daily snapshot.
		baseURL = fmt.Sprintf("%s/snapshots/targets/%s/",
			s.definition.Source.URL, architecturePath)
		releaseInFilename = ""
	} else {
		baseURL = fmt.Sprintf("%s/releases", s.definition.Source.URL)

		matched, err := regexp.MatchString(`^\d+\.\d+$`, release)
		if err != nil {
			return fmt.Errorf("Failed to match release: %w", err)
		}

		if matched {
			// A release of the form '18.06' has been provided. We need to find
			// out the latest service release of the form '18.06.0'.
			release, err = s.getLatestServiceRelease(baseURL, release)
			if err != nil {
				return fmt.Errorf("Failed to get latest service release: %w", err)
			}

			releaseInFilename = strings.ToLower(release) + "-"
		}

		baseURL = fmt.Sprintf("%s/%s/targets/%s/", baseURL, release, architecturePath)
	}

	var fname string

	fname = fmt.Sprintf("openwrt-%s%s-rootfs.tar.gz", releaseInFilename,
		strings.Replace(architecturePath, "/", "-", 1))

	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = s.client.Head(baseURL)
		if err != nil {
			return fmt.Errorf("Failed to HEAD %q: %w", baseURL, err)
		}

		return nil
	}, 3)
	if err != nil {
		return err
	}

	// Use fallback image "generic"
	if resp.StatusCode == http.StatusNotFound && s.definition.Image.ArchitectureMapped == "x86_64" {
		baseURL = strings.ReplaceAll(baseURL, "x86/64", "x86/generic")
		baseURL = strings.ReplaceAll(baseURL, "x86-64", "x86-generic")
		fname = strings.ReplaceAll(fname, "x86-64", "x86-generic")
	}

	_, err = url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", baseURL, err)
	}

	checksumFile := ""
	if !s.definition.Source.SkipVerification {
		const checksumFileName string = "sha256sums"
		const signatureFileName string = "sha256sums.asc"

		if len(s.definition.Source.Keys) == 0 {
			return errors.New(`GPG keys are required unless "skip_verification: true".`)
		}

		checksumFile = baseURL + checksumFileName
		dirpath, err := s.DownloadHash(s.definition.Image, checksumFile, "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", checksumFile, err)
		}

		checksumSignatureFile := baseURL + signatureFileName
		_, err = s.DownloadHash(s.definition.Image, checksumSignatureFile, "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", checksumSignatureFile, err)
		}

		valid, err := s.VerifyFile(
			filepath.Join(dirpath, checksumFileName),
			filepath.Join(dirpath, signatureFileName))
		if err != nil {
			return fmt.Errorf(`Failed to verify %q using %q: %w`, checksumFile, checksumSignatureFile, err)
		}

		if !valid {
			return fmt.Errorf(`Invalid signature for %q`, checksumFile)
		}
	}

	fpath, err := s.DownloadHash(s.definition.Image, baseURL+fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+fname, err)
	}

	s.logger.WithField("file", filepath.Join(fpath, fname)).Info("Unpacking image")

	// Unpack
	err = shared.Unpack(filepath.Join(fpath, fname), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, fname), err)
	}

	return nil
}

func (s *openwrt) getLatestServiceRelease(baseURL, release string) (string, error) {
	var (
		resp *http.Response
		err  error
	)

	err = shared.Retry(func() error {
		resp, err = s.client.Get(baseURL)
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

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", errors.New("Failed to find latest service release")
}
