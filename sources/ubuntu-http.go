package sources

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/flosch/pongo2.v3"

	"github.com/lxc/distrobuilder/shared"
)

// UbuntuHTTP represents the Ubuntu HTTP downloader.
type UbuntuHTTP struct {
	fname string
}

// NewUbuntuHTTP creates a new UbuntuHTTP instance.
func NewUbuntuHTTP() *UbuntuHTTP {
	return &UbuntuHTTP{}
}

// Run downloads the tarball and unpacks it.
func (s *UbuntuHTTP) Run(source shared.DefinitionSource, release, arch, rootfsDir string) error {
	baseURL := fmt.Sprintf("%s/releases/%s/release/", source.URL, release)

	if strings.ContainsAny(release, "0123456789") {
		s.fname = fmt.Sprintf("ubuntu-base-%s-base-%s.tar.gz", release, arch)
	} else {
		// if release is non-numerical, find the latest release
		s.fname = getLatestRelease(source.URL, release, arch)
		if s.fname == "" {
			return fmt.Errorf("Couldn't find latest release")
		}
	}

	shared.Download(baseURL+"SHA256SUMS.gpg", "")
	shared.Download(baseURL+"SHA256SUMS", "")

	valid, err := shared.VerifyFile(
		filepath.Join(os.TempDir(), "SHA256SUMS"),
		filepath.Join(os.TempDir(), "SHA256SUMS.gpg"),
		source.Keys,
		source.Keyserver)
	if err != nil {
		return err
	}
	if !valid {
		return fmt.Errorf("Failed to validate tarball")
	}

	err = shared.Download(
		baseURL+s.fname,
		baseURL+"SHA256SUMS")
	if err != nil {
		return fmt.Errorf("Error downloading Ubuntu image: %s", err)
	}

	err = s.unpack(filepath.Join(os.TempDir(), s.fname), rootfsDir)
	if err != nil {
		return err
	}

	if source.AptSources != "" {
		ctx := pongo2.Context{
			"source": source,
			// We use an anonymous struct instead of DefinitionImage because we
			// need the mapped architecture, and Release is all one needs in
			// the sources.list.
			"image": struct {
				Release string
			}{
				release,
			},
		}

		out, err := shared.RenderTemplate(source.AptSources, ctx)
		if err != nil {
			return err
		}

		// Replace content of sources.list with the templated content.
		file, err := os.Create(filepath.Join(rootfsDir, "etc", "apt", "sources.list"))
		if err != nil {
			return err
		}
		defer file.Close()

		file.WriteString(out)
	}

	return nil
}

func (s UbuntuHTTP) unpack(filePath, rootDir string) error {
	os.RemoveAll(rootDir)
	os.MkdirAll(rootDir, 0755)

	err := lxd.Unpack(filePath, rootDir, false, false)
	if err != nil {
		return fmt.Errorf("Failed to unpack tarball: %s", err)
	}

	return nil
}

func getLatestRelease(URL, release, arch string) string {
	resp, err := http.Get(URL + path.Join("/", "releases", release, "release"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return ""
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	regex := regexp.MustCompile(fmt.Sprintf("ubuntu-base-\\d{2}\\.\\d{2}(\\.\\d+)?-base-%s.tar.gz", arch))
	releases := regex.FindAllString(string(body), -1)

	if len(releases) > 1 {
		return string(releases[len(releases)-1])
	}

	return ""
}
