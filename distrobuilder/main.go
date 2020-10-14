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
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/shared/version"
	"github.com/lxc/distrobuilder/sources"
)

type cmdGlobal struct {
	flagCleanup  bool
	flagCacheDir string
	flagOptions  []string
	flagTimeout  uint
	flagVersion  bool

	definition *shared.Definition
	sourceDir  string
	targetDir  string
	interrupt  chan os.Signal
}

func main() {
	// Global flags
	globalCmd := cmdGlobal{}

	app := &cobra.Command{
		Use:   "distrobuilder",
		Short: "System container image builder for LXC and LXD",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			// Sanity checks
			if os.Geteuid() != 0 {
				fmt.Fprintf(os.Stderr, "You must be root to run this tool\n")
				os.Exit(1)
			}

			// Create temp directory if the cache directory isn't explicitly set
			if globalCmd.flagCacheDir == "" {
				dir, err := ioutil.TempDir("/var/cache", "distrobuilder.")
				if err != nil {
					fmt.Fprintf(os.Stderr, "Failed to create cache directory: %s\n", err)
					os.Exit(1)
				}

				globalCmd.flagCacheDir = dir
			}
		},
		PersistentPostRunE: globalCmd.postRun,
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

	// Version handling
	app.SetVersionTemplate("{{.Version}}\n")
	app.Version = version.Version

	// LXC sub-commands
	LXCCmd := cmdLXC{global: &globalCmd}
	app.AddCommand(LXCCmd.commandBuild())
	app.AddCommand(LXCCmd.commandPack())

	// LXD sub-commands
	LXDCmd := cmdLXD{global: &globalCmd}
	app.AddCommand(LXDCmd.commandBuild())
	app.AddCommand(LXDCmd.commandPack())

	// build-dir sub-command
	buildDirCmd := cmdBuildDir{global: &globalCmd}
	app.AddCommand(buildDirCmd.command())

	// Timeout handler
	go func() {
		// No timeout set
		if globalCmd.flagTimeout == 0 {
			return
		}

		time.Sleep(time.Duration(globalCmd.flagTimeout) * time.Second)
		fmt.Println("Timed out")
		os.Exit(1)
	}()

	go func() {
		<-globalCmd.interrupt

		// exit all chroots otherwise we cannot remove the cache directory
		for _, exit := range shared.ActiveChroots {
			if exit != nil {
				exit()
			}
		}

		globalCmd.postRun(nil, nil)
		fmt.Println("Interrupted")
		os.Exit(1)
	}()

	globalCmd.interrupt = make(chan os.Signal, 1)
	signal.Notify(globalCmd.interrupt, os.Interrupt)

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		globalCmd.postRun(nil, nil)
		os.Exit(1)
	}
}

