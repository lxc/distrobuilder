package sources

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

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

	resp, err := http.Head(baseURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to HEAD %q", baseURL)
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
			return errors.Wrap(err, "Failed to get latest release")
		}
	}

	baseURL = fmt.Sprintf("%s/%s/%s/%s/",
		baseURL, exactRelease, s.definition.Image.ArchitectureMapped, s.definition.Image.Variant)
	fname := fmt.Sprintf("ospack_%s-%s-%s_%s.tar.gz",
		release, s.definition.Image.ArchitectureMapped, s.definition.Image.Variant, exactRelease)

	url, err := url.Parse(baseURL)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse %q", baseURL)
	}

	// Force gpg checks when using http
	if url.Scheme != "https" {
		return errors.New("Only HTTPS server is supported")
	}

	fpath, err := shared.DownloadHash(s.definition.Image, baseURL+fname, "", nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", baseURL+fname)
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", fname)
	}

	return nil
}

func (s *apertis) getLatestRelease(baseURL, release string) (string, error) {
	resp, err := http.Get(baseURL)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to GET %q", baseURL)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to ready body")
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)/<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1], nil
	}

	return "", nil
}
