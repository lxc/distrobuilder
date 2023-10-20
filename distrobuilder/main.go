package main

/*
#define _GNU_SOURCE
#include <errno.h>
#include <sched.h>
#include <stdio.h>
#include <string.h>
#include <sys/mount.h>
#include <sys/types.h>
#include <sys/wait.h>
#include <unistd.h>

__attribute__((constructor)) void init(void) {
	pid_t pid;
	int ret;

	if (geteuid() != 0) {
		return;
	}

	// Unshare a new mntns so our mounts don't leak
	if (unshare(CLONE_NEWNS | CLONE_NEWPID | CLONE_NEWUTS) < 0) {
		fprintf(stderr, "Failed to unshare namespaces: %s\n", strerror(errno));
		_exit(1);
	}

	// Hardcode the hostname to "distrobuilder"
	if (sethostname("distrobuilder", 13) < 0) {
		fprintf(stderr, "Failed to set hostname: %s\n", strerror(errno));
		_exit(1);
	}

	// Prevent mount propagation back to initial namespace
	if (mount(NULL, "/", NULL, MS_REC | MS_PRIVATE, NULL) < 0) {
		fprintf(stderr, "Failed to mark / private: %s\n", strerror(errno));
		_exit(1);
	}

	pid = fork();
	if (pid < 0) {
		fprintf(stderr, "Failed to fork: %s\n", strerror(errno));
		_exit(1);
	} else if (pid > 0) {
		// parent
		waitpid(pid, &ret, 0);
		_exit(WEXITSTATUS(ret));
	}

	// We're done, jump back to Go
}
*/
import "C"

import (
	"bufio"
	"bytes"
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	incus "github.com/lxc/incus/shared/util"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/shared/version"
	"github.com/lxc/distrobuilder/sources"
)

//go:embed lxc.generator
var lxcGenerator embed.FS

var typeDescription = `Depending on the type, it either outputs a unified (single tarball)
or split image (tarball + squashfs or qcow2 image). The --type flag can take one of the
following values:
  - split (default)
  - unified
`

var compressionDescription = `The compression can be set with the --compression flag. I can take one of the
following values:
  - bzip2
  - gzip
  - lzip
  - lzma
  - lzo
  - lzop
  - xz (default)
  - zstd

For supported compression methods, a compression level can be specified with
method-N, where N is an integer, e.g. gzip-9.
`

type cmdGlobal struct {
	flagCleanup        bool
	flagCacheDir       string
	flagDebug          bool
	flagOptions        []string
	flagTimeout        uint
	flagVersion        bool
	flagDisableOverlay bool
	flagSourcesDir     string
	flagKeepSources    bool

	definition     *shared.Definition
	sourceDir      string
	targetDir      string
	interrupt      chan os.Signal
	logger         *logrus.Logger
	overlayCleanup func()
	ctx            context.Context
	cancel         context.CancelFunc
}

