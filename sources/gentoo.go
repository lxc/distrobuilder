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
	baseURL := fmt.Sprintf("%s/releases/%s/autobuilds/current-install-%s-minimal",
		definition.Source.URL, definition.Image.ArchitectureMapped,
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

	if definition.Source.SkipVerification {
		err = shared.DownloadSha512(tarball, "")
	} else {
		err = shared.DownloadSha512(tarball, tarball+".DIGESTS")
	}
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadSha512(tarball+".DIGESTS.asc", "")
		valid, err := shared.VerifyFile(
			filepath.Join(os.TempDir(), fname+".DIGESTS.asc"),
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
	err = lxd.Unpack(filepath.Join(os.TempDir(), fname), rootfsDir, false, false)
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

	regex := regexp.MustCompile(fmt.Sprintf("stage3-%s-\\d{8}T\\d{6}Z.tar.xz", arch))

	// Find all stage3 related files
	matches := regex.FindAllString(string(body), -1)
	if len(matches) > 1 {
		// Take the first match since they're all the same anyway
		return matches[0], nil
	}

	return "", nil
}
