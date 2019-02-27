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

// GentooHTTP represents the Alpine Linux downloader.
type GentooHTTP struct{}

// NewGentooHTTP creates a new GentooHTTP instance.
func NewGentooHTTP() *GentooHTTP {
	return &GentooHTTP{}
}

// Run downloads a Gentoo stage3 tarball.
func (s *GentooHTTP) Run(definition shared.Definition, rootfsDir string) error {
	topLevelArch := definition.Image.ArchitectureMapped
	if topLevelArch == "i686" {
		topLevelArch = "x86"
	} else if strings.HasPrefix(topLevelArch, "arm") {
		topLevelArch = "arm"
	} else if strings.HasPrefix(topLevelArch, "ppc") {
		topLevelArch = "ppc"
	} else if strings.HasPrefix(topLevelArch, "s390") {
		topLevelArch = "s390"
	}

	baseURL := fmt.Sprintf("%s/releases/%s/autobuilds/current-stage3-%s",
		definition.Source.URL, topLevelArch,
		definition.Image.ArchitectureMapped)
	fname, err := s.getLatestBuild(baseURL, definition.Image.ArchitectureMapped)
	if err != nil {
		return err
	}

	if fname == "" {
		return errors.New("Failed to determine latest build")
	}

	tarball := fmt.Sprintf("%s/%s", baseURL, fname)

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
		fpath, err = shared.DownloadHash(definition.Image, tarball, tarball+".DIGESTS", sha512.New())
	}
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(definition.Image, tarball+".DIGESTS.asc", "", nil)
		valid, err := shared.VerifyFile(
			filepath.Join(fpath, fname+".DIGESTS.asc"),
			"",
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

	return nil
}

func (s *GentooHTTP) getLatestBuild(baseURL, arch string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Look for .tar.xz
	regex := regexp.MustCompile(fmt.Sprintf(">stage3-%s-.*.tar.xz<", arch))

	// Find all stage3 related files
	matches := regex.FindAllString(string(body), -1)
	if len(matches) > 0 {
		// Take the first match since they're all the same anyway
		return strings.Trim(matches[0], "<>"), nil
	}

	// Look for .tar.bz2
	regex = regexp.MustCompile(fmt.Sprintf(">stage3-%s-.*.tar.bz2<", arch))

	// Find all stage3 related files
	matches = regex.FindAllString(string(body), -1)
	if len(matches) > 0 {
		// Take the first match since they're all the same anyway
		return strings.Trim(matches[0], "<>"), nil
	}

	return "", nil
}
