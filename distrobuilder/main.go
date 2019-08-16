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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/sources"
)

type cmdGlobal struct {
	flagCleanup  bool
	flagCacheDir string
	flagOptions  []string
	flagTimeout  uint

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

	if cmd.CalledAs() == "build-dir" {
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

	// Create cache directory if we also plan on creating LXC or LXD images
	if cmd.CalledAs() != "build-dir" {
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

	// Download the root filesystem
	err = downloader.Run(*c.definition, c.sourceDir)
	if err != nil {
		return fmt.Errorf("Error while downloading source: %s", err)
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := shared.SetupChroot(c.sourceDir, c.definition.Environment)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}
	// Unmount everything and exit the chroot
	defer exitChroot()

	// Run post unpack hook
	for _, hook := range c.definition.GetRunnableActions("post-unpack") {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-unpack: %s", err)
		}
	}

	// Install/remove/update packages
	err = managePackages(c.definition.Packages,
		c.definition.GetRunnableActions("post-update"), c.definition.Image.Release,
		c.definition.Image.ArchitectureMapped, c.definition.Image.Variant)
	if err != nil {
		return fmt.Errorf("Failed to manage packages: %s", err)
	}

	// Run post packages hook
	for _, hook := range c.definition.GetRunnableActions("post-packages") {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-packages: %s", err)
		}
	}

	return nil
}

func (c *cmdGlobal) preRunPack(cmd *cobra.Command, args []string) error {
	var err error

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
			return nil, fmt.Errorf("Failed to set option %s: %s", o, err)
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
