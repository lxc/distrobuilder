package sources

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

type rootfs struct {
	common
}

// Run downloads a tarball.
func (s *rootfs) Run() error {
	fpath, err := shared.DownloadHash(s.definition.Image, s.definition.Source.URL, "", nil)
	if err != nil {
		return fmt.Errorf("Failed to download %q: %w", s.definition.Source.URL, err)
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, path.Base(s.definition.Source.URL)))

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, path.Base(s.definition.Source.URL)), s.rootfsDir, false, false, nil)
	if err != nil {
		return fmt.Errorf("Failed to unpack %q: %w", filepath.Join(fpath, path.Base(s.definition.Source.URL)), err)
	}

	return nil
}
