package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
					return errors.Wrap(err, "Failed to check VM dependencies")
				}
			}

			return c.global.preRunBuild(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			overlayDir, cleanup, err := c.global.getOverlayDir()
			if err != nil {
				return errors.Wrap(err, "Failed to get overlay directory")
			}

			if cleanup != nil {
				c.global.overlayCleanup = cleanup

				defer func() {
					cleanup()
					c.global.overlayCleanup = nil
				}()
			}

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
					return errors.Wrap(err, "Failed to check VM dependencies")
				}
			}

			return c.global.preRunPack(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			overlayDir, cleanup, err := c.global.getOverlayDir()
			if err != nil {
				return errors.Wrap(err, "Failed to get overlay directory")
			}

			if cleanup != nil {
				c.global.overlayCleanup = cleanup

				defer func() {
					cleanup()
					c.global.overlayCleanup = nil
				}()
			}

			if c.flagVM {
				c.global.definition.Targets.Type = "vm"
			}

			err = c.runPack(cmd, args, overlayDir)
			if err != nil {
				return errors.Wrap(err, "Failed to pack image")
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
	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(overlayDir, c.global.definition.Environment, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to setup chroot")
	}
	// Unmount everything and exit the chroot
	defer exitChroot()

	imageTargets := shared.ImageTargetAll

	if c.flagVM {
		imageTargets = shared.ImageTargetVM
	} else {
		imageTargets = shared.ImageTargetContainer
	}

	manager, err := managers.Load(c.global.definition.Packages.Manager, c.global.logger, *c.global.definition)
	if err != nil {
		return errors.Wrapf(err, "Failed to load manager %q", c.global.definition.Packages.Manager)
	}

	err = manager.ManageRepositories(imageTargets)
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
	err = manager.ManagePackages(imageTargets)
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

		generator, err := generators.Load(file.Generator, c.global.logger, c.global.flagCacheDir, overlayDir, file)
		if err != nil {
			return errors.Wrapf(err, "Failed to load generator %q", file.Generator)
		}

		err = generator.RunLXD(img, c.global.definition.Targets.LXD)
		if err != nil {
			return errors.Wrap(err, "Failed to create LXD data")
		}
	}

	rootfsDir := overlayDir
	var mounts []shared.ChrootMount
	var vmDir string
	var vm *vm

	if c.flagVM {
		vmDir = filepath.Join(c.global.flagCacheDir, "vm")

		err := os.Mkdir(vmDir, 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", vmDir)
		}

		imgFilename, err := shared.RenderTemplate(fmt.Sprintf("%s.raw", c.global.definition.Image.Name), c.global.definition)
		if err != nil {
			return errors.Wrap(err, "Failed to render template")
		}

		imgFile := filepath.Join(c.global.flagCacheDir, imgFilename)

		vm, err = newVM(imgFile, vmDir, c.global.definition.Targets.LXD.VM.Filesystem, c.global.definition.Targets.LXD.VM.Size)
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

		err = vm.createRootFS()
		if err != nil {
			return errors.Wrap(err, "Failed to create root filesystem")
		}

		err = vm.mountRootPartition()
		if err != nil {
			return errors.Wrap(err, "failed to mount root partion")
		}
		defer lxd.RunCommand("umount", "-R", vmDir)

		err = vm.createUEFIFS()
		if err != nil {
			return errors.Wrap(err, "Failed to create UEFI filesystem")
		}

		err = vm.mountUEFIPartition()
		if err != nil {
			return errors.Wrap(err, "Failed to mount UEFI partition")
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

	exitChroot, err := shared.SetupChroot(rootfsDir,
		c.global.definition.Environment, mounts)
	if err != nil {
		return errors.Wrap(err, "Failed to chroot")
	}

	addSystemdGenerator()

	// Run post files hook
	for _, action := range c.global.definition.GetRunnableActions("post-files", imageTargets) {
		err := shared.RunScript(action.Action)
		if err != nil {
			exitChroot()
			return errors.Wrap(err, "Failed to run post-files")
		}
	}

	exitChroot()

	// Unmount VM directory and loop device before creating the image.
	if c.flagVM {
		_, err := lxd.RunCommand("umount", "-R", vmDir)
		if err != nil {
			return errors.Wrapf(err, "Failed to unmount %q", vmDir)
		}

		err = vm.umountImage()
		if err != nil {
			return errors.Wrap(err, "Failed to unmount image")
		}
	}

	err = img.Build(c.flagType == "unified", c.flagCompression, c.flagVM)
	if err != nil {
		return errors.Wrap(err, "Failed to create LXD image")
	}

	return nil
}

func (c *cmdLXD) checkVMDependencies() error {
	dependencies := []string{"btrfs", "mkfs.ext4", "mkfs.vfat", "qemu-img", "rsync", "sgdisk"}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return errors.Errorf("Required tool %q is missing", dep)
		}
	}

	return nil
}
