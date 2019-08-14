package main

import (
	"fmt"

	lxd "github.com/lxc/lxd/shared"
	"github.com/spf13/cobra"

	"github.com/lxc/distrobuilder/generators"
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
				generator := generators.Get(file.Generator)
				if generator == nil {
					return fmt.Errorf("Unknown generator '%s'", file.Generator)
				}

				if len(file.Releases) > 0 && !lxd.StringInSlice(
					c.global.definition.Image.Release, file.Releases) {
					continue
				}

				if len(file.Architectures) > 0 && !lxd.StringInSlice(
					c.global.definition.Image.ArchitectureMapped, file.Architectures) {
					continue
				}

				if len(file.Variants) > 0 && !lxd.StringInSlice(
					c.global.definition.Image.Variant, file.Variants) {
					continue
				}

				err := generator.Run(c.global.flagCacheDir, c.global.targetDir, file)
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
