package sources

import (
	"crypto/sha256"
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

type voidlinux struct {
	common
}

// Run downloads a Void Linux rootfs tarball.
func (s *voidlinux) Run() error {
	baseURL := s.definition.Source.URL
	fname, err := s.getLatestBuild(baseURL, s.definition.Image.ArchitectureMapped, s.definition.Source.Variant)
	if err != nil {
		return err
	}

	if fname == "" {
		return errors.New("Failed to determine latest build")
	}

	tarball := fmt.Sprintf("%s/%s", baseURL, fname)
	digests := fmt.Sprintf("%s/sha256sum.txt", baseURL)
	signatures := fmt.Sprintf("%s/sha256sum.sig", baseURL)

	url, err := url.Parse(tarball)
	if err != nil {
		return err
	}

	if !s.definition.Source.SkipVerification && url.Scheme != "https" &&
		len(s.definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	var fpath string

	if s.definition.Source.SkipVerification {
		fpath, err = shared.DownloadHash(s.definition.Image, tarball, "", nil)
	} else {
		fpath, err = shared.DownloadHash(s.definition.Image, tarball, digests, sha256.New())
	}
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(s.definition.Image, digests, "", nil)
		shared.DownloadHash(s.definition.Image, signatures, "", nil)
		valid, err := shared.VerifyFile(
			filepath.Join(fpath, "sha256sum.txt"),
			filepath.Join(fpath, "sha256sum.sig"),
			s.definition.Source.Keys,
			s.definition.Source.Keyserver)
		if err != nil {
			return err
		}
		if !valid {
			return errors.New("Failed to verify tarball")
		}
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	return nil
}

func (s *voidlinux) getLatestBuild(baseURL, arch, variant string) (string, error) {
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
	selector := arch
	if variant != "" {
		selector = fmt.Sprintf("%s-%s", selector, variant)
	}
	regex := regexp.MustCompile(fmt.Sprintf(">void-%s-ROOTFS-.*.tar.xz<", selector))

	// Find all rootfs related files
	matches := regex.FindAllString(string(body), -1)
	if len(matches) > 0 {
		// Take the first match since they're all the same anyway
		return strings.Trim(matches[0], "<>"), nil
	}

	return "", nil
}
