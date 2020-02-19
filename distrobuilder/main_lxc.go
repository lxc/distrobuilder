package main

import (
	"fmt"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

type cmdLXC struct {
	cmdBuild *cobra.Command
	cmdPack  *cobra.Command
	global   *cmdGlobal
}

func (c *cmdLXC) commandBuild() *cobra.Command {
	c.cmdBuild = &cobra.Command{
		Use:     "build-lxc <filename|-> [target dir]",
		Short:   "Build LXC image from scratch",
		Args:    cobra.RangeArgs(1, 2),
		PreRunE: c.global.preRunBuild,
		RunE: func(cmd *cobra.Command, args []string) error {
			cleanup, overlayDir, err := getOverlay(c.global.flagCacheDir, c.global.sourceDir)
			if err != nil {
				return errors.Wrap(err, "Failed to create overlay")
			}
			defer cleanup()

			return c.run(cmd, args, overlayDir)
		},
	}
	return c.cmdBuild
}

func (c *cmdLXC) commandPack() *cobra.Command {
	c.cmdPack = &cobra.Command{
		Use:     "pack-lxc <filename|-> <source dir> [target dir]",
		Short:   "Create LXC image from existing rootfs",
		Args:    cobra.RangeArgs(2, 3),
		PreRunE: c.global.preRunPack,
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
	return c.cmdPack
}

func (c *cmdLXC) runPack(cmd *cobra.Command, args []string, overlayDir string) error {
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

func (c *cmdLXC) run(cmd *cobra.Command, args []string, overlayDir string) error {
	img := image.NewLXCImage(overlayDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	for _, file := range c.global.definition.Files {
		generator := generators.Get(file.Generator)
		if generator == nil {
			return fmt.Errorf("Unknown generator '%s'", file.Generator)
		}

		if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, shared.ImageTargetAll|shared.ImageTargetContainer) {
			continue
		}

		err := generator.RunLXC(c.global.flagCacheDir, overlayDir, img,
			c.global.definition.Targets.LXC, file)
		if err != nil {
			return err
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
			return errors.Wrap(err, "Failed to run post-files")
		}
	}

	exitChroot()

	err = img.Build()
	if err != nil {
		return errors.Wrap(err, "Failed to create LXC image")
	}

	// Clean up the chroot by restoring the orginal files.
	err = generators.RestoreFiles(c.global.flagCacheDir, overlayDir)
	if err != nil {
		return errors.Wrap(err, "Failed to restore cached files")
	}

	return nil
}
