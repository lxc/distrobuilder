package sources

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lxc/distrobuilder/shared"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/antchfx/htmlquery.v1"
)

// ArchLinuxHTTP represents the Arch Linux downloader.
type ArchLinuxHTTP struct{}

// NewArchLinuxHTTP creates a new ArchLinuxHTTP instance.
func NewArchLinuxHTTP() *ArchLinuxHTTP {
	return &ArchLinuxHTTP{}
}

// Run downloads an Arch Linux tarball.
func (s *ArchLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {
	release := definition.Image.Release

	// Releases are only available for the x86_64 architecture. ARM only has
	// a "latest" tarball.
	if definition.Image.ArchitectureMapped == "x86_64" && release == "" {
		var err error

		// Get latest release
		release, err = s.getLatestRelease(definition.Source.URL, definition.Image.ArchitectureMapped)
		if err != nil {
			return err
		}
	}

	var fname string
	var tarball string

	if definition.Image.ArchitectureMapped == "x86_64" {
		fname = fmt.Sprintf("archlinux-bootstrap-%s-%s.tar.gz",
			release, definition.Image.ArchitectureMapped)
		tarball = fmt.Sprintf("%s/%s/%s", definition.Source.URL,
			release, fname)
	} else {
		fname = fmt.Sprintf("ArchLinuxARM-%s-latest.tar.gz",
			definition.Image.ArchitectureMapped)
		tarball = fmt.Sprintf("%s/os/%s", definition.Source.URL, fname)
	}

	url, err := url.Parse(tarball)
	if err != nil {
		return err
	}

	if !definition.Source.SkipVerification && url.Scheme != "https" &&
		len(definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	fpath, err := shared.DownloadHash(definition.Image, tarball, "", nil)
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if !definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(definition.Image, tarball+".sig", "", nil)

		valid, err := shared.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".sig"),
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

	// Move everything inside 'root.<architecture>' (which was is the tarball) to its
	// parent directory
	files, err := filepath.Glob(fmt.Sprintf("%s/*", filepath.Join(rootfsDir,
		"root."+definition.Image.ArchitectureMapped)))
	if err != nil {
		return err
	}

	for _, file := range files {
		err = os.Rename(file, filepath.Join(rootfsDir, path.Base(file)))
		if err != nil {
			return err
		}
	}

	return os.RemoveAll(filepath.Join(rootfsDir, "root."+
		definition.Image.ArchitectureMapped))
}

func (s *ArchLinuxHTTP) getLatestRelease(URL string, arch string) (string, error) {
	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return "", err
	}

	re := regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}/?$`)

	var releases []string

	for _, node := range htmlquery.Find(doc, `//a[@href]/text()`) {
		if re.MatchString(node.Data) {
			releases = append(releases, strings.TrimSuffix(node.Data, "/"))
		}
	}

	if len(releases) == 0 {
		return "", fmt.Errorf("Failed to determine latest release")
	}

	// Sort releases in case they're out-of-order
	sort.Strings(releases)

	return releases[len(releases)-1], nil
}
