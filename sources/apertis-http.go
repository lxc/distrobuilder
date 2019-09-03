package sources

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// ApertisHTTP represents the Apertis downloader.
type ApertisHTTP struct{}

// NewApertisHTTP creates a new ApertisHTTP instance.
func NewApertisHTTP() *ApertisHTTP {
	return &ApertisHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *ApertisHTTP) Run(definition shared.Definition, rootfsDir string) error {
	release := definition.Image.Release
	exactRelease := release

	// https://images.apertis.org/daily/v2020dev0/20190830.0/amd64/minimal/ospack_v2020dev0-amd64-minimal_20190830.0.tar.gz
	baseURL := fmt.Sprintf("%s/%s/%s",
		definition.Source.URL, definition.Source.Variant, release)

	resp, err := http.Head(baseURL)
	if err != nil {
		return err
	}

	if resp.StatusCode == http.StatusNotFound {
		// Possibly, release is a specific release (18.12.0 instead of 18.12). Lets trim the prefix and continue.
		re := regexp.MustCompile(`\.\d+$`)
		release = strings.TrimSuffix(release, re.FindString(release))
		baseURL = fmt.Sprintf("%s/%s/%s",
			definition.Source.URL, definition.Source.Variant, release)
	} else {
		exactRelease = s.getLatestRelease(baseURL, release)
	}

	baseURL = fmt.Sprintf("%s/%s/%s/%s/",
		baseURL, exactRelease, definition.Image.ArchitectureMapped, definition.Image.Variant)
	fname := fmt.Sprintf("ospack_%s-%s-%s_%s.tar.gz",
		release, definition.Image.ArchitectureMapped, definition.Image.Variant, exactRelease)

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if url.Scheme != "https" {
		return errors.New("Only HTTPS server is supported")
	}

	fpath, err := shared.DownloadHash(definition.Image, baseURL+fname, "", nil)
	if err != nil {
		return err
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *ApertisHTTP) getLatestRelease(baseURL, release string) string {
	resp, err := http.Get(baseURL)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)/<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1]
	}

	return ""
}
