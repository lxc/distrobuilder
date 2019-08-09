package main

import (
	"errors"
	"fmt"

	lxd "github.com/lxc/lxd/shared"
	"github.com/spf13/cobra"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/image"
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
		RunE: c.run,
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
		RunE: c.run,
	}

	c.cmdPack.Flags().StringVar(&c.flagType, "type", "split", "Type of tarball to create")
	c.cmdPack.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use")

	return c.cmdPack
}

func (c *cmdLXD) run(cmd *cobra.Command, args []string) error {
	img := image.NewLXDImage(c.global.sourceDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	for _, file := range c.global.definition.Files {
		if len(file.Releases) > 0 && !lxd.StringInSlice(c.global.definition.Image.Release,
			file.Releases) {
			continue
		}

		if len(file.Variants) > 0 && !lxd.StringInSlice(c.global.definition.Image.Variant, file.Variants) {
			continue
		}

		generator := generators.Get(file.Generator)
		if generator == nil {
			return fmt.Errorf("Unknown generator '%s'", file.Generator)
		}

		err := generator.RunLXD(c.global.flagCacheDir, c.global.sourceDir,
			img, file)
		if err != nil {
			return fmt.Errorf("Failed to create LXD data: %s", err)
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

	err = img.Build(c.flagType == "unified", c.flagCompression)
	if err != nil {
		return fmt.Errorf("Failed to create LXD image: %s", err)
	}

	return nil
}
