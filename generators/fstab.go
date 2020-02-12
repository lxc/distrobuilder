package generators

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

// FstabGenerator represents the fstab generator.
type FstabGenerator struct{}

// RunLXC doesn't support the fstab generator.
func (g FstabGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	target shared.DefinitionTargetLXC, defFile shared.DefinitionFile) error {
	return fmt.Errorf("fstab generator not supported for LXC")
}

// RunLXD writes to /etc/fstab.
func (g FstabGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	target shared.DefinitionTargetLXD, defFile shared.DefinitionFile) error {
	f, err := os.Create(filepath.Join(sourceDir, "etc/fstab"))
	if err != nil {
		return err
	}
	defer f.Close()

	content := `LABEL=rootfs  /         %s  %s  0 0
LABEL=UEFI    /boot/efi vfat  defaults  0 0
`

	fs := target.VM.Filesystem

	if fs == "" {
		fs = "ext4"
	}

	options := "defaults"

	if fs == "btrfs" {
		options = fmt.Sprintf("%s,subvol=@", options)
	}

	_, err = f.WriteString(fmt.Sprintf(content, fs, options))
	return err
}

// Run does nothing.
func (g FstabGenerator) Run(string, string, shared.DefinitionFile) error {
	return nil
}
