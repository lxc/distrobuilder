package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/shared"
)

// OS represents the operating system.
type OS int

const (
	// OSLinux represents the Linux operating system.
	OSLinux OS = iota
	// OSWindows represents the Windows operating system.
	OSWindows
)

type vm struct {
	imageFile  string
	loopDevice string
	rootFS     string
	rootfsDir  string
	size       uint64
	os         OS
}

func newVM(imageFile, rootfsDir, fs string, size uint64, os OS) (*vm, error) {
	if fs == "" {
		if os == OSLinux {
			fs = "ext4"
		} else {
			fs = "ntfs"
		}
	}

	var supportedFs []string

	if os == OSLinux {
		supportedFs = []string{"btrfs", "ext4"}
	} else {
		supportedFs = []string{"ntfs"}
	}

	if !lxd.StringInSlice(fs, supportedFs) {
		return nil, fmt.Errorf("Unsupported fs: %s", fs)
	}

	if size == 0 {
		size = 4294967296
	}

	return &vm{imageFile: imageFile, rootfsDir: rootfsDir, rootFS: fs, size: size, os: os}, nil
}

func (v *vm) getLoopDev() string {
	return v.loopDevice
}

func (v *vm) getRootfsDevFile() string {
	if v.loopDevice == "" {
		return ""
	}

	var partNum int

	if v.os == OSLinux {
		partNum = 2
	} else {
		partNum = 3
	}

	return fmt.Sprintf("%sp%d", v.loopDevice, partNum)
}

func (v *vm) getUEFIDevFile() string {
	if v.loopDevice == "" {
		return ""
	}

	return fmt.Sprintf("%sp1", v.loopDevice)
}

func (v *vm) getMSRDevFile() string {
	if v.loopDevice == "" {
		return ""
	}

	if v.os != OSWindows {
		return ""
	}

	return fmt.Sprintf("%sp2", v.loopDevice)
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
		// EFI system partition (ESP)
		{"--new=1::+100M", "-t 1:C12A7328-F81F-11D2-BA4B-00A0C93EC93B"},
	}

	if v.os == OSLinux {
		// Linux partition
		args = append(args, []string{"--new=2::", "-t 2:0FC63DAF-8483-4772-8E79-3D69D8477DE4"})
	} else {
		args = append(args,
			// Microsoft Reserved Partition (MSR)
			[]string{"--new=2::+128M", "-t 2:E3C9E316-0B5C-4DB8-817D-F92DF00215AE"},
			/// Microsoft basic data partition
			[]string{"--new=3::", "-t 3:EBD0A0A2-B9E5-4433-87C0-68B6B72699C7"},
		)
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

	// Ensure the partitions are accessible. This part is usually only needed
	// if building inside of a container.

	out, err := lxd.RunCommand("lsblk", "--raw", "--output", "MAJ:MIN", "--noheadings", v.loopDevice)
	if err != nil {
		return err
	}

	deviceNumbers := strings.Split(out, "\n")

	if !lxd.PathExists(v.getUEFIDevFile()) {
		fields := strings.Split(deviceNumbers[1], ":")

		major, err := strconv.Atoi(fields[0])
		if err != nil {
			return err
		}

		minor, err := strconv.Atoi(fields[1])
		if err != nil {
			return err
		}

		dev := unix.Mkdev(uint32(major), uint32(minor))

		err = unix.Mknod(v.getUEFIDevFile(), unix.S_IFBLK|0644, int(dev))
		if err != nil {
			return err
		}
	}

	if !lxd.PathExists(v.getRootfsDevFile()) {
		var idx int

		if v.os == OSLinux {
			idx = 2
		} else {
			idx = 3
		}

		fields := strings.Split(deviceNumbers[idx], ":")

		major, err := strconv.Atoi(fields[0])
		if err != nil {
			return err
		}

		minor, err := strconv.Atoi(fields[1])
		if err != nil {
			return err
		}

		dev := unix.Mkdev(uint32(major), uint32(minor))

		err = unix.Mknod(v.getRootfsDevFile(), unix.S_IFBLK|0644, int(dev))
		if err != nil {
			return err
		}
	}

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

	// Make sure that all partitions are removed.
	for i := 1; i <= 3; i++ {
		partition := fmt.Sprintf("%sp%d", v.loopDevice, i)

		if lxd.PathExists(partition) {
			err := os.Remove(partition)
			if err != nil {
				return err
			}
		}
	}

	v.loopDevice = ""

	return nil
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

func (v *vm) createFilesystems() error {
	if v.loopDevice == "" {
		return fmt.Errorf("Disk image not mounted")
	}

	var err error

	// Create root filesystem
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

		err = shared.RunCommand("btrfs", "subvolume", "create", fmt.Sprintf("%s/@", v.rootfsDir))
	case "ext4":
		err = shared.RunCommand("mkfs.ext4", "-F", "-b", "4096", "-i 8192", "-m", "0", "-L", "rootfs", "-E", "resize=536870912", v.getRootfsDevFile())
	case "ntfs":
		err = shared.RunCommand("mkfs.ntfs", "-f", "-L", "rootfs", v.getRootfsDevFile())
	}
	if err != nil {
		return err
	}

	// Create UEFI filesystem
	return shared.RunCommand("mkfs.vfat", "-F", "32", "-n", "UEFI", v.getUEFIDevFile())
}

func (v *vm) mountRootFilesystem() error {
	if v.loopDevice == "" {
		return fmt.Errorf("Disk image not mounted")
	}

	var err error

	switch v.rootFS {
	case "btrfs":
		err = shared.RunCommand("mount", v.getRootfsDevFile(), v.rootfsDir, "-o", "defaults,subvol=/@")
	case "ext4":
	case "ntfs":
		err = shared.RunCommand("mount", v.getRootfsDevFile(), v.rootfsDir)
	}

	return err
}

func (v *vm) mountUEFIFilesystem() error {
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

func (v *vm) getDiskUUID() (string, error) {
	if v.loopDevice == "" {
		return "", fmt.Errorf("Disk image not mounted")
	}

	stdout, err := lxd.RunCommand("blkid", "-s", "PTUUID", "-o", "value", v.loopDevice)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(stdout), nil
}
