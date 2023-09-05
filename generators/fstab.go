package generators

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type fstab struct {
	common
}

// RunLXC doesn't support the fstab generator.
func (g *fstab) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	return errors.New("fstab generator not supported for LXC")
}

// RunIncus writes to /etc/fstab.
func (g *fstab) RunIncus(img *image.IncusImage, target shared.DefinitionTargetIncus) error {
	f, err := os.Create(filepath.Join(g.sourceDir, "etc/fstab"))
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", filepath.Join(g.sourceDir, "etc/fstab"), err)
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
	if err != nil {
		return fmt.Errorf("Failed to write string to file %q: %w", filepath.Join(g.sourceDir, "etc/fstab"), err)
	}

	return nil
}

// Run does nothing.
func (g *fstab) Run() error {
	return nil
}
