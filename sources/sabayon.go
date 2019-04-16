package sources

import (
	"crypto/md5"
	"fmt"
	"net/http"
	"net/url"
	"path"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// SabayonHTTP represents the Sabayon Linux downloader.
type SabayonHTTP struct{}

// NewSabayonHTTP creates a new SabayonHTTP instance.
func NewSabayonHTTP() *SabayonHTTP {
	return &SabayonHTTP{}
}

// Run downloads a Sabayon tarball.
func (s *SabayonHTTP) Run(definition shared.Definition, rootfsDir string) error {
	var baseURL string

	fname := fmt.Sprintf("Sabayon_Linux_DAILY_%s_tarball.tar.gz",
		definition.Image.ArchitectureMapped)
	tarballPath := fmt.Sprintf("%s/%s", definition.Source.URL, fname)

	resp, err := http.Head(tarballPath)
	if err != nil {
		return fmt.Errorf("Couldn't resolve URL: %v", err)
	}

	baseURL, fname = path.Split(resp.Request.URL.String())

	url, err := url.Parse(fmt.Sprintf("%s/%s", baseURL, fname))
	if err != nil {
		return err
	}

	var fpath string

	// From sabayon currently we have only MD5 checksum for now.
	if definition.Source.SkipVerification {
		fpath, err = shared.DownloadHash(definition.Image, url.String(), "", nil)
	} else {
		fpath, err = shared.DownloadHash(definition.Image, url.String(), url.String()+".md5", md5.New())
	}
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
