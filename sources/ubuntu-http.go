package sources

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"

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
func (s *UbuntuHTTP) Run(definition shared.Definition, rootfsDir string) error {
	baseURL := fmt.Sprintf("%s/releases/%s/release/", definition.Source.URL,
		definition.Image.Release)

	if strings.ContainsAny(definition.Image.Release, "0123456789") {
		s.fname = fmt.Sprintf("ubuntu-base-%s-base-%s.tar.gz",
			definition.Image.Release, definition.Image.MappedArchitecture)
	} else {
		// if release is non-numerical, find the latest release
		s.fname = getLatestRelease(definition.Source.URL,
			definition.Image.Release, definition.Image.MappedArchitecture)
		if s.fname == "" {
			return fmt.Errorf("Couldn't find latest release")
		}
	}

	url, err := url.Parse(baseURL)
	if err != nil {
		return err
	}

	checksumFile := ""
	// Force gpg checks when using http
	if url.Scheme != "https" {
		if len(definition.Source.Keys) == 0 {
			return errors.New("GPG keys are required if downloading from HTTP")
		}

		checksumFile = baseURL + "SHA256SUMS"
		shared.Download(baseURL+"SHA256SUMS.gpg", "")
		shared.Download(checksumFile, "")

		valid, err := shared.VerifyFile(
			filepath.Join(os.TempDir(), "SHA256SUMS"),
			filepath.Join(os.TempDir(), "SHA256SUMS.gpg"),
			definition.Source.Keys,
			definition.Source.Keyserver)
		if err != nil {
			return err
		}
		if !valid {
			return fmt.Errorf("Failed to validate tarball")
		}
	}

	err = shared.Download(baseURL+s.fname, checksumFile)
	if err != nil {
		return fmt.Errorf("Error downloading Ubuntu image: %s", err)
	}

	err = s.unpack(filepath.Join(os.TempDir(), s.fname), rootfsDir)
	if err != nil {
		return err
	}

	if definition.Source.AptSources != "" {
		// Run the template
		out, err := shared.RenderTemplate(definition.Source.AptSources, definition)
		if err != nil {
			return err
		}

		// Append final new line if missing
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
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
