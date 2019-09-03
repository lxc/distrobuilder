package sources

import (
	"crypto/sha256"
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

// OpenWrtHTTP represents the ALT Linux downloader.
type OpenWrtHTTP struct{}

// NewOpenWrtHTTP creates a new OpenWrtHTTP instance.
func NewOpenWrtHTTP() *OpenWrtHTTP {
	return &OpenWrtHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *OpenWrtHTTP) Run(definition shared.Definition, rootfsDir string) error {
	var baseURL string

	release := definition.Image.Release
	releaseInFilename := strings.ToLower(release) + "-"
	architecturePath := strings.Replace(definition.Image.ArchitectureMapped, "_", "/", 1)

	// Figure out the correct release
	if release == "snapshot" {
		// Build a daily snapshot.
		baseURL = fmt.Sprintf("%s/snapshots/targets/%s/",
			definition.Source.URL, architecturePath)
		releaseInFilename = ""
	} else {
		baseURL = fmt.Sprintf("%s/releases", definition.Source.URL)

		matched, err := regexp.MatchString(`^\d+\.\d+$`, release)
		if err != nil {
			return err
		}

		if matched {
			// A release of the form '18.06' has been provided. We need to find
			// out the latest service release of the form '18.06.0'.
			release = s.getLatestServiceRelease(baseURL, release)
			releaseInFilename = strings.ToLower(release) + "-"
		}

		baseURL = fmt.Sprintf("%s/%s/targets/%s/", baseURL, release, architecturePath)
	}

	fname := fmt.Sprintf("openwrt-%s%s-generic-rootfs.tar.gz", releaseInFilename,
		strings.Replace(definition.Image.ArchitectureMapped, "_", "-", 1))

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	if !definition.Source.SkipVerification {
		if len(definition.Source.Keys) != 0 {
			checksumFile = baseURL + "sha256sums"
			fpath, err := shared.DownloadHash(definition.Image, checksumFile+".asc", "", nil)
			if err != nil {
				return err
			}

			_, err = shared.DownloadHash(definition.Image, checksumFile, "", nil)
			if err != nil {
				return err
			}

			valid, err := shared.VerifyFile(
				filepath.Join(fpath, "sha256sums"),
				filepath.Join(fpath, "sha256sums.asc"),
				definition.Source.Keys,
				definition.Source.Keyserver)
			if err != nil {
				return err
			}
			if !valid {
				return fmt.Errorf("Failed to validate archive")
			}
		} else {
			// Force gpg checks when using http
			if url.Scheme != "https" {
				return errors.New("GPG keys are required if downloading from HTTP")
			}
		}
	}

	fpath, err := shared.DownloadHash(definition.Image, baseURL+fname, checksumFile, sha256.New())
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

func (s *OpenWrtHTTP) getLatestServiceRelease(baseURL, release string) string {
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

	regex := regexp.MustCompile(fmt.Sprintf(">(%s\\.\\d+)<", release))
	releases := regex.FindAllStringSubmatch(string(body), -1)

	if len(releases) > 0 {
		return releases[len(releases)-1][1]
	}

	return ""
}
