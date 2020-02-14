package main

import (
	"fmt"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

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

			return c.global.preRunPack(cmd, args)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup, overlayDir, err := getOverlay(c.global.flagCacheDir, c.global.sourceDir)
			if err != nil {
				return errors.Wrap(err, "Failed to create overlay")
			}
			defer cleanup()

			err = c.runPack(cmd, args, overlayDir)
			if err != nil {
				return err
			}

			return c.run(cmd, args, overlayDir)
		},
	}

	c.cmdPack.Flags().StringVar(&c.flagType, "type", "split", "Type of tarball to create")
	c.cmdPack.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use")

	return c.cmdPack
}

func (c *cmdLXD) runPack(cmd *cobra.Command, args []string, overlayDir string) error {
	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(overlayDir, c.global.definition.Environment, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}
	// Unmount everything and exit the chroot
	defer exitChroot()

	var manager *managers.Manager
	imageTargets := shared.ImageTargetContainer

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
		return fmt.Errorf("Failed to manage repositories: %s", err)
	}

	// Run post unpack hook
	for _, hook := range c.global.definition.GetRunnableActions("post-unpack", imageTargets) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-unpack: %s", err)
		}
	}

	// Install/remove/update packages
	err = managePackages(c.global.definition, manager, imageTargets)
	if err != nil {
		return fmt.Errorf("Failed to manage packages: %s", err)
	}

	// Run post packages hook
	for _, hook := range c.global.definition.GetRunnableActions("post-packages", imageTargets) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-packages: %s", err)
		}
	}

	return nil
}

func (c *cmdLXD) run(cmd *cobra.Command, args []string, overlayDir string) error {
	img := image.NewLXDImage(overlayDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	for _, file := range c.global.definition.Files {
		if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, shared.ImageTargetAll|shared.ImageTargetContainer) {
			continue
		}

		generator := generators.Get(file.Generator)
		if generator == nil {
			return fmt.Errorf("Unknown generator '%s'", file.Generator)
		}

		err := generator.RunLXD(c.global.flagCacheDir, overlayDir,
			img, file)
		if err != nil {
			return fmt.Errorf("Failed to create LXD data: %s", err)
		}
	}

	exitChroot, err := shared.SetupChroot(overlayDir,
		c.global.definition.Environment, nil)
	if err != nil {
		return err
	}

	// Run post files hook
	for _, action := range c.global.definition.GetRunnableActions("post-files", shared.ImageTargetAll|shared.ImageTargetContainer) {
		err := shared.RunScript(action.Action)
		if err != nil {
			exitChroot()
			return fmt.Errorf("Failed to run post-files: %s", err)
		}
	}

	exitChroot()

	err = img.Build(c.flagType == "unified", c.flagCompression)
	if err != nil {
		return fmt.Errorf("Failed to create LXD image: %s", err)
	}

	return nil
}
