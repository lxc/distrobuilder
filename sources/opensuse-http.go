package sources

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/antchfx/htmlquery"

	"github.com/lxc/distrobuilder/shared"
)

type opensuse struct {
	common
}

// Run downloads an OpenSUSE tarball.
func (s *opensuse) Run() error {
	var baseURL string
	var fname string

	if s.definition.Source.URL == "" {
		s.definition.Source.URL = "https://mirrorcache-eu.opensuse.org/download"
	}

	tarballPath, err := s.getPathToTarball(s.definition.Source.URL, s.definition.Image.Release,
		s.definition.Image.ArchitectureMapped)
	if err != nil {
		return fmt.Errorf("Failed to get tarball path: %w", err)
	}

	var resp *http.Response

	err = shared.Retry(func() error {
		resp, err = http.Head(tarballPath)
		if err != nil {
			return fmt.Errorf("Failed to HEAD %q: %w", tarballPath, err)
		}

		return nil
	}, 3)
	if err != nil {
		return err
	}

	baseURL, fname = path.Split(resp.Request.URL.String())

	url, err := url.Parse(fmt.Sprintf("%s%s", baseURL, fname))
	if err != nil {
		return fmt.Errorf("Failed to parse %q: %w", fmt.Sprintf("%s%s", baseURL, fname), err)
	}

	fpath, err := s.DownloadHash(s.definition.Image, url.String(), "", nil)
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", url.String(), err)
	}

	_, err = s.DownloadHash(s.definition.Image, url.String()+".sha256", "", nil)
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", url.String()+".sha256", err)
	}

	if !s.definition.Source.SkipVerification {
		err = s.verifyTarball(filepath.Join(fpath, fname), s.definition)
		if err != nil {
			return fmt.Errorf("Failed to verify %q: %w", filepath.Join(fpath, fname), err)
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

func (s *opensuse) verifyTarball(imagePath string, definition shared.Definition) error {
	var err error
	var checksum []byte

	checksumPath := imagePath + ".sha256"

	valid, err := s.VerifyFile(checksumPath, "")
	if err == nil && valid {
		checksum, err = s.GetSignedContent(checksumPath)
	} else {
		checksum, err = os.ReadFile(checksumPath)
	}

	if err != nil {
		return fmt.Errorf("Failed to read checksum file: %w", err)
	}

	image, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("Failed to open %q: %w", imagePath, err)
	}

	defer image.Close()

	hash := sha256.New()

	_, err = io.Copy(hash, image)
	if err != nil {
		return fmt.Errorf("Failed to copy tarball content: %w", err)
	}

	result := fmt.Sprintf("%x", hash.Sum(nil))
	checksumStr := strings.TrimSpace(strings.Split(string(checksum), " ")[0])

	if result != checksumStr {
		return fmt.Errorf("Hash mismatch for %s: %s != %s", imagePath, result, checksumStr)
	}

	return nil
}

func (s *opensuse) getPathToTarball(baseURL string, release string, arch string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("Failed to parse URL %q: %w", baseURL, err)
	}

	var tarballName string

	if strings.ToLower(release) == "tumbleweed" {
		u.Path = path.Join(u.Path, "repositories", "Virtualization:", "containers:", "images:", "openSUSE-Tumbleweed")

		switch arch {
		case "i686", "x86_64":
			u.Path = path.Join(u.Path, "container")
		case "aarch64":
			u.Path = path.Join(u.Path, "container_ARM")
		case "ppc64le":
			u.Path = path.Join(u.Path, "container_PowerPC")
		case "s390x":
			u.Path = path.Join(u.Path, "container_zSystems")
		default:
			return "", fmt.Errorf("Unsupported architecture %q", arch)
		}

		release = "tumbleweed"
	} else {
		u.Path = path.Join(u.Path, "distribution", "leap", release, "appliances")
		release = "leap"
	}

	tarballName, err = s.getTarballName(u, release, arch)
	if err != nil {
		return "", fmt.Errorf("Failed to get tarball name: %w", err)
	}

	u.Path = path.Join(u.Path, tarballName)

	return u.String(), nil
}

func (s *opensuse) getTarballName(u *url.URL, release, arch string) (string, error) {
	doc, err := htmlquery.LoadURL(u.String())
	if err != nil {
		return "", fmt.Errorf("Failed to load URL %q: %w", u.String(), err)
	}

	if doc == nil {
		return "", errors.New("Empty HTML document")
	}

	// Translate x86 architectures.
	if strings.HasSuffix(arch, "86") {
		arch = "ix86"
	}

	nodes := htmlquery.Find(doc, `//a/@href`)
	re := regexp.MustCompile(fmt.Sprintf("^opensuse-%s-image.*%s.*\\.tar.xz$", release, arch))

	var builds []string

	for _, n := range nodes {
		text := strings.TrimPrefix(htmlquery.InnerText(n), "./")

		if !re.MatchString(text) {
			continue
		}

		if strings.Contains(text, "Build") {
			builds = append(builds, text)
		} else {
			if !s.validateURL(*u, text) {
				continue
			}

			return text, nil
		}
	}

	if len(builds) > 0 {
		// Unfortunately, the link to the latest build is missing, hence we need
		// to manually select the latest build.
		sort.Strings(builds)

		for i := len(builds) - 1; i >= 0; i-- {
			if !s.validateURL(*u, builds[i]) {
				continue
			}

			return builds[i], nil
		}
	}

	return "", errors.New("Failed to find tarball name")
}

func (s *opensuse) validateURL(u url.URL, tarball string) bool {
	u.Path = path.Join(u.Path, tarball)

	resp, err := http.Head(u.String())
	if err != nil {
		return false
	}

	// Check whether the link to the tarball is valid.
	if resp.StatusCode == http.StatusNotFound {
		return false
	}

	return true
}