func (c *cmdGlobal) preRunBuild(cmd *cobra.Command, args []string) error {
	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	isRunningBuildDir := cmd.CalledAs() == "build-dir"

	// Clean up cache directory before doing anything
	os.RemoveAll(c.flagCacheDir)
	os.Mkdir(c.flagCacheDir, 0755)

	if len(args) > 1 {
		// Create and set target directory if provided
		err := os.MkdirAll(args[1], 0755)
		if err != nil {
			return err
		}
		c.targetDir = args[1]
	} else {
		// Use current working directory as target
		var err error
		c.targetDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}
	if isRunningBuildDir {
		c.sourceDir = c.targetDir
	} else {
		c.sourceDir = filepath.Join(c.flagCacheDir, "rootfs")
	}

	// Create source directory if it doesn't exist
	err := os.MkdirAll(c.sourceDir, 0755)
	if err != nil {
		return err
	}

	// Get the image definition
	c.definition, err = getDefinition(args[0], c.flagOptions)
	if err != nil {
		return err
	}

	// Sanity checks for Windows VMs.
	if c.definition.Source.Downloader == "windows" {
		subcommand := cmd.CalledAs()

		if strings.Contains(subcommand, "lxc") {
			return fmt.Errorf("Windows is not supported by LXC")
		}

		if subcommand == "build-lxd" || subcommand == "pack-lxd" {
			ok, err := cmd.Flags().GetBool("vm")
			if err != nil {
				return err
			}

			if !ok {
				return fmt.Errorf("LXD only supports Windows VMs")
			}

			c.definition.Targets.Type = "vm"
		}
	}

	// Create cache directory if we also plan on creating LXC or LXD images
	if !isRunningBuildDir {
		err = os.MkdirAll(c.flagCacheDir, 0755)
		if err != nil {
			return err
		}
	}

	// Get the downloader to use for this image
	downloader := sources.Get(c.definition.Source.Downloader)
	if downloader == nil {
		return fmt.Errorf("Unsupported source downloader: %s", c.definition.Source.Downloader)
	}

	// Run template on source keys
	for i, key := range c.definition.Source.Keys {
		c.definition.Source.Keys[i], err = shared.RenderTemplate(key, c.definition)
		if err != nil {
			return errors.Wrap(err, "Failed to render source keys")
		}
	}

	// Download the root filesystem
	err = downloader.Run(*c.definition, c.sourceDir)
	if err != nil {
		return errors.Wrap(err, "Error while downloading source")
	}

	// Return here as we cannot use chroot for Windows.
	if c.definition.Source.Downloader == "windows" {
		return nil
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(c.sourceDir, c.definition.Environment, nil)
	if err != nil {
		return errors.Wrap(err, "Failed to setup chroot")
	}
	// Unmount everything and exit the chroot
	defer exitChroot()

	// Always include sections which have no type filter. If running build-dir,
	// only these sections will be processed.
	imageTargets := shared.ImageTargetUndefined

	// If we're running either build-lxc or build-lxd, include types which are
	// meant for all.
	if !isRunningBuildDir {
		imageTargets |= shared.ImageTargetAll
	}

	switch cmd.CalledAs() {
	case "build-lxc":
		// If we're running build-lxc, also process container-only sections.
		imageTargets |= shared.ImageTargetContainer
	case "build-lxd":
		// Include either container-specific or vm-specific sections when
		// running build-lxd.
		ok, err := cmd.Flags().GetBool("vm")
		if err != nil {
			return err
		}

		if ok {
			imageTargets |= shared.ImageTargetVM
			c.definition.Targets.Type = "vm"
		} else {
			imageTargets |= shared.ImageTargetContainer
		}
	}

	var manager *managers.Manager

	if c.definition.Packages.Manager != "" {
		manager = managers.Get(c.definition.Packages.Manager)
		if manager == nil {
			return fmt.Errorf("Couldn't get manager")
		}
	} else {
		manager = managers.GetCustom(*c.definition.Packages.CustomManager)
	}

	err = manageRepositories(c.definition, manager, imageTargets)
	if err != nil {
		return errors.Wrap(err, "Failed to manage repositories")
	}

	// Run post unpack hook
	for _, hook := range c.definition.GetRunnableActions("post-unpack", imageTargets) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return errors.Wrap(err, "Failed to run post-unpack")
		}
	}

	// Install/remove/update packages
	err = managePackages(c.definition, manager, imageTargets)
	if err != nil {
		return errors.Wrap(err, "Failed to manage packages")
	}

	// Run post packages hook
	for _, hook := range c.definition.GetRunnableActions("post-packages", imageTargets) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return errors.Wrap(err, "Failed to run post-packages")
		}
	}

	return nil
}

func (c *cmdGlobal) preRunPack(cmd *cobra.Command, args []string) error {
	var err error

	// if an error is returned, disable the usage message
	cmd.SilenceUsage = true

	// Clean up cache directory before doing anything
	os.RemoveAll(c.flagCacheDir)
	os.Mkdir(c.flagCacheDir, 0755)

	// resolve path
	c.sourceDir, err = filepath.Abs(args[1])
	if err != nil {
		return err
	}

	c.targetDir = "."
	if len(args) == 3 {
		c.targetDir = args[2]
	}

	// Get the image definition
	c.definition, err = getDefinition(args[0], c.flagOptions)
	if err != nil {
		return err
	}

	return nil
}

func (c *cmdGlobal) postRun(cmd *cobra.Command, args []string) error {
	// Clean up cache directory
	if c.flagCleanup {
		return os.RemoveAll(c.flagCacheDir)
	}

	return nil
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
	err := yaml.Unmarshal(buf.Bytes(), &def)
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
			return nil, errors.Wrapf(err, "Failed to set option %s", o)
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
