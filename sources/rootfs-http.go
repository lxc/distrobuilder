package sources

import (
	"path"
	"path/filepath"

	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

// RootfsHTTP represents the generic HTTP downloader.
type RootfsHTTP struct{}

// NewRootfsHTTP creates a new RootfsHTTP instance.
func NewRootfsHTTP() *RootfsHTTP {
	return &RootfsHTTP{}
}

// Run downloads a tarball.
func (s *RootfsHTTP) Run(definition shared.Definition, rootfsDir string) error {
	fpath, err := shared.DownloadHash(definition.Image, definition.Source.URL, "", nil)
	if err != nil {
		return errors.Wrap(err, "Failed to download tarball")
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, path.Base(definition.Source.URL)), rootfsDir, false, false, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to unpack tarball")
	}

	return nil
}
