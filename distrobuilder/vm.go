package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type vm struct {
	imageFile  string
	loopDevice string
	rootFS     string
	rootfsDir  string
	size       uint
}

func newVM(imageFile, rootfsDir, fs string, size uint) (*vm, error) {
	if fs == "" {
		fs = "ext4"
	}

	if !lxd.StringInSlice(fs, []string{"btrfs", "ext4"}) {
		return nil, fmt.Errorf("Unsupported fs: %s", fs)
	}

	if size == 0 {
		size = 4294967296
	}

	return &vm{imageFile: imageFile, rootfsDir: rootfsDir, rootFS: fs, size: size}, nil
}

func (v *vm) getRootfsDevFile() string {
	if v.loopDevice == "" {
		return ""
	}

	return fmt.Sprintf("%sp2", v.loopDevice)
}

func (v *vm) getUEFIDevFile() string {
	if v.loopDevice == "" {
		return ""
	}

	return fmt.Sprintf("%sp1", v.loopDevice)
}

func (v *vm) createEmptyDiskImage() error {
	f, err := os.Create(v.imageFile)
	if err != nil {
		return errors.Wrapf(err, "Failed to open %s", v.imageFile)
	}
	defer f.Close()

	err = f.Chmod(0600)
	if err != nil {
		return errors.Wrapf(err, "Failed to chmod %s", v.imageFile)
	}

	err = f.Truncate(int64(v.size))
	if err != nil {
		return errors.Wrapf(err, "Failed to create sparse file %s", v.imageFile)
	}

	return nil
}

func (v *vm) createPartitions() error {
	args := [][]string{
		{"--zap-all"},
		{"--new=1::+100M", "-t 1:EF00"},
		{"--new=2::", "-t 2:8300"},
	}

	for _, cmd := range args {
		err := shared.RunCommand("sgdisk", append([]string{v.imageFile}, cmd...)...)
		if err != nil {
			return err
		}
	}

	return nil
}

func (v *vm) mountImage() error {
	// If loopDevice is set, it probably is already mounted.
	if v.loopDevice != "" {
		return nil
	}

	stdout, err := lxd.RunCommand("losetup", "-P", "-f", "--show", v.imageFile)
	if err != nil {
		return err
	}

	v.loopDevice = strings.TrimSpace(stdout)

	return nil
}

func (v *vm) umountImage() error {
	// If loopDevice is empty, the image probably isn't mounted.
	if v.loopDevice == "" {
		return nil
	}

	err := shared.RunCommand("losetup", "-d", v.loopDevice)
	if err != nil {
		return err
	}

	v.loopDevice = ""

	return nil
}

func (v *vm) createRootFS() error {
	if v.loopDevice == "" {
		return fmt.Errorf("Disk image not mounted")
	}

	switch v.rootFS {
	case "btrfs":
		err := shared.RunCommand("mkfs.btrfs", "-f", "-L", "rootfs", v.getRootfsDevFile())
		if err != nil {
			return err
		}

		// Create the root subvolume as well
		err = shared.RunCommand("mount", v.getRootfsDevFile(), v.rootfsDir)
		if err != nil {
			return err
		}
		defer shared.RunCommand("umount", v.rootfsDir)

		return shared.RunCommand("btrfs", "subvolume", "create", fmt.Sprintf("%s/@", v.rootfsDir))
	case "ext4":
		return shared.RunCommand("mkfs.ext4", "-F", "-b", "4096", "-i 8192", "-m", "0", "-L", "rootfs", "-E", "resize=536870912", v.getRootfsDevFile())
	}

	return nil
}

func (v *vm) createUEFIFS() error {
	if v.loopDevice == "" {
		return fmt.Errorf("Disk image not mounted")
	}

	return shared.RunCommand("mkfs.vfat", "-F", "32", "-n", "UEFI", v.getUEFIDevFile())
}

func (v *vm) getRootfsPartitionUUID() (string, error) {
	if v.loopDevice == "" {
		return "", fmt.Errorf("Disk image not mounted")
	}

	stdout, err := lxd.RunCommand("blkid", "-s", "PARTUUID", "-o", "value", v.getRootfsDevFile())
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout), nil
}

func (v *vm) getUEFIPartitionUUID() (string, error) {
	if v.loopDevice == "" {
		return "", fmt.Errorf("Disk image not mounted")
	}

	stdout, err := lxd.RunCommand("blkid", "-s", "PARTUUID", "-o", "value", v.getUEFIDevFile())
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout), nil
}

func (v *vm) mountRootPartition() error {
	if v.loopDevice == "" {
		return fmt.Errorf("Disk image not mounted")
	}

	switch v.rootFS {
	case "btrfs":
		return shared.RunCommand("mount", v.getRootfsDevFile(), v.rootfsDir, "-o", "defaults,subvol=/@")
	case "ext4":
		return shared.RunCommand("mount", v.getRootfsDevFile(), v.rootfsDir)

	}

	return nil
}

func (v *vm) mountUEFIPartition() error {
	if v.loopDevice == "" {
		return fmt.Errorf("Disk image not mounted")
	}

	mountpoint := filepath.Join(v.rootfsDir, "boot", "efi")

	err := os.MkdirAll(mountpoint, 0755)
	if err != nil {
		return err
	}

	return shared.RunCommand("mount", v.getUEFIDevFile(), mountpoint)
}
