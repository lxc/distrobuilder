package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/lxc/distrobuilder/generators"
	"github.com/lxc/distrobuilder/shared"
)

type cmdBuildDir struct {
	cmdBuild *cobra.Command
	global   *cmdGlobal
}

func (c *cmdBuildDir) command() *cobra.Command {
	c.cmdBuild = &cobra.Command{
		Use:   "build-dir <filename|-> <target dir>",
		Short: "Build plain rootfs",
		Args:  cobra.ExactArgs(2),
		RunE:  c.global.preRunBuild,
		PostRunE: func(cmd *cobra.Command, args []string) error {
			// Run global generators
			for _, file := range c.global.definition.Files {
				if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, 0) {
					continue
				}

				generator, err := generators.Load(file.Generator, c.global.logger, c.global.flagCacheDir, c.global.targetDir, file, *c.global.definition)
				if err != nil {
					return fmt.Errorf("Failed to load generator %q: %w", file.Generator, err)
				}

				c.global.logger.WithField("generator", file.Generator).Info("Running generator")

				err = generator.Run()
				if err != nil {
					continue
				}
			}

			return nil
		},
	}

	c.cmdBuild.Flags().StringVar(&c.global.flagSourcesDir, "sources-dir", filepath.Join(os.TempDir(), "distrobuilder"), "Sources directory for distribution tarballs"+"``")
	c.cmdBuild.Flags().BoolVar(&c.global.flagKeepSources, "keep-sources", true, "Keep sources after build"+"``")

	return c.cmdBuild
}
