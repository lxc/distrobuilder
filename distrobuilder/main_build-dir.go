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

	flagWithPostFiles bool
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

			if !c.flagWithPostFiles {
				return nil
			}

			exitChroot, err := shared.SetupChroot(c.global.targetDir,
				c.global.definition.Environment, nil)
			if err != nil {
				return fmt.Errorf("Failed to setup chroot in %q: %w", c.global.targetDir, err)
			}

			c.global.logger.WithField("trigger", "post-files").Info("Running hooks")

			// Run post files hook
			for _, action := range c.global.definition.GetRunnableActions("post-files", shared.ImageTargetUndefined) {
				if action.Pongo {
					action.Action, err = shared.RenderTemplate(action.Action, c.global.definition)
					if err != nil {
						return fmt.Errorf("Failed to render action: %w", err)
					}
				}

				err := shared.RunScript(c.global.ctx, action.Action)
				if err != nil {
					exitChroot()
					return fmt.Errorf("Failed to run post-files: %w", err)
				}
			}

			exitChroot()

			return nil
		},
	}

	c.cmdBuild.Flags().StringVar(&c.global.flagSourcesDir, "sources-dir", filepath.Join(os.TempDir(), "distrobuilder"), "Sources directory for distribution tarballs"+"``")
	c.cmdBuild.Flags().BoolVar(&c.global.flagKeepSources, "keep-sources", true, "Keep sources after build"+"``")
	c.cmdBuild.Flags().BoolVar(&c.flagWithPostFiles, "with-post-files", false, "Run post-files actions"+"``")
	return c.cmdBuild
}
