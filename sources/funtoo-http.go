package sources

import (
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"gopkg.in/antchfx/htmlquery.v1"

	"github.com/lxc/distrobuilder/shared"
)

// FuntooHTTP represents the Funtoo downloader.
type FuntooHTTP struct{}

// NewFuntooHTTP creates a new FuntooHTTP instance.
func NewFuntooHTTP() *FuntooHTTP {
	return &FuntooHTTP{}
}

// Run downloads a Funtoo stage3 tarball.
func (s *FuntooHTTP) Run(definition shared.Definition, rootfsDir string) error {
	topLevelArch := definition.Image.ArchitectureMapped
	if topLevelArch == "generic_32" {
		topLevelArch = "x86-32bit"
	} else if topLevelArch == "generic_64" {
		topLevelArch = "x86-64bit"
	} else if topLevelArch == "armv7a_vfpv3_hardfp" {
		topLevelArch = "arm-32bit"
	} else if topLevelArch == "arm64_generic" {
		topLevelArch = "arm-64bit"
	}

	baseURL := fmt.Sprintf("%s/%s-release-std/%s/%s",
		definition.Source.URL, definition.Image.Release,
		topLevelArch, definition.Image.ArchitectureMapped)

	releaseDates, err := s.getReleaseDates(baseURL)
	if err != nil {
		return errors.Wrap(err, "Failed to get release dates")
	}

	var fname string
	var tarball string

	// Find a valid release tarball
	for i := len(releaseDates) - 1; i >= 0; i-- {
		fname = fmt.Sprintf("stage3-%s-%s-release-std-%s.tar.xz", definition.Image.ArchitectureMapped, definition.Image.Release, releaseDates[i])
		tarball = fmt.Sprintf("%s/%s/%s", baseURL, releaseDates[i], fname)

		resp, err := http.Head(tarball)
		if err != nil {
			return errors.Wrapf(err, "Failed to call HEAD on %q", tarball)
		}

		if resp.StatusCode == http.StatusNotFound {
			continue
		}
	}

	url, err := url.Parse(tarball)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse URL %q", tarball)
	}

	if !definition.Source.SkipVerification && url.Scheme != "https" &&
		len(definition.Source.Keys) == 0 {
		return fmt.Errorf("GPG keys are required if downloading from HTTP")
	}

	var fpath string

	fpath, err = shared.DownloadHash(definition.Image, tarball, "", nil)
	if err != nil {
		return errors.Wrap(err, "Failed to download tarball")
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(definition.Image, tarball+".gpg", "", nil)
		valid, err := shared.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".gpg"),
			definition.Source.Keys,
			definition.Source.Keyserver)
		if err != nil {
			return errors.Wrap(err, "Failed to download verification files")
		}
		if !valid {
			return fmt.Errorf("Failed to verify tarball")
		}
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack tarball to %q", rootfsDir)
	}

	return nil
}

func (s *FuntooHTTP) getReleaseDates(URL string) ([]string, error) {
	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to load URL %q", URL)
	}

	re := regexp.MustCompile(`^\d{4}\-\d{2}\-\d{2}/?$`)

	var dirs []string

	for _, node := range htmlquery.Find(doc, `//a[@href]/text()`) {
		if re.MatchString(node.Data) {
			dirs = append(dirs, strings.TrimSuffix(node.Data, "/"))
		}
	}

	if len(dirs) == 0 {
		return nil, fmt.Errorf("Failed to get release dates")
	}

	// Sort dirs in case they're out-of-order
	sort.Strings(dirs)

	return dirs, nil
}
