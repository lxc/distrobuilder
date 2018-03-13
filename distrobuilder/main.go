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
	"os"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/sources"
)

type cmdGlobal struct {
	flagCleanup  bool
	flagCacheDir string

	definition *shared.Definition
	sourceDir  string
	targetDir  string
	rootfsDir  string
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
				fmt.Fprintln(os.Stderr, "You must be root to run this tool")
				os.Exit(1)
			}
		},
		PersistentPostRunE: globalCmd.postRun,
	}

	app.PersistentFlags().BoolVar(&globalCmd.flagCleanup, "cleanup", true,
		"Clean up cache directory")
	app.PersistentFlags().StringVar(&globalCmd.flagCacheDir, "cache-dir",
		"/var/cache/distrobuilder", "Cache directory"+"``")

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

	// Run the main command and handle errors
	err := app.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func (c *cmdGlobal) preRunBuild(cmd *cobra.Command, args []string) error {
	c.sourceDir = c.flagCacheDir

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
		c.rootfsDir = c.targetDir
	} else {
		c.rootfsDir = filepath.Join(c.flagCacheDir, "rootfs")
	}

	var err error

	// Get the image definition
	c.definition, err = getDefinition(args[0])
	if err != nil {
		return err
	}

	// Get the mapped architecture
	arch, err := getMappedArchitecture(c.definition)
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
	err = downloader.Run(c.definition.Source, c.definition.Image.Release, arch, c.rootfsDir)
	if err != nil {
		return fmt.Errorf("Error while downloading source: %s", err)
	}

	// Setup the mounts and chroot into the rootfs
	exitChroot, err := setupChroot(c.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %s", err)
	}

	// Run post unpack hook
	for _, hook := range getRunnableActions("post-unpack", c.definition) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-unpack: %s", err)
		}
	}

	// Install/remove/update packages
	err = managePackages(c.definition.Packages,
		getRunnableActions("post-update", c.definition))
	if err != nil {
		exitChroot()
		return fmt.Errorf("Failed to manage packages: %s", err)
	}

	// Run post packages hook
	for _, hook := range getRunnableActions("post-packages", c.definition) {
		err := shared.RunScript(hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-packages: %s", err)
		}
	}

	// Unmount everything and exit the chroot
	exitChroot()

	return nil
}

func (c *cmdGlobal) preRunPack(cmd *cobra.Command, args []string) error {
	var err error

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
	c.definition, err = getDefinition(args[0])
	if err != nil {
		return err
	}

	return nil
}

func (c *cmdGlobal) postRun(cmd *cobra.Command, args []string) error {
	// Clean up cache directory if needed. Do not clean up if the build-dir
	// sub-command is run since the directory is needed for further actions.
	if c.flagCleanup && cmd.CalledAs() != "build-dir" {
		return os.RemoveAll(c.flagCacheDir)
	}

	return nil
}

func getDefinition(fname string) (*shared.Definition, error) {
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

	// Apply some defaults on top of the provided configuration
	shared.SetDefinitionDefaults(&def)

	// Validate the result
	err = shared.ValidateDefinition(def)
	if err != nil {
		return nil, err
	}

	return &def, nil
}

func getMappedArchitecture(def *shared.Definition) (string, error) {
	var arch string

	if def.Mappings.ArchitectureMap != "" {
		// Translate the architecture using the requested map
		var err error
		arch, err = shared.GetArch(def.Mappings.ArchitectureMap, def.Image.Arch)
		if err != nil {
			return "", fmt.Errorf("Failed to translate the architecture name: %s", err)
		}
	} else if len(def.Mappings.Architectures) > 0 {
		// Translate the architecture using a user specified mapping
		var ok bool
		arch, ok = def.Mappings.Architectures[def.Image.Arch]
		if !ok {
			// If no mapping exists, it means it doesn't need translating
			arch = def.Image.Arch
		}
	} else {
		// No map or mappings provided, just go with it as it is
		arch = def.Image.Arch
	}

	return arch, nil
}

func getRunnableActions(trigger string, definition *shared.Definition) []shared.DefinitionAction {
	out := []shared.DefinitionAction{}

	for _, action := range definition.Actions {
		if action.Trigger != trigger {
			continue
		}

		if len(action.Releases) > 0 && !lxd.StringInSlice(definition.Image.Release, action.Releases) {
			continue
		}

		out = append(out, action)
	}

	return out
}
