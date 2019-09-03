package sources

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

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
	architecture := definition.Image.Architecture
	release := definition.Image.Release
	variant := definition.Image.Variant
	serial := definition.Image.Serial

	// https://images.apertis.org/daily/v2020dev0/20190830.0/amd64/minimal/ospack_v2020dev0-amd64-minimal_20190830.0.tar.gz
	baseURL := fmt.Sprintf("%s/%s",
		definition.Source.URL, definition.Source.Variant)
	baseURL = fmt.Sprintf("%s/%s/%s/%s/%s/",
		baseURL, release, serial, architecture, variant)
	fname := fmt.Sprintf("ospack_%s-%s-%s_%s.tar.gz",
		release, architecture, variant, serial)

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
