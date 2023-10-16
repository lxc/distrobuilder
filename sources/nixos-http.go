package sources

import (
	"fmt"
	"path/filepath"

	"github.com/lxc/distrobuilder/shared"
)

type nixos struct {
	common
}

func (s *nixos) Run() error {
	tarballURL := fmt.Sprintf("https://hydra.nixos.org/job/nixos/trunk-combined/nixos.lxdContainerImage.%s-linux/latest/download-by-type/file/system-tarball", s.definition.Image.ArchitectureMapped)

	fpath, err := s.DownloadHash(s.definition.Image, tarballURL, "", nil)
	if err != nil {
		return fmt.Errorf("Failed downloading tarball: %w", err)
	}

	err = shared.Unpack(filepath.Join(fpath, "system-tarball"), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed unpacking rootfs: %w", err)
	}

	return nil
}
