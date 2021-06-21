package sources

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/lxc/distrobuilder/shared"
	"github.com/pkg/errors"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/antchfx/htmlquery.v1"
)

type archlinux struct {
	common
}

// Run downloads an Arch Linux tarball.
func (s *archlinux) Run() error {
	release := s.definition.Image.Release

	// Releases are only available for the x86_64 architecture. ARM only has
	// a "latest" tarball.
	if s.definition.Image.ArchitectureMapped == "x86_64" && release == "" {
		var err error

		// Get latest release
		release, err = s.getLatestRelease(s.definition.Source.URL, s.definition.Image.ArchitectureMapped)
		if err != nil {
			return errors.Wrap(err, "Failed to get latest release")
		}
	}

	var fname string
	var tarball string

	if s.definition.Image.ArchitectureMapped == "x86_64" {
		fname = fmt.Sprintf("archlinux-bootstrap-%s-%s.tar.gz",
			release, s.definition.Image.ArchitectureMapped)
		tarball = fmt.Sprintf("%s/%s/%s", s.definition.Source.URL,
			release, fname)
	} else {
		fname = fmt.Sprintf("ArchLinuxARM-%s-latest.tar.gz",
			s.definition.Image.ArchitectureMapped)
		tarball = fmt.Sprintf("%s/os/%s", s.definition.Source.URL, fname)
	}

	url, err := url.Parse(tarball)
	if err != nil {
		return errors.Wrapf(err, "Failed to parse URL %q", tarball)
	}

	if !s.definition.Source.SkipVerification && url.Scheme != "https" &&
		len(s.definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	fpath, err := shared.DownloadHash(s.definition.Image, tarball, "", nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", tarball)
	}

	// Force gpg checks when using http
	if !s.definition.Source.SkipVerification && url.Scheme != "https" {
		shared.DownloadHash(s.definition.Image, tarball+".sig", "", nil)

		valid, err := shared.VerifyFile(
			filepath.Join(fpath, fname),
			filepath.Join(fpath, fname+".sig"),
			s.definition.Source.Keys,
			s.definition.Source.Keyserver)
		if err != nil {
			return errors.Wrapf(err, "Failed to verify %q", fname)
		}
		if !valid {
			return errors.Errorf("Invalid signature for %q", fname)
		}
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	// Move everything inside 'root.<architecture>' (which was is the tarball) to its
	// parent directory
	files, err := filepath.Glob(fmt.Sprintf("%s/*", filepath.Join(s.rootfsDir,
		"root."+s.definition.Image.ArchitectureMapped)))
	if err != nil {
		return errors.Wrap(err, "Failed to get files")
	}

	for _, file := range files {
		err = os.Rename(file, filepath.Join(s.rootfsDir, path.Base(file)))
		if err != nil {
			return errors.Wrapf(err, "Failed to rename file %q", file)
		}
	}

	path := filepath.Join(s.rootfsDir, "root."+s.definition.Image.ArchitectureMapped)

	err = os.RemoveAll(path)
	if err != nil {
		return errors.Wrapf(err, "Failed to remove %q", path)
	}

	return nil
}

func (s *archlinux) getLatestRelease(URL string, arch string) (string, error) {
	doc, err := htmlquery.LoadURL(URL)
	if err != nil {
		return "", errors.Wrapf(err, "Failed to load URL %q", URL)
	}

	re := regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}/?$`)

	var releases []string

	for _, node := range htmlquery.Find(doc, `//a[@href]/text()`) {
		if re.MatchString(node.Data) {
			releases = append(releases, strings.TrimSuffix(node.Data, "/"))
		}
	}

	if len(releases) == 0 {
		return "", errors.Errorf("Failed to determine latest release")
	}

	// Sort releases in case they're out-of-order
	sort.Strings(releases)

	return releases[len(releases)-1], nil
}
