package main

import (
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/shared"
)

type cmdBuildDir struct {
	cmd    *cobra.Command
	global *cmdGlobal
}

func (c *cmdBuildDir) command() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build-dir <filename|-> <target dir>",
		Short: "Build plain rootfs",
		Args:  cobra.ExactArgs(2),
		RunE:  c.global.preRunBuild,
		PostRunE: func(cmd *cobra.Command, args []string) error {
			// Run global generators
			for _, file := range c.global.definition.Files {
				generator, err := generators.Load(file.Generator, c.global.logger, c.global.flagCacheDir, c.global.targetDir, file)
				if err != nil {
					return errors.Wrapf(err, "Failed to load generator %q", file.Generator)
				}

				if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, 0) {
					continue
				}

				err = generator.Run()
				if err != nil {
					continue
				}
			}

			return nil
		},
	}

	c.cmd = cmd
	return cmd
}
