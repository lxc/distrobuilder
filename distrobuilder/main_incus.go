package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"

	client "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

type cmdIncus struct {
	cmdBuild *cobra.Command
	cmdPack  *cobra.Command
	global   *cmdGlobal

	flagType            string
	flagCompression     string
	flagVM              bool
	flagImportIntoIncus string
}

func (c *cmdIncus) commandBuild() *cobra.Command {
	c.cmdBuild = &cobra.Command{
		Use:   "build-incus <filename|-> [target dir] [--type=TYPE] [--compression=COMPRESSION] [--import-into-incus]",
		Short: "Build Incus image from scratch",
		Long: fmt.Sprintf(`Build Incus image from scratch

%s

%s
`, typeDescription, compressionDescription),
		Args: cobra.RangeArgs(1, 2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !slices.Contains([]string{"split", "unified"}, c.flagType) {
				return errors.New("--type needs to be one of ['split', 'unified']")
			}

			// Check compression arguments
			_, _, err := shared.ParseCompression(c.flagCompression)
			if err != nil {
				return fmt.Errorf("Failed to parse compression level: %w", err)
			}

			if c.flagType == "split" {
				_, _, err := shared.ParseSquashfsCompression(c.flagCompression)
				if err != nil {
					return fmt.Errorf("Failed to parse compression level: %w", err)
				}
			}

			// Check dependencies
			if c.flagVM {
				err := c.checkVMDependencies()
				if err != nil {
					return fmt.Errorf("Failed to check VM dependencies: %w", err)
				}
			}

			return c.global.preRunBuild(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			overlayDir, cleanup, err := c.global.getOverlayDir()
			if err != nil {
				return fmt.Errorf("Failed to get overlay directory: %w", err)
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
	c.cmdBuild.Flags().StringVar(&c.flagImportIntoIncus, "import-into-incus", "", "Import built image into Incus"+"``")
	c.cmdBuild.Flags().BoolVar(&c.global.flagKeepSources, "keep-sources", true, "Keep sources after build"+"``")
	c.cmdBuild.Flags().StringVar(&c.global.flagSourcesDir, "sources-dir", filepath.Join(os.TempDir(), "distrobuilder"), "Sources directory for distribution tarballs"+"``")

	return c.cmdBuild
}

func (c *cmdIncus) commandPack() *cobra.Command {
	c.cmdPack = &cobra.Command{
		Use:   "pack-incus <filename|-> <source dir> [target dir] [--type=TYPE] [--compression=COMPRESSION] [--import-into-incus]",
		Short: "Create Incus image from existing rootfs",
		Long: fmt.Sprintf(`Create Incus image from existing rootfs

%s

%s
`, typeDescription, compressionDescription),
		Args: cobra.RangeArgs(2, 3),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !slices.Contains([]string{"split", "unified"}, c.flagType) {
				return errors.New("--type needs to be one of ['split', 'unified']")
			}

			// Check compression arguments
			_, _, err := shared.ParseCompression(c.flagCompression)
			if err != nil {
				return fmt.Errorf("Failed to parse compression level: %w", err)
			}

			if c.flagType == "split" {
				_, _, err := shared.ParseSquashfsCompression(c.flagCompression)
				if err != nil {
					return fmt.Errorf("Failed to parse compression level: %w", err)
				}
			}

			// Check dependencies
			if c.flagVM {
				err := c.checkVMDependencies()
				if err != nil {
					return fmt.Errorf("Failed to check VM dependencies: %w", err)
				}
			}

			return c.global.preRunPack(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			overlayDir, cleanup, err := c.global.getOverlayDir()
			if err != nil {
				return fmt.Errorf("Failed to get overlay directory: %w", err)
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
				return fmt.Errorf("Failed to pack image: %w", err)
			}

			return c.run(cmd, args, overlayDir)
		},
	}

	c.cmdPack.Flags().StringVar(&c.flagType, "type", "split", "Type of tarball to create")
	c.cmdPack.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use")
	c.cmdPack.Flags().BoolVar(&c.flagVM, "vm", false, "Create a qcow2 image for VMs"+"``")
	c.cmdPack.Flags().StringVar(&c.flagImportIntoIncus, "import-into-incus", "", "Import built image into Incus"+"``")
	c.cmdPack.Flags().Lookup("import-into-incus").NoOptDefVal = "-"

	return c.cmdPack
}

func (c *cmdIncus) runPack(cmd *cobra.Command, args []string, overlayDir string) error {
	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(overlayDir, *c.global.definition, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %w", err)
	}
	// Unmount everything and exit the chroot
	defer func() {
		_ = exitChroot()
	}()

	imageTargets := shared.ImageTargetAll

	if c.flagVM {
		imageTargets |= shared.ImageTargetVM
	} else {
		imageTargets |= shared.ImageTargetContainer
	}

	manager, err := managers.Load(c.global.ctx, c.global.definition.Packages.Manager, c.global.logger, *c.global.definition)
	if err != nil {
		return fmt.Errorf("Failed to load manager %q: %w", c.global.definition.Packages.Manager, err)
	}

	c.global.logger.Info("Managing repositories")

	err = manager.ManageRepositories(imageTargets)
	if err != nil {
		return fmt.Errorf("Failed to manage repositories: %w", err)
	}

	c.global.logger.WithField("trigger", "post-unpack").Info("Running hooks")

	// Run post unpack hook
	for _, hook := range c.global.definition.GetRunnableActions("post-unpack", imageTargets) {
		if hook.Pongo {
			hook.Action, err = shared.RenderTemplate(hook.Action, c.global.definition)
			if err != nil {
				return fmt.Errorf("Failed to render action: %w", err)
			}
		}

		err := shared.RunScript(c.global.ctx, hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-unpack: %w", err)
		}
	}

	c.global.logger.Info("Managing packages")

	// Install/remove/update packages
	err = manager.ManagePackages(imageTargets)
	if err != nil {
		return fmt.Errorf("Failed to manage packages: %w", err)
	}

	c.global.logger.WithField("trigger", "post-packages").Info("Running hooks")

	// Run post packages hook
	for _, hook := range c.global.definition.GetRunnableActions("post-packages", imageTargets) {
		if hook.Pongo {
			hook.Action, err = shared.RenderTemplate(hook.Action, c.global.definition)
			if err != nil {
				return fmt.Errorf("Failed to render action: %w", err)
			}
		}

		err := shared.RunScript(c.global.ctx, hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-packages: %w", err)
		}
	}

	return nil
}

func (c *cmdIncus) run(cmd *cobra.Command, args []string, overlayDir string) error {
	img := image.NewIncusImage(c.global.ctx, overlayDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	imageTargets := shared.ImageTargetUndefined | shared.ImageTargetAll

	if c.flagVM {
		imageTargets |= shared.ImageTargetVM
	} else {
		imageTargets |= shared.ImageTargetContainer
	}

	// Maps symbolic user/group names to their numeric IDs using information from passwd and group files.
	userMap, groupMap, err := parsePasswdAndGroupFiles(overlayDir)
	if err != nil {
		c.global.logger.WithField("overlay", overlayDir).Warn("Could not parse passwd/group file: %w", err)
	}

	for i, file := range c.global.definition.Files {
		if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, imageTargets) {
			continue
		}

		if file.UID != "" && !isNumeric(file.UID) {
			uid, exists := userMap[file.UID]
			if exists {
				c.global.definition.Files[i].UID = uid
			} else {
				c.global.logger.WithField("generator", file.Generator).Warnf("Could not find UID for user %q", file.UID)
			}
		}

		if file.UID != "" && !isNumeric(file.GID) {
			gid, exists := groupMap[file.GID]
			if exists {
				c.global.definition.Files[i].GID = gid
			} else {
				c.global.logger.WithField("generator", file.Generator).Warnf("Could not find GID for group %q", file.GID)
			}
		}

		generator, err := generators.Load(file.Generator, c.global.logger, c.global.flagCacheDir, overlayDir, file, *c.global.definition)
		if err != nil {
			return fmt.Errorf("Failed to load generator %q: %w", file.Generator, err)
		}

		c.global.logger.WithField("generator", file.Generator).Info("Running generator")

		err = generator.RunIncus(img, c.global.definition.Targets.Incus)
		if err != nil {
			return fmt.Errorf("Failed to create Incus data: %w", err)
		}
	}

	rootfsDir := overlayDir
	var mounts []shared.ChrootMount
	var vmDir string
	var vm *vm
	cleanup := true
	if c.flagVM {
		vmDir = filepath.Join(c.global.flagCacheDir, "vm")

		err := os.Mkdir(vmDir, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", vmDir, err)
		}

		imgFilename, err := shared.RenderTemplate(fmt.Sprintf("%s.raw", c.global.definition.Image.Name), c.global.definition)
		if err != nil {
			return fmt.Errorf("Failed to render template: %w", err)
		}

		imgFile := filepath.Join(c.global.flagCacheDir, imgFilename)

		vm, err = newVM(c.global.ctx, imgFile, vmDir, c.global.definition.Targets.Incus.VM.Filesystem, c.global.definition.Targets.Incus.VM.Size)
		if err != nil {
			return fmt.Errorf("Failed to instantiate VM: %w", err)
		}

		err = vm.createEmptyDiskImage()
		if err != nil {
			return fmt.Errorf("Failed to create disk image: %w", err)
		}

		err = vm.createPartitions()
		if err != nil {
			return fmt.Errorf("Failed to create partitions: %w", err)
		}

		err = vm.mountImage()
		if err != nil {
			return fmt.Errorf("Failed to mount image: %w", err)
		}

		defer func() {
			_ = vm.umountImage()
		}()

		err = vm.createRootFS()
		if err != nil {
			return fmt.Errorf("Failed to create root filesystem: %w", err)
		}

		err = vm.mountRootPartition()
		if err != nil {
			return fmt.Errorf("failed to mount root partion: %w", err)
		}

		defer func() {
			if cleanup {
				_ = vm.umountPartition(vmDir)
			}
		}()

		err = vm.createUEFIFS()
		if err != nil {
			return fmt.Errorf("Failed to create UEFI filesystem: %w", err)
		}

		err = vm.mountUEFIPartition()
		if err != nil {
			return fmt.Errorf("Failed to mount UEFI partition: %w", err)
		}

		rootUUID, err := vm.findRootfsDevUUID()
		if err != nil {
			return fmt.Errorf("Failed to find rootfs device UUID: %w", err)
		}

		c.global.ctx = context.WithValue(c.global.ctx, shared.ContextKeyEnviron,
			[]string{fmt.Sprintf("%s=%s", shared.EnvRootUUID, rootUUID)})
		// We cannot use Incus' rsync package as that uses the --delete flag which
		// causes an issue due to the boot/efi directory being present.
		err = shared.RsyncLocal(c.global.ctx, overlayDir+"/", vmDir)
		if err != nil {
			return fmt.Errorf("Failed to copy rootfs: %w", err)
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
		*c.global.definition, mounts)
	if err != nil {
		return fmt.Errorf("Failed to chroot: %w", err)
	}

	err = addSystemdGenerator()
	if err != nil {
		return fmt.Errorf("Failed adding systemd generator: %w", err)
	}

	c.global.logger.WithField("trigger", "post-files").Info("Running hooks")

	// Run post files hook
	for _, action := range c.global.definition.GetRunnableActions("post-files", imageTargets) {
		if action.Pongo {
			action.Action, err = shared.RenderTemplate(action.Action, c.global.definition)
			if err != nil {
				return fmt.Errorf("Failed to render action: %w", err)
			}
		}

		err := shared.RunScript(c.global.ctx, action.Action)
		if err != nil {
			{
				err := exitChroot()
				if err != nil {
					c.global.logger.WithField("err", err).Warn("Failed exiting chroot")
				}
			}

			return fmt.Errorf("Failed to run post-files: %w", err)
		}
	}

	err = exitChroot()
	if err != nil {
		return fmt.Errorf("Failed exiting chroot: %w", err)
	}

	// Unmount VM directory and loop device before creating the image.
	if c.flagVM {
		err := vm.umountPartition(vmDir)
		if err != nil {
			return fmt.Errorf("Failed to unmount %q: %w", vmDir, err)
		}

		cleanup = false
		err = vm.umountImage()
		if err != nil {
			return fmt.Errorf("Failed to unmount image: %w", err)
		}
	}

	c.global.logger.WithFields(logrus.Fields{"type": c.flagType, "vm": c.flagVM, "compression": c.flagCompression}).Info("Creating Incus image")

	imageFile, rootfsFile, err := img.Build(c.flagType == "unified", c.flagCompression, c.flagVM)
	if err != nil {
		return fmt.Errorf("Failed to create Incus image: %w", err)
	}

	importFlag := cmd.Flags().Lookup("import-into-incus")

	if importFlag.Changed {
		path := ""

		server, err := client.ConnectIncusUnix(path, nil)
		if err != nil {
			return fmt.Errorf("Failed to connect to Incus: %w", err)
		}

		image := api.ImagesPost{
			Filename: imageFile,
		}

		imageType := "container"

		var meta io.ReadCloser
		var rootfs io.ReadCloser

		// Open meta
		meta, err = os.Open(imageFile)
		if err != nil {
			return err
		}

		defer meta.Close()

		// Open rootfs
		if rootfsFile != "" {
			rootfs, err = os.Open(rootfsFile)
			if err != nil {
				return err
			}

			defer rootfs.Close()

			if filepath.Ext(rootfsFile) == ".qcow2" {
				imageType = "virtual-machine"
			}
		}

		createArgs := &client.ImageCreateArgs{
			MetaFile:   meta,
			MetaName:   filepath.Base(imageFile),
			RootfsFile: rootfs,
			RootfsName: filepath.Base(rootfsFile),
			Type:       imageType,
		}

		op, err := server.CreateImage(image, createArgs)
		if err != nil {
			return fmt.Errorf("Failed to create image: %w", err)
		}

		err = op.Wait()
		if err != nil {
			return fmt.Errorf("Failed to create image: %w", err)
		}

		// Don't create alias if the flag value is equal to the NoOptDefVal (the default value if --import-into-incus flag is set without any value).
		if importFlag.Value.String() == importFlag.NoOptDefVal {
			return nil
		}

		opAPI := op.Get()

		alias := api.ImageAliasesPost{}
		alias.Target = opAPI.Metadata["fingerprint"].(string)

		alias.Name, err = shared.RenderTemplate(importFlag.Value.String(), c.global.definition)
		if err != nil {
			return fmt.Errorf("Failed to render %q: %w", importFlag.Value.String(), err)
		}

		alias.Description, err = shared.RenderTemplate(c.global.definition.Image.Description, c.global.definition)
		if err != nil {
			return fmt.Errorf("Failed to render %q: %w", c.global.definition.Image.Description, err)
		}

		err = server.CreateImageAlias(alias)
		if err != nil {
			return fmt.Errorf("Failed to create image alias: %w", err)
		}
	}

	return nil
}

func (c *cmdIncus) checkVMDependencies() error {
	dependencies := []string{"btrfs", "mkfs.ext4", "mkfs.vfat", "qemu-img", "rsync", "sgdisk"}

	for _, dep := range dependencies {
		_, err := exec.LookPath(dep)
		if err != nil {
			return fmt.Errorf("Required tool %q is missing", dep)
		}
	}

	return nil
}
