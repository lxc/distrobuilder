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

type ubuntu struct {
	common

	fname string
	fpath string
}

// Run downloads the tarball and unpacks it.
func (s *ubuntu) Run() error {
	err := s.downloadImage(s.definition)
	if err != nil {
		return fmt.Errorf("Failed to download image: %w", err)
	}

	return s.unpack(filepath.Join(s.fpath, s.fname), s.rootfsDir)
}

func (s *ubuntu) downloadImage(definition shared.Definition) error {
	var baseURL string
	var err error

	switch strings.ToLower(s.definition.Image.Variant) {
	case "default":
		baseURL = fmt.Sprintf("%s/releases/%s/release/", s.definition.Source.URL,
			s.definition.Image.Release)

		if strings.ContainsAny(s.definition.Image.Release, "0123456789") {
			s.fname = fmt.Sprintf("ubuntu-base-%s-base-%s.tar.gz",
				s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
		} else {
			// if release is non-numerical, find the latest release
			s.fname, err = getLatestRelease(baseURL,
				s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
			if err != nil {
				return fmt.Errorf("Failed to get latest release: %w", err)
			}
		}
	case "core":
		baseURL = fmt.Sprintf("%s/%s/stable/current/", s.definition.Source.URL, s.definition.Image.Release)
		s.fname = fmt.Sprintf("ubuntu-core-%s-%s.img.xz", s.definition.Image.Release, s.definition.Image.ArchitectureMapped)
	default:
		return fmt.Errorf("Unknown Ubuntu variant %q", s.definition.Image.Variant)
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return fmt.Errorf("Failed to parse URL %q: %w", baseURL, err)
	}

	var fpath string

	checksumFile := ""
	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		if len(s.definition.Source.Keys) == 0 {
			return errors.New("GPG keys are required if downloading from HTTP")
		}

		checksumFile = baseURL + "SHA256SUMS"
		fpath, err = s.DownloadHash(s.definition.Image, baseURL+"SHA256SUMS.gpg", "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", baseURL+"SHA256SUMS.gpg", err)
		}

		_, err = s.DownloadHash(s.definition.Image, checksumFile, "", nil)
		if err != nil {
			return fmt.Errorf("Failed to download %q: %w", checksumFile, err)
		}

		valid, err := s.VerifyFile(
			filepath.Join(fpath, "SHA256SUMS"),
			filepath.Join(fpath, "SHA256SUMS.gpg"))
		if err != nil {
			return fmt.Errorf(`Failed to verify "SHA256SUMS": %w`, err)
		}

		if !valid {
			return errors.New(`Invalid signature for "SHA256SUMS"`)
		}
	}

	s.fpath, err = s.DownloadHash(s.definition.Image, baseURL+s.fname, checksumFile, sha256.New())
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", baseURL+s.fname, err)
	}

	return nil
}

func (s ubuntu) unpack(filePath, rootDir string) error {
	err := os.RemoveAll(rootDir)
	if err != nil {
		return fmt.Errorf("Failed to remove directory %q: %w", rootDir, err)
	}

	err = os.MkdirAll(rootDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", rootDir, err)
	}

	s.logger.WithField("file", filePath).Info("Unpacking image")

	err = shared.Unpack(filePath, rootDir)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filePath, err)
	}

	return nil
}

func getLatestRelease(baseURL, release, arch string) (string, error) {
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
		return "", fmt.Errorf("Failed to read body: %w", err)
	}

	regex := regexp.MustCompile(fmt.Sprintf("ubuntu-base-\\d{2}\\.\\d{2}(\\.\\d+)?-base-%s.tar.gz", arch))
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return string(releases[len(releases)-1]), nil
	}

	return "", errors.New("Failed to find latest release")
}
