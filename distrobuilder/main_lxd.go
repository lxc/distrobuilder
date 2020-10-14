package main

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/google/uuid"
	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

type cmdLXD struct {
	cmdBuild *cobra.Command
	cmdPack  *cobra.Command
	global   *cmdGlobal

	flagType        string
	flagCompression string
	flagVM          bool
}

func (c *cmdLXD) commandBuild() *cobra.Command {
	c.cmdBuild = &cobra.Command{
		Use:   "build-lxd <filename|-> [target dir] [--type=TYPE] [--compression=COMPRESSION]",
		Short: "Build LXD image from scratch",
		Args:  cobra.RangeArgs(1, 2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !lxd.StringInSlice(c.flagType, []string{"split", "unified"}) {
				return errors.New("--type needs to be one of ['split', 'unified']")
			}

			// Check dependencies
			if c.flagVM {
				err := c.checkVMDependencies()
				if err != nil {
					return err
				}
			}

			return c.global.preRunBuild(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup, overlayDir, err := getOverlay(c.global.flagCacheDir, c.global.sourceDir)
			if err != nil {
				return errors.Wrap(err, "Failed to create overlay")
			}
			defer cleanup()

			return c.run(cmd, args, overlayDir)
		},
	}

	c.cmdBuild.Flags().StringVar(&c.flagType, "type", "split", "Type of tarball to create"+"``")
	c.cmdBuild.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use"+"``")
	c.cmdBuild.Flags().BoolVar(&c.flagVM, "vm", false, "Create a qcow2 image for VMs"+"``")

	return c.cmdBuild
}

func (c *cmdLXD) commandPack() *cobra.Command {
	c.cmdPack = &cobra.Command{
		Use:   "pack-lxd <filename|-> <source dir> [target dir] [--type=TYPE] [--compression=COMPRESSION]",
		Short: "Create LXD image from existing rootfs",
		Args:  cobra.RangeArgs(2, 3),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !lxd.StringInSlice(c.flagType, []string{"split", "unified"}) {
				return errors.New("--type needs to be one of ['split', 'unified']")
			}

			// Check dependencies
			if c.flagVM {
				err := c.checkVMDependencies()
				if err != nil {
					return err
				}
			}

			return c.global.preRunPack(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup, overlayDir, err := getOverlay(c.global.flagCacheDir, c.global.sourceDir)
			if err != nil {
				return errors.Wrap(err, "Failed to create overlay")
			}
			defer cleanup()

			if c.flagVM {
				c.global.definition.Targets.Type = "vm"
			}

			err = c.runPack(cmd, args, overlayDir)
			if err != nil {
				return err
			}

			return c.run(cmd, args, overlayDir)
		},
	}

	c.cmdPack.Flags().StringVar(&c.flagType, "type", "split", "Type of tarball to create")
	c.cmdPack.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use")
	c.cmdPack.Flags().BoolVar(&c.flagVM, "vm", false, "Create a qcow2 image for VMs"+"``")

	return c.cmdPack
}

func (c *cmdLXD) runPack(cmd *cobra.Command, args []string, overlayDir string) error {
	// Return here as we cannot use chroot in Windows.
	if c.global.definition.Source.Downloader == "windows" {
		return nil
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(overlayDir, c.global.definition.Environment, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to setup chroot")
	}
	// Unmount everything and exit the chroot
	defer exitChroot()

	var manager *managers.Manager
	imageTargets := shared.ImageTargetAll

	if c.flagVM {
		imageTargets = shared.ImageTargetVM
	} else {
		imageTargets = shared.ImageTargetContainer
	}

	if c.global.definition.Packages.Manager != "" {
		manager = managers.Get(c.global.definition.Packages.Manager)
		if manager == nil {
			return fmt.Errorf("Couldn't get manager")
		}
	} else {
		manager = managers.GetCustom(*c.global.definition.Packages.CustomManager)
	}

	err = manageRepositories(c.global.definition, manager, imageTargets)
	if err != nil {
		return errors.Wrap(err, "Failed to manage repositories")
	}

	// Run post unpack hook
	for _, hook := range c.global.definition.GetRunnableActions("post-unpack", imageTargets) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return errors.Wrap(err, "Failed to run post-unpack")
		}
	}

	// Install/remove/update packages
	err = managePackages(c.global.definition, manager, imageTargets)
	if err != nil {
		return errors.Wrap(err, "Failed to manage packages")
	}

	// Run post packages hook
	for _, hook := range c.global.definition.GetRunnableActions("post-packages", imageTargets) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return errors.Wrap(err, "Failed to run post-packages")
		}
	}

	return nil
}