func main() {
	// Global flags
	globalCmd := cmdGlobal{}

	app := &cobra.Command{
		Use:   "distrobuilder",
		Short: "System container and VM image builder for LXC and Incus",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Quick checks
			if os.Geteuid() != 0 {
				fmt.Fprintf(os.Stderr, "You must be root to run this tool\n")
				os.Exit(1)
			}

			var err error

			globalCmd.logger, err = shared.GetLogger(globalCmd.flagDebug)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to get logger: %s\n", err)
				os.Exit(1)
			}

			if globalCmd.flagTimeout == 0 {
				globalCmd.ctx, globalCmd.cancel = context.WithCancel(context.Background())
			} else {
				globalCmd.ctx, globalCmd.cancel = context.WithTimeout(context.Background(), time.Duration(globalCmd.flagTimeout)*time.Second)
			}

			go func() {
				for {
					select {
					case <-globalCmd.interrupt:
						globalCmd.cancel()
						globalCmd.logger.Info("Interrupted")
						return
					case <-globalCmd.ctx.Done():
						if globalCmd.flagTimeout > 0 {
							globalCmd.logger.Info("Timed out")
						}

						return
					}
				}
			}()

			// No need to create cache directory if we're only validating.
			if cmd.CalledAs() == "validate" {
				return
			}

			// Create temp directory if the cache directory isn't explicitly set
			if globalCmd.flagCacheDir == "" {
				dir, err := os.MkdirTemp("/var/cache", "distrobuilder.")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create cache directory: %s\n", err)
					os.Exit(1)
				}

				globalCmd.flagCacheDir = dir
			}
		},
		PersistentPostRunE: globalCmd.postRun,
		CompletionOptions:  cobra.CompletionOptions{DisableDefaultCmd: true},
	}

	app.PersistentFlags().BoolVar(&globalCmd.flagCleanup, "cleanup", true,
		"Clean up cache directory")
	app.PersistentFlags().StringVar(&globalCmd.flagCacheDir, "cache-dir",
		"", "Cache directory"+"``")
	app.PersistentFlags().StringSliceVarP(&globalCmd.flagOptions, "options", "o",
		[]string{}, "Override options (list of key=value)"+"``")
	app.PersistentFlags().UintVarP(&globalCmd.flagTimeout, "timeout", "t", 0,
		"Timeout in seconds"+"``")
	app.PersistentFlags().BoolVar(&globalCmd.flagVersion, "version", false, "Print version number")
	app.PersistentFlags().BoolVar(&globalCmd.flagDebug, "debug", false, "Enable debug output")
	app.PersistentFlags().BoolVar(&globalCmd.flagDisableOverlay, "disable-overlay", false, "Disable the use of filesystem overlays")

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = version.Version

	// LXC sub-commands
	LXCCmd := cmdLXC{global: &globalCmd}
	app.AddCommand(LXCCmd.commandBuild())
	app.AddCommand(LXCCmd.commandPack())

	// Incus sub-commands
	IncusCmd := cmdIncus{global: &globalCmd}
	app.AddCommand(IncusCmd.commandBuild())
	app.AddCommand(IncusCmd.commandPack())

	// build-dir sub-command
	buildDirCmd := cmdBuildDir{global: &globalCmd}
	app.AddCommand(buildDirCmd.command())

	// repack-windows sub-command
	repackWindowsCmd := cmdRepackWindows{global: &globalCmd}
	app.AddCommand(repackWindowsCmd.command())

	validateCmd := cmdValidate{global: &globalCmd}
	app.AddCommand(validateCmd.command())

	globalCmd.interrupt = make(chan os.Signal, 1)
	signal.Notify(globalCmd.interrupt, os.Interrupt)

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		if globalCmd.logger != nil {
			globalCmd.logger.WithFields(logrus.Fields{"err": err}).Error("Failed running distrobuilder")
		} else {
			fmt.Fprintf(os.Stderr, "Failed running distrobuilder: %s\n", err.Error())
		}

		_ = globalCmd.postRun(nil, nil)
		os.Exit(1)
	}
}

func (c *cmdGlobal) cleanupCacheDirectory() {
	// Try removing the entire cache directory.
	err := os.RemoveAll(c.flagCacheDir)
	if err == nil {
		return
	}

	// Try removing the content of the cache directory if the directory itself cannot be removed.
	err = filepath.Walk(c.flagCacheDir, func(path string, info os.FileInfo, err error) error {
		if path == c.flagCacheDir {
			return nil
		}

		return os.RemoveAll(path)
	})
	if err != nil {
		c.logger.WithField("err", err).Warn("Failed cleaning up cache directory")
	}
}

