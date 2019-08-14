package main

import (
	"fmt"

	lxd "github.com/lxc/lxd/shared"
	"github.com/spf13/cobra"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/image"
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
		RunE:    c.run,
	}
	return c.cmdBuild
}

func (c *cmdLXC) commandPack() *cobra.Command {
	c.cmdPack = &cobra.Command{
		Use:     "pack-lxc <filename|-> <source dir> [target dir]",
		Short:   "Create LXC image from existing rootfs",
		Args:    cobra.RangeArgs(2, 3),
		PreRunE: c.global.preRunPack,
		RunE:    c.run,
	}
	return c.cmdPack
}

func (c *cmdLXC) run(cmd *cobra.Command, args []string) error {
	img := image.NewLXCImage(c.global.sourceDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	for _, file := range c.global.definition.Files {
		generator := generators.Get(file.Generator)
		if generator == nil {
			return fmt.Errorf("Unknown generator '%s'", file.Generator)
		}

		if len(file.Releases) > 0 && !lxd.StringInSlice(
			c.global.definition.Image.Release, file.Releases) {
			continue
		}

		if len(file.Variants) > 0 && !lxd.StringInSlice(
			c.global.definition.Image.Variant, file.Variants) {
			continue
		}

		err := generator.RunLXC(c.global.flagCacheDir, c.global.sourceDir, img,
			file)
		if err != nil {
			return err
		}
	}

	exitChroot, err := shared.SetupChroot(c.global.sourceDir,
		c.global.definition.Environment)
	if err != nil {
		return err
	}

	// Run post files hook
	for _, action := range c.global.definition.GetRunnableActions("post-files") {
		err := shared.RunScript(action.Action)
		if err != nil {
			exitChroot()
			return fmt.Errorf("Failed to run post-files: %s", err)
		}
	}

	exitChroot()

	err = img.Build()
	if err != nil {
		return fmt.Errorf("Failed to create LXC image: %s", err)
	}

	// Clean up the chroot by restoring the orginal files.
	err = generators.RestoreFiles(c.global.flagCacheDir, c.global.sourceDir)
	if err != nil {
		return fmt.Errorf("Failed to restore cached files: %s", err)
	}

	return nil
}