func (c *cmdLXD) run(cmd *cobra.Command, args []string, overlayDir string) error {
	img := image.NewLXDImage(overlayDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	imageTargets := shared.ImageTargetUndefined | shared.ImageTargetAll

	if c.flagVM {
		imageTargets |= shared.ImageTargetVM
	} else {
		imageTargets |= shared.ImageTargetContainer
	}

	for _, file := range c.global.definition.Files {
		if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, imageTargets) {
			continue
		}

		generator := generators.Get(file.Generator)
		if generator == nil {
			return fmt.Errorf("Unknown generator '%s'", file.Generator)
		}

		err := generator.RunLXD(c.global.flagCacheDir, overlayDir,
			img, c.global.definition.Targets.LXD, file)
		if err != nil {
			return errors.Wrap(err, "Failed to create LXD data")
		}
	}

	rootfsDir := overlayDir
	var mounts []shared.ChrootMount
	var vmDir string
	var vm *vm
	var targetOS OS

	if c.global.definition.Source.Downloader == "windows" {
		targetOS = OSWindows
	} else {
		targetOS = OSLinux
	}

	if c.flagVM {
		vmDir = filepath.Join(c.global.flagCacheDir, "vm")

		err := os.Mkdir(vmDir, 0755)
		if err != nil {
			return err
		}

		imgFilename, err := shared.RenderTemplate(fmt.Sprintf("%s.raw", c.global.definition.Image.Name), c.global.definition)
		if err != nil {
			return err
		}

		imgFile := filepath.Join(c.global.flagCacheDir, imgFilename)

		vm, err = newVM(imgFile, vmDir, c.global.definition.Targets.LXD.VM.Filesystem, c.global.definition.Targets.LXD.VM.Size, targetOS)
		if err != nil {
			return errors.Wrap(err, "Failed to instanciate VM")
		}

		err = vm.createEmptyDiskImage()
		if err != nil {
			return errors.Wrap(err, "Failed to create disk image")
		}

		err = vm.createPartitions()
		if err != nil {
			return errors.Wrap(err, "Failed to create partitions")
		}

		err = vm.mountImage()
		if err != nil {
			return errors.Wrap(err, "Failed to mount image")
		}
		defer vm.umountImage()

		err = vm.createFilesystems()
		if err != nil {
			return errors.Wrap(err, "Failed to create filesystems")
		}

		err = vm.mountRootFilesystem()
		if err != nil {
			return errors.Wrap(err, "Failed to mount root filesystem")
		}
		defer shared.RunCommand("umount", "-R", vmDir)

		err = vm.mountUEFIFilesystem()
		if err != nil {
			return errors.Wrap(err, "Failed to mount UEFI filesystem")
		}

		// Copy EFI files and BCD to EFI partition, and populate MSR partition.
		if targetOS == OSWindows {
			// This is just a temporary mountpoint which will be removed later.
			targetEFIDir := filepath.Join(vmDir, "boot", "efi")
			targetEFIMicrosoftBootDir := filepath.Join(targetEFIDir, "EFI", "Microsoft", "Boot")
			targetEFIBootDir := filepath.Join(targetEFIDir, "EFI", "Boot")

			err = os.MkdirAll(targetEFIMicrosoftBootDir, 0755)
			if err != nil {
				return errors.Wrapf(err, "Failed to create %s", targetEFIMicrosoftBootDir)
			}

			err = os.MkdirAll(targetEFIBootDir, 0755)
			if err != nil {
				return errors.Wrapf(err, "Failed to create %s", targetEFIBootDir)
			}

			// Copy EFI directory to boot partition
			err = shared.RunCommand("rsync", "-a", "-HA", "--sparse", "--devices", "--checksum", "--numeric-ids", filepath.Join(overlayDir, "Windows", "Boot", "EFI")+"/", targetEFIMicrosoftBootDir)
			if err != nil {
				return errors.Wrap(err, "Failed to copy EFI data")
			}

			// Copy bootx64.efi file
			err = shared.RunCommand("rsync", "-a", "-HA", "--sparse", "--devices", "--checksum", "--numeric-ids", filepath.Join(targetEFIMicrosoftBootDir, "bootmgfw.efi"), filepath.Join(targetEFIBootDir, "bootx64.efi"))
			if err != nil {
				return errors.Wrap(err, "Failed to copy bootx64.efi")
			}

			// Copy BCD file
			bcdFile, err := url.Parse(c.global.definition.Targets.LXD.VM.BCD)
			if err != nil {
				return errors.Wrap(err, "Failed to get BCD file")
			}

			if bcdFile.Scheme == "file" {
				err = shared.RunCommand("rsync", "-a", "-HA", "--sparse", "--devices", "--checksum", "--numeric-ids", bcdFile.Path, filepath.Join(targetEFIMicrosoftBootDir, "BCD"))
				if err != nil {
					return errors.Wrap(err, "Failed to copy BCD file")
				}
			}

			// Fix UUIDs in the BCD file
			efiPartUUID, err := vm.getUEFIPartitionUUID()
			if err != nil {
				return errors.Wrap(err, "Failed to get part UUID of EFI partition")
			}

			dataPartUUID, err := vm.getRootfsPartitionUUID()
			if err != nil {
				return errors.Wrap(err, "Failed to get part UUID of rootfs partition")
			}

			diskUUID, err := vm.getDiskUUID()
			if err != nil {
				return errors.Wrap(err, "Failed to get disk UUID")
			}

			err = replaceUUID(filepath.Join(targetEFIMicrosoftBootDir, "BCD"), uuid.MustParse(c.global.definition.Targets.LXD.VM.UUID.EFI), uuid.MustParse(efiPartUUID))
			if err != nil {
				return errors.Wrap(err, "Failed to replace EFI part UUID")
			}

			err = replaceUUID(filepath.Join(targetEFIMicrosoftBootDir, "BCD"), uuid.MustParse(c.global.definition.Targets.LXD.VM.UUID.Data), uuid.MustParse(dataPartUUID))
			if err != nil {
				return errors.Wrap(err, "Failed to replace rootfs part UUID")
			}

			err = replaceUUID(filepath.Join(targetEFIMicrosoftBootDir, "BCD"), uuid.MustParse(c.global.definition.Targets.LXD.VM.UUID.Disk), uuid.MustParse(diskUUID))
			if err != nil {
				return errors.Wrap(err, "Failed to replace rootfs part UUID")
			}

			err = shared.RunCommand("umount", "-R", targetEFIDir)
			if err != nil {
				return errors.Wrap(err, "Failed to unmount UEFI filesystem")
			}

			err = os.Remove(targetEFIDir)
			if err != nil {
				return errors.Wrap(err, "Failed to remove EFI mountpoint")
			}

			// Copy MSR (Microsoft Reserved Partition)
			msrFile, err := url.Parse(c.global.definition.Targets.LXD.VM.MSR)
			if err != nil {
				return errors.Wrap(err, "Failed to get MSR file")
			}

			if msrFile.Scheme == "file" {
				err = shared.RunCommand("dd", fmt.Sprintf("if=%s", msrFile.Path), fmt.Sprintf("of=%s", vm.getMSRDevFile()))
				if err != nil {
					return errors.Wrap(err, "Failed to copy MSR")
				}
			}
		}

		// We cannot use LXD's rsync package as that uses the --delete flag which
		// causes an issue due to the boot/efi directory being present.
		err = shared.RunCommand("rsync", "-a", "-HA", "--sparse", "--devices", "--checksum", "--numeric-ids", overlayDir+"/", vmDir)
		if err != nil {
			return errors.Wrap(err, "Failed to copy rootfs")
		}

		rootfsDir = vmDir

		mounts = []shared.ChrootMount{
			{
				Source: vm.getLoopDev(),
				Target: filepath.Join("/", "dev", filepath.Base(vm.getLoopDev())),
				Flags:  unix.MS_BIND,
			},
			{
				Source: vm.getRootfsDevFile(),
				Target: filepath.Join("/", "dev", filepath.Base(vm.getRootfsDevFile())),
				Flags:  unix.MS_BIND,
			},
			{
				Source: vm.getUEFIDevFile(),
				Target: filepath.Join("/", "dev", filepath.Base(vm.getUEFIDevFile())),
				Flags:  unix.MS_BIND,
			},
			{
				Source: vm.getUEFIDevFile(),
				Target: "/boot/efi",
				FSType: "vfat",
				Flags:  0,
				Data:   "",
				IsDir:  true,
			},
		}
	}

	if targetOS == OSLinux {
		exitChroot, err := shared.SetupChroot(rootfsDir,
			c.global.definition.Environment, mounts)
		if err != nil {
			return errors.Wrap(err, "Failed to chroot")
		}

		// Run post files hook
		for _, action := range c.global.definition.GetRunnableActions("post-files", imageTargets) {
			err := shared.RunScript(action.Action)
			if err != nil {
				exitChroot()
				return errors.Wrap(err, "Failed to run post-files")
			}
		}

		exitChroot()
	}

	// Unmount VM directory and loop device before creating the image.
	if c.flagVM {
		err := shared.RunCommand("umount", "-R", vmDir)
		if err != nil {
			return err
		}

		err = vm.umountImage()
		if err != nil {
			return err
		}
	}

	err := img.Build(c.flagType == "unified", c.flagCompression, c.flagVM)
	if err != nil {
		return errors.Wrap(err, "Failed to create LXD image")
	}

	return nil
}

func (c *cmdLXD) checkVMDependencies() error {
	dependencies := []string{"btrfs", "mkfs.ext4", "mkfs.ntfs", "mkfs.vfat", "qemu-img", "rsync", "sgdisk"}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return fmt.Errorf("Required tool %q is missing", dep)
		}
	}

	return nil
}
