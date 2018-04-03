package sources

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// ArchLinuxHTTP represents the Arch Linux downloader.
type ArchLinuxHTTP struct{}

// NewArchLinuxHTTP creates a new ArchLinuxHTTP instance.
func NewArchLinuxHTTP() *ArchLinuxHTTP {
	return &ArchLinuxHTTP{}
}

// Run downloads an Arch Linux tarball.
func (s *ArchLinuxHTTP) Run(definition shared.Definition, rootfsDir string) error {
	fname := fmt.Sprintf("archlinux-bootstrap-%s-%s.tar.gz",
		definition.Image.Release, definition.Image.ArchitectureMapped)
	tarball := fmt.Sprintf("%s/%s/%s", definition.Source.URL,
		definition.Image.Release, fname)

	url, err := url.Parse(tarball)
	if err != nil {
		return err
	}

	if url.Scheme != "https" && len(definition.Source.Keys) == 0 {
		return errors.New("GPG keys are required if downloading from HTTP")
	}

	err = shared.DownloadSha256(tarball, "")
	if err != nil {
		return err
	}

	// Force gpg checks when using http
	if url.Scheme != "https" {
		shared.DownloadSha256(tarball+".sig", "")

		valid, err := shared.VerifyFile(
			filepath.Join(os.TempDir(), fname),
			filepath.Join(os.TempDir(), fname+".sig"),
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

	// Move everything inside 'root.x86_64' (which was is the tarball) to its
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

	return os.RemoveAll(filepath.Join(rootfsDir, "root",
		definition.Image.ArchitectureMapped))
}
