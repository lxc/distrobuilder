package sources

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

type commonRHEL struct {
	common
}

func (c *commonRHEL) unpackISO(filePath, rootfsDir string, scriptRunner func(string) error) error {
	isoDir := filepath.Join(c.cacheDir, "iso")
	squashfsDir := filepath.Join(c.cacheDir, "squashfs")
	roRootDir := filepath.Join(c.cacheDir, "rootfs.ro")
	tempRootDir := filepath.Join(c.cacheDir, "rootfs")

	for _, dir := range []string{isoDir, squashfsDir, roRootDir} {
		err := os.MkdirAll(dir, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", dir)
		}
	}

	// this is easier than doing the whole loop thing ourselves
	err := shared.RunCommand("mount", "-o", "ro", filePath, isoDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q", filePath)
	}
	defer unix.Unmount(isoDir, 0)

	var rootfsImage string
	squashfsImage := filepath.Join(isoDir, "LiveOS", "squashfs.img")
	if lxd.PathExists(squashfsImage) {
		// The squashfs.img contains an image containing the rootfs, so first
		// mount squashfs.img
		err = shared.RunCommand("mount", "-o", "ro", squashfsImage, squashfsDir)
		if err != nil {
			return errors.Wrapf(err, "Failed to mount %q", squashfsImage)
		}
		defer unix.Unmount(squashfsDir, 0)

		rootfsImage = filepath.Join(squashfsDir, "LiveOS", "rootfs.img")
	} else {
		rootfsImage = filepath.Join(isoDir, "images", "install.img")
	}

	// Remove rootfsDir otherwise rsync will copy the content into the directory
	// itself
	err = os.RemoveAll(rootfsDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to remove directory %q", rootfsDir)
	}

	err = c.unpackRootfsImage(rootfsImage, tempRootDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", rootfsImage)
	}

	gpgKeysPath := ""

	packagesDir := filepath.Join(isoDir, "Packages")
	repodataDir := filepath.Join(isoDir, "repodata")

	if !lxd.PathExists(packagesDir) {
		packagesDir = filepath.Join(isoDir, "BaseOS", "Packages")
	}
	if !lxd.PathExists(repodataDir) {
		repodataDir = filepath.Join(isoDir, "BaseOS", "repodata")
	}

	if lxd.PathExists(packagesDir) && lxd.PathExists(repodataDir) {
		// Create cdrom repo for yum
		err = os.MkdirAll(filepath.Join(tempRootDir, "mnt", "cdrom"), 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", filepath.Join(tempRootDir, "mnt", "cdrom"))
		}

		// Copy repo relevant files to the cdrom
		err = shared.RunCommand("rsync", "-qa",
			packagesDir,
			repodataDir,
			filepath.Join(tempRootDir, "mnt", "cdrom"))
		if err != nil {
			return errors.Wrap(err, `Failed to run "rsync"`)
		}

		// Find all relevant GPG keys
		gpgKeys, err := filepath.Glob(filepath.Join(isoDir, "RPM-GPG-KEY-*"))
		if err != nil {
			return errors.Wrap(err, "Failed to match gpg keys")
		}

		// Copy the keys to the cdrom
		for _, key := range gpgKeys {
			fmt.Printf("key=%v\n", key)
			if len(gpgKeysPath) > 0 {
				gpgKeysPath += " "
			}
			gpgKeysPath += fmt.Sprintf("file:///mnt/cdrom/%s", filepath.Base(key))

			err = shared.RunCommand("rsync", "-qa", key,
				filepath.Join(tempRootDir, "mnt", "cdrom"))
			if err != nil {
				return errors.Wrap(err, `Failed to run "rsync"`)
			}
		}
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to setup chroot")
	}

	err = scriptRunner(gpgKeysPath)
	if err != nil {
		exitChroot()
		return errors.Wrap(err, "Failed to run script")
	}

	exitChroot()

	err = shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
	if err != nil {
		return errors.Wrap(err, `Failed to run "rsync"`)
	}

	return nil
}

func (c *commonRHEL) unpackRootfsImage(imageFile string, target string) error {
	installDir, err := ioutil.TempDir(c.cacheDir, "temp_")
	if err != nil {
		return errors.Wrap(err, "Failed to create temporary directory")
	}
	defer os.RemoveAll(installDir)

	err = shared.RunCommand("mount", "-o", "ro", imageFile, installDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q", imageFile)
	}
	defer unix.Unmount(installDir, 0)

	rootfsDir := installDir
	rootfsFile := filepath.Join(installDir, "LiveOS", "rootfs.img")

	if lxd.PathExists(rootfsFile) {
		rootfsDir, err = ioutil.TempDir(c.cacheDir, "temp_")
		if err != nil {
			return errors.Wrap(err, "Failed to create temporary directory")
		}
		defer os.RemoveAll(rootfsDir)

		err = shared.RunCommand("mount", "-o", "ro", rootfsFile, rootfsDir)
		if err != nil {
			return errors.Wrapf(err, "Failed to mount %q", rootfsFile)
		}
		defer unix.Unmount(rootfsDir, 0)
	}

	// Since rootfs is read-only, we need to copy it to a temporary rootfs
	// directory in order to create the minimal rootfs.
	err = shared.RunCommand("rsync", "-qa", rootfsDir+"/", target)
	if err != nil {
		return errors.Wrap(err, `Failed to run "rsync"`)
	}

	return nil
}

func (c *commonRHEL) unpackRaw(filePath, rootfsDir string, scriptRunner func() error) error {
	roRootDir := filepath.Join(c.cacheDir, "rootfs.ro")
	tempRootDir := filepath.Join(c.cacheDir, "rootfs")

	err := os.MkdirAll(roRootDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to create directory %q", roRootDir)
	}

	if strings.HasSuffix(filePath, ".raw.xz") {
		// Uncompress raw image
		err := shared.RunCommand("unxz", filePath)
		if err != nil {
			return errors.Wrap(err, `Failed to run "unxz"`)
		}
	}

	rawFilePath := strings.TrimSuffix(filePath, ".xz")

	// Figure out the offset
	var buf bytes.Buffer

	err = lxd.RunCommandWithFds(nil, &buf, "fdisk", "-l", "-o", "Start", rawFilePath)
	if err != nil {
		return errors.Wrap(err, `Failed to run "fdisk"`)
	}

	output := strings.Split(buf.String(), "\n")
	offsetStr := strings.TrimSpace(output[len(output)-2])

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return errors.Wrapf(err, "Failed to convert %q", offsetStr)
	}

	// Mount the partition read-only since we don't want to accidently modify it.
	err = shared.RunCommand("mount", "-o", fmt.Sprintf("ro,loop,offset=%d", offset*512),
		rawFilePath, roRootDir)
	if err != nil {
		return errors.Wrapf(err, "Failed to mount %q", rawFilePath)
	}
	defer unix.Unmount(roRootDir, 0)

	// Since roRootDir is read-only, we need to copy it to a temporary rootfs
	// directory in order to create the minimal rootfs.
	err = shared.RunCommand("rsync", "-qa", roRootDir+"/", tempRootDir)
	if err != nil {
		return errors.Wrapf(err, `Failed to run "rsync"`)
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(tempRootDir, shared.DefinitionEnv{}, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to setup chroot")
	}

	err = scriptRunner()
	if err != nil {
		exitChroot()
		return errors.Wrap(err, "Failed to run script")
	}

	exitChroot()

	err = shared.RunCommand("rsync", "-qa", tempRootDir+"/rootfs/", rootfsDir)
	if err != nil {
		return errors.Wrap(err, `Failed to run "rsync"`)
	}

	return nil
}
