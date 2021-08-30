package sources

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"gopkg.in/antchfx/htmlquery.v1"

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
		s.definition.Source.URL = "https://mirrorcache-us.opensuse.org/download"
	}

	tarballPath, err := s.getPathToTarball(s.definition.Source.URL, s.definition.Image.Release,
		s.definition.Image.ArchitectureMapped)
	if err != nil {
		return errors.WithMessage(err, "Failed to get tarball path")
	}

	resp, err := http.Head(tarballPath)
	if err != nil {
		return errors.WithMessagef(err, "Failed to HEAD %q", tarballPath)
	}

	baseURL, fname = path.Split(resp.Request.URL.String())

	url, err := url.Parse(fmt.Sprintf("%s%s", baseURL, fname))
	if err != nil {
		return errors.WithMessagef(err, "Failed to parse %q", fmt.Sprintf("%s%s", baseURL, fname))
	}

	fpath, err := shared.DownloadHash(s.definition.Image, url.String(), "", nil)
	if err != nil {
		return errors.WithMessagef(err, "Failed to download %q", url.String())
	}

	_, err = shared.DownloadHash(s.definition.Image, url.String()+".sha256", "", nil)
	if err != nil {
		return errors.WithMessagef(err, "Failed to download %q", url.String()+".sha256")
	}

	if !s.definition.Source.SkipVerification {
		err = s.verifyTarball(filepath.Join(fpath, fname), s.definition)
		if err != nil {
			return errors.WithMessagef(err, "Failed to verify %q", filepath.Join(fpath, fname))
		}
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return errors.WithMessagef(err, "Failed to unpack %q", filepath.Join(fpath, fname))
	}

	return nil
}

func (s *opensuse) verifyTarball(imagePath string, definition shared.Definition) error {
	var err error
	var checksum []byte

	checksumPath := imagePath + ".sha256"

	valid, err := shared.VerifyFile(checksumPath, "", s.definition.Source.Keys, s.definition.Source.Keyserver)
	if err == nil && valid {
		checksum, err = shared.GetSignedContent(checksumPath, s.definition.Source.Keys, s.definition.Source.Keyserver)
	} else {
		checksum, err = ioutil.ReadFile(checksumPath)
	}
	if err != nil {
		return errors.WithMessage(err, "Failed to read checksum file")
	}

	image, err := os.Open(imagePath)
	if err != nil {
		return errors.WithMessagef(err, "Failed to open %q", imagePath)
	}
	defer image.Close()

	hash := sha256.New()

	_, err = io.Copy(hash, image)
	if err != nil {
		return errors.WithMessage(err, "Failed to copy tarball content")
	}

	result := fmt.Sprintf("%x", hash.Sum(nil))
	checksumStr := strings.TrimSpace(strings.Split(string(checksum), " ")[0])

	if result != checksumStr {
		return errors.Errorf("Hash mismatch for %s: %s != %s", imagePath, result, checksumStr)
	}

	return nil
}

func (s *opensuse) getPathToTarball(baseURL string, release string, arch string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", errors.WithMessagef(err, "Failed to parse URL %q", baseURL)
	}

	u.Path = path.Join(u.Path, "repositories", "Virtualization:", "containers:", "images:")

	if strings.ToLower(release) == "tumbleweed" {
		u.Path = path.Join(u.Path, "openSUSE-Tumbleweed")

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
			return "", errors.Errorf("Unsupported architecture %q", arch)
		}

		tarballName, err := s.getTarballName(u, "tumbleweed", arch)
		if err != nil {
			return "", errors.WithMessage(err, "Failed to get tarball name")
		}

		u.Path = path.Join(u.Path, tarballName)
	} else {
		u.Path = path.Join(u.Path, fmt.Sprintf("openSUSE-Leap-%s", release))

		if release == "15.3" {
			u.Path = path.Join(u.Path, "containers")
		} else {
			switch arch {
			case "x86_64":
				u.Path = path.Join(u.Path, "containers")
			case "aarch64", "ppc64le":
				u.Path = path.Join(u.Path, "containers_ports")
			}
		}

		tarballName, err := s.getTarballName(u, "leap", arch)
		if err != nil {
			return "", errors.WithMessage(err, "Failed to get tarball name")
		}

		u.Path = path.Join(u.Path, tarballName)
	}

	return u.String(), nil
}

func (s *opensuse) getTarballName(u *url.URL, release, arch string) (string, error) {
	doc, err := htmlquery.LoadURL(u.String())
	if err != nil {
		return "", errors.WithMessagef(err, "Failed to load URL %q", u.String())
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
			return text, nil
		}
	}

	if len(builds) > 0 {
		// Unfortunately, the link to the latest build is missing, hence we need
		// to manually select the latest build.
		sort.Strings(builds)
		return builds[len(builds)-1], nil
	}

	return "", errors.New("Failed to find tarball name")
}