func (c *cmdGlobal) preRunBuild(cmd *cobra.Command, args []string) error {
	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	isRunningBuildDir := cmd.CalledAs() == "build-dir"

	// Clean up cache directory before doing anything
	c.cleanupCacheDirectory()

	err := os.MkdirAll(c.flagCacheDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed creating cache directory: %w", err)
	}

	if len(args) > 1 {
		// Create and set target directory if provided
		err := os.MkdirAll(args[1], 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", args[1], err)
		}

		c.targetDir = args[1]
	} else {
		// Use current working directory as target
		var err error
		c.targetDir, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("Failed to get working directory: %w", err)
		}
	}
	if isRunningBuildDir {
		c.sourceDir = c.targetDir
	} else {
		c.sourceDir = filepath.Join(c.flagCacheDir, "rootfs")
	}

	// Create source directory if it doesn't exist
	err = os.MkdirAll(c.sourceDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", c.sourceDir, err)
	}

	// Get the image definition
	c.definition, err = getDefinition(args[0], c.flagOptions)
	if err != nil {
		return fmt.Errorf("Failed to get definition: %w", err)
	}

	// Create cache directory if we also plan on creating LXC or Incus images
	if !isRunningBuildDir {
		err = os.MkdirAll(c.flagCacheDir, 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", c.flagCacheDir, err)
		}
	}

	// Run template on source keys
	for i, key := range c.definition.Source.Keys {
		c.definition.Source.Keys[i], err = shared.RenderTemplate(key, c.definition)
		if err != nil {
			return fmt.Errorf("Failed to render source keys: %w", err)
		}
	}

	// Run template on source URL
	c.definition.Source.URL, err = shared.RenderTemplate(c.definition.Source.URL, c.definition)
	if err != nil {
		return fmt.Errorf("Failed to render source URL: %w", err)
	}

	// Load and run downloader
	downloader, err := sources.Load(c.ctx, c.definition.Source.Downloader, c.logger, *c.definition, c.sourceDir, c.flagCacheDir, c.flagSourcesDir)
	if err != nil {
		return fmt.Errorf("Failed to load downloader %q: %w", c.definition.Source.Downloader, err)
	}

	c.logger.Info("Downloading source")

	err = downloader.Run()
	if err != nil {
		return fmt.Errorf("Error while downloading source: %w", err)
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(c.sourceDir, *c.definition, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %w", err)
	}
	// Unmount everything and exit the chroot
	defer func() {
		_ = exitChroot()
	}()

	// Always include sections which have no type filter. If running build-dir,
	// only these sections will be processed.
	imageTargets := shared.ImageTargetUndefined

	// If we're running either build-lxc or build-incus, include types which are
	// meant for all.
	if !isRunningBuildDir {
		imageTargets |= shared.ImageTargetAll
	}

	switch cmd.CalledAs() {
	case "build-lxc":
		// If we're running build-lxc, also process container-only sections.
		imageTargets |= shared.ImageTargetContainer
	case "build-incus":
		// Include either container-specific or vm-specific sections when
		// running build-incus.
		ok, err := cmd.Flags().GetBool("vm")
		if err != nil {
			return fmt.Errorf(`Failed to get bool value of "vm": %w`, err)
		}

		if ok {
			imageTargets |= shared.ImageTargetVM
			c.definition.Targets.Type = shared.DefinitionFilterTypeVM
		} else {
			imageTargets |= shared.ImageTargetContainer
		}
	}

	manager, err := managers.Load(c.ctx, c.definition.Packages.Manager, c.logger, *c.definition)
	if err != nil {
		return fmt.Errorf("Failed to load manager %q: %w", c.definition.Packages.Manager, err)
	}

	c.logger.Info("Managing repositories")

	err = manager.ManageRepositories(imageTargets)
	if err != nil {
		return fmt.Errorf("Failed to manage repositories: %w", err)
	}

	c.logger.WithField("trigger", "post-unpack").Info("Running hooks")

	// Run post unpack hook
	for _, hook := range c.definition.GetRunnableActions("post-unpack", imageTargets) {
		if hook.Pongo {
			hook.Action, err = shared.RenderTemplate(hook.Action, c.definition)
			if err != nil {
				return fmt.Errorf("Failed to render action: %w", err)
			}
		}

		err := shared.RunScript(c.ctx, hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-unpack: %w", err)
		}
	}

	c.logger.Info("Managing packages")

	// Install/remove/update packages
	err = manager.ManagePackages(imageTargets)
	if err != nil {
		return fmt.Errorf("Failed to manage packages: %w", err)
	}

	c.logger.WithField("trigger", "post-packages").Info("Running hooks")

	// Run post packages hook
	for _, hook := range c.definition.GetRunnableActions("post-packages", imageTargets) {
		if hook.Pongo {
			hook.Action, err = shared.RenderTemplate(hook.Action, c.definition)
			if err != nil {
				return fmt.Errorf("Failed to render action: %w", err)
			}
		}

		err := shared.RunScript(c.ctx, hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-packages: %w", err)
		}
	}

	return nil
}

func (c *cmdGlobal) preRunPack(cmd *cobra.Command, args []string) error {
	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	// Clean up cache directory before doing anything
	c.cleanupCacheDirectory()

	err := os.MkdirAll(c.flagCacheDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed creating cache directory: %w", err)
	}

	// resolve path
	c.sourceDir, err = filepath.Abs(args[1])
	if err != nil {
		return fmt.Errorf("Failed to get absolute path of %q: %w", args[1], err)
	}

	c.targetDir = "."
	if len(args) == 3 {
		c.targetDir = args[2]
	}

	// Get the image definition
	c.definition, err = getDefinition(args[0], c.flagOptions)
	if err != nil {
		return fmt.Errorf("Failed to get definition: %w", err)
	}

	return nil
}

