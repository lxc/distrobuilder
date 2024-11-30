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
	hydraProject := "nixos"

	hydraJobset := fmt.Sprintf("release-%s", s.definition.Image.Release)
	releaseAttr := "incusContainerImage"
	hydraBuildProduct := "squashfs-image"

	if s.definition.Image.Release == "24.05" {
		releaseAttr = "lxdContainerImage"
		hydraBuildProduct = "system-tarball"
	}

	if s.definition.Image.Release == "unstable" {
		hydraJobset = "trunk-combined"
	}

	hydraJob := fmt.Sprintf("nixos.%s.%s-linux", releaseAttr, s.definition.Image.ArchitectureMapped)

	imageURL := fmt.Sprintf("https://hydra.nixos.org/job/%s/%s/%s/latest/download-by-type/file/%s", hydraProject, hydraJobset, hydraJob, hydraBuildProduct)

	fpath, err := s.DownloadHash(s.definition.Image, imageURL, "", nil)
	if err != nil {
		return fmt.Errorf("Failed downloading rootfs: %w", err)
	}

	err = shared.Unpack(filepath.Join(fpath, hydraBuildProduct), s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed unpacking rootfs: %w", err)
	}

	return nil
}
