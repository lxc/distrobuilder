package main

import (
	"fmt"
	"os"
	"path/filepath"

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

	flagCompression string
}

func (c *cmdLXC) commandBuild() *cobra.Command {
	c.cmdBuild = &cobra.Command{
		Use:   "build-lxc <filename|-> [target dir] [--compression=COMPRESSION]",
		Short: "Build LXC image from scratch",
		Long: fmt.Sprintf(`Build LXC image from scratch

%s
`, compressionDescription),
		Args: cobra.RangeArgs(1, 2),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Check compression arguments
			_, _, err := shared.ParseCompression(c.flagCompression)
			if err != nil {
				return fmt.Errorf("Failed to parse compression level: %w", err)
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

	c.cmdBuild.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use"+"``")
	c.cmdBuild.Flags().StringVar(&c.global.flagSourcesDir, "sources-dir", filepath.Join(os.TempDir(), "distrobuilder"), "Sources directory for distribution tarballs"+"``")
	c.cmdBuild.Flags().BoolVar(&c.global.flagKeepSources, "keep-sources", true, "Keep sources after build"+"``")

	return c.cmdBuild
}

func (c *cmdLXC) commandPack() *cobra.Command {
	c.cmdPack = &cobra.Command{
		Use:   "pack-lxc <filename|-> <source dir> [target dir] [--compression=COMPRESSION]",
		Short: "Create LXC image from existing rootfs",
		Long: fmt.Sprintf(`Create LXC image from existing rootfs

%s
`, compressionDescription),
		Args: cobra.RangeArgs(2, 3),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Check compression arguments
			_, _, err := shared.ParseCompression(c.flagCompression)
			if err != nil {
				return fmt.Errorf("Failed to parse compression level: %w", err)
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

			err = c.runPack(cmd, args, overlayDir)
			if err != nil {
				return fmt.Errorf("Failed to pack image: %w", err)
			}

			return c.run(cmd, args, overlayDir)
		},
	}

	c.cmdPack.Flags().StringVar(&c.flagCompression, "compression", "xz", "Type of compression to use"+"``")

	return c.cmdPack
}

func (c *cmdLXC) runPack(cmd *cobra.Command, args []string, overlayDir string) error {
	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(overlayDir, *c.global.definition, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %w", err)
	}
	// Unmount everything and exit the chroot
	defer func() {
		_ = exitChroot()
	}()

	imageTargets := shared.ImageTargetAll | shared.ImageTargetContainer

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

func (c *cmdLXC) run(cmd *cobra.Command, args []string, overlayDir string) error {
	img := image.NewLXCImage(c.global.ctx, overlayDir, c.global.targetDir,
		c.global.flagCacheDir, *c.global.definition)

	for _, file := range c.global.definition.Files {
		if !shared.ApplyFilter(&file, c.global.definition.Image.Release, c.global.definition.Image.ArchitectureMapped, c.global.definition.Image.Variant, c.global.definition.Targets.Type, shared.ImageTargetUndefined|shared.ImageTargetAll|shared.ImageTargetContainer) {
			c.global.logger.WithField("generator", file.Generator).Info("Skipping generator")

			continue
		}

		generator, err := generators.Load(file.Generator, c.global.logger, c.global.flagCacheDir, overlayDir, file, *c.global.definition)
		if err != nil {
			return fmt.Errorf("Failed to load generator %q: %w", file.Generator, err)
		}

		c.global.logger.WithField("generator", file.Generator).Info("Running generator")

		err = generator.RunLXC(img, c.global.definition.Targets.LXC)
		if err != nil {
			return fmt.Errorf("Failed to run generator %q: %w", file.Generator, err)
		}
	}

	exitChroot, err := shared.SetupChroot(overlayDir,
		*c.global.definition, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot in %q: %w", overlayDir, err)
	}

	err = addSystemdGenerator()
	if err != nil {
		return fmt.Errorf("Failed adding systemd generator: %w", err)
	}

	c.global.logger.WithField("trigger", "post-files").Info("Running hooks")

	// Run post files hook
	for _, action := range c.global.definition.GetRunnableActions("post-files", shared.ImageTargetUndefined|shared.ImageTargetAll|shared.ImageTargetContainer) {
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

	c.global.logger.WithField("compression", c.flagCompression).Info("Creating LXC image")

	err = img.Build(c.flagCompression)
	if err != nil {
		return fmt.Errorf("Failed to create LXC image: %w", err)
	}

	return nil
}