func (c *cmdGlobal) postRun(cmd *cobra.Command, args []string) error {
	// If we're only validating, there's nothing to clean up.
	if cmd != nil && cmd.CalledAs() == "validate" {
		return nil
	}

	hasLogger := c.logger != nil

	// exit all chroots otherwise we cannot remove the cache directory
	for _, exit := range shared.ActiveChroots {
		if exit != nil {
			err := exit()
			if err != nil && hasLogger {
				c.logger.WithField("err", err).Warn("Failed exiting chroot")
			}
		}
	}

	// Clean up overlay
	if c.overlayCleanup != nil {
		if hasLogger {
			c.logger.Info("Cleaning up overlay")
		}

		c.overlayCleanup()
	}

	// Clean up cache directory
	if c.flagCleanup {
		if hasLogger {
			c.logger.Info("Removing cache directory")
		}

		c.cleanupCacheDirectory()
	}

	// Clean up sources directory
	if !c.flagKeepSources {
		if hasLogger {
			c.logger.Info("Removing sources directory")
		}

		_ = os.RemoveAll(c.flagSourcesDir)
	}

	return nil
}

func (c *cmdGlobal) getOverlayDir() (string, func(), error) {
	var (
		cleanup    func()
		overlayDir string
		err        error
	)

	if c.flagDisableOverlay {
		overlayDir = filepath.Join(c.flagCacheDir, "overlay")

		// Use rsync if overlay doesn't work
		err = shared.RsyncLocal(c.ctx, c.sourceDir+"/", overlayDir)
		if err != nil {
			return "", nil, fmt.Errorf("Failed to copy image content: %w", err)
		}
	} else {
		cleanup, overlayDir, err = getOverlay(c.logger, c.flagCacheDir, c.sourceDir)
		if err != nil {
			c.logger.WithField("err", err).Warn("Failed to create overlay")

			overlayDir = filepath.Join(c.flagCacheDir, "overlay")

			// Use rsync if overlay doesn't work
			err = shared.RsyncLocal(c.ctx, c.sourceDir+"/", overlayDir)
			if err != nil {
				return "", nil, fmt.Errorf("Failed to copy image content: %w", err)
			}
		}
	}

	return overlayDir, cleanup, nil
}

func getDefinition(fname string, options []string) (*shared.Definition, error) {
	// Read the provided file, or if none was given, read from stdin
	var buf bytes.Buffer
	if fname == "" || fname == "-" {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			buf.WriteString(scanner.Text())
		}
	} else {
		f, err := os.Open(fname)
		if err != nil {
			return nil, err
		}

		defer f.Close()

		_, err = io.Copy(&buf, f)
		if err != nil {
			return nil, err
		}
	}

	// Parse the yaml input
	var def shared.Definition
	err := yaml.UnmarshalStrict(buf.Bytes(), &def)
	if err != nil {
		return nil, err
	}

	// Set options from the command line
	for _, o := range options {
		parts := strings.Split(o, "=")
		if len(parts) != 2 {
			return nil, errors.New("Options need to be of type key=value")
		}

		err := def.SetValue(parts[0], parts[1])
		if err != nil {
			return nil, fmt.Errorf("Failed to set option %s: %w", o, err)
		}
	}

	// Apply some defaults on top of the provided configuration
	def.SetDefaults()

	// Validate the result
	err = def.Validate()
	if err != nil {
		return nil, err
	}

	return &def, nil
}

// addSystemdGenerator creates a systemd-generator which runs on boot, and does some configuration around the system itself and networking.
func addSystemdGenerator() error {
	// Check if container has systemd
	if !incus.PathExists("/etc/systemd") {
		return nil
	}

	err := os.MkdirAll("/etc/systemd/system-generators", 0755)
	if err != nil {
		return fmt.Errorf("Failed creating directory: %w", err)
	}

	content, err := lxcGenerator.ReadFile("lxc.generator")
	if err != nil {
		return fmt.Errorf("Failed reading lxc.generator: %w", err)
	}

	err = os.WriteFile("/etc/systemd/system-generators/lxc", content, 0755)
	if err != nil {
		return fmt.Errorf("Failed creating system generator: %w", err)
	}

	return nil
}
