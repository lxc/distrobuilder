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

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/shared/version"
	"github.com/lxc/distrobuilder/sources"
)

type cmdGlobal struct {
	flagCleanup        bool
	flagCacheDir       string
	flagDebug          bool
	flagOptions        []string
	flagTimeout        uint
	flagVersion        bool
	flagDisableOverlay bool

	definition *shared.Definition
	sourceDir  string
	targetDir  string
	interrupt  chan os.Signal
	logger     *zap.SugaredLogger
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

			var err error

			globalCmd.logger, err = shared.GetLogger(globalCmd.flagDebug)
			if err != nil {
				fmt.Println(errors.Wrap(err, "Failed to get logger"))
				os.Exit(1)
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
	app.PersistentFlags().BoolVar(&globalCmd.flagDebug, "debug", false, "Enable debug output")
	app.PersistentFlags().BoolVar(&globalCmd.flagDisableOverlay, "disable-overlay", false, "Disable the use of filesystem overlays")

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

	// repack-windows sub-command
	repackWindowsCmd := cmdRepackWindows{global: &globalCmd}
	app.AddCommand(repackWindowsCmd.command())

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

	// Run template on source URL
	c.definition.Source.URL, err = shared.RenderTemplate(c.definition.Source.URL, c.definition)
	if err != nil {
		return errors.Wrap(err, "Failed to render source URL")
	}

	// Download the root filesystem
	err = downloader.Run(*c.definition, c.sourceDir)
	if err != nil {
		return errors.Wrap(err, "Error while downloading source")
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
	if c.logger != nil {
		defer c.logger.Sync()
	}

	// Clean up cache directory
	if c.flagCleanup {
		return os.RemoveAll(c.flagCacheDir)
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
		err = shared.RunCommand("rsync", "-a", c.sourceDir+"/", overlayDir)
		if err != nil {
			return "", nil, errors.Wrap(err, "Failed to copy image content")
		}
	} else {
		cleanup, overlayDir, err = getOverlay(c.logger, c.flagCacheDir, c.sourceDir)
		if err != nil {
			c.logger.Warnw("Failed to create overlay", "err", err)

			overlayDir = filepath.Join(c.flagCacheDir, "overlay")

			// Use rsync if overlay doesn't work
			err = shared.RunCommand("rsync", "-a", c.sourceDir+"/", overlayDir)
			if err != nil {
				return "", nil, errors.Wrap(err, "Failed to copy image content")
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

// addSystemdGenerator creates a systemd-generator which runs on boot, and does some configuration around the system itself and networking.
func addSystemdGenerator() {
	// Check if container has systemd
	if !lxd.PathExists("/etc/systemd") {
		return
	}

	content := `#!/bin/sh
# NOTE: systemctl is not available for systemd-generators
set -eu

## Helper functions
# is_lxc_container succeeds if we're running inside a LXC container
is_lxc_container() {
	grep -qa container=lxc /proc/1/environ
}

# is_lxd_vm succeeds if we're running inside a LXD VM
is_lxd_vm() {
	[ -e /dev/virtio-ports/org.linuxcontainers.lxd ]
}

## Fix functions
# fix_networkd avoids udevd issues with /sys being writable
fix_networkd() {
	[ "${ID}" = "altlinux" ] || return

	mkdir -p /run/systemd/system/systemd-networkd.service.d
	cat <<-EOF > /run/systemd/system/systemd-networkd.service.d/lxc-ropath.conf
[Service]
BindReadOnlyPaths=/sys
EOF
}

# fix_nm_force_up sets up a unit override to force NetworkManager to start the system connection
fix_nm_force_up() {
	# Check if the device exists
	[ -e "/sys/class/net/$1" ] || return

	# Check if NetworkManager exists
	which NetworkManager >/dev/null || return

	cat <<-EOF > /run/systemd/system/network-connection-activate.service
[Unit]
Description=Activate connection
After=NetworkManager-wait-online.service

[Service]
ExecStart=-/usr/bin/nmcli c up "System $1"
Type=oneshot
RemainAfterExit=true

[Install]
WantedBy=default.target
EOF

	mkdir -p /run/systemd/system/default.target.wants
	ln -sf /run/systemd/system/network-connection-activate.service /run/systemd/system/default.target.wants/network-connection-activate.service
}

# fix_nm_link_state forces the network interface to a DOWN state ahead of NetworkManager starting up
fix_nm_link_state() {
	[ -e "/sys/class/net/$1" ] || return

	cat <<-EOF > /run/systemd/system/network-device-down.service
[Unit]
Description=Turn off network device
Before=NetworkManager.service

[Service]
ExecStart=-/bin/ip link set $1 down
Type=oneshot
RemainAfterExit=true

[Install]
WantedBy=default.target
EOF

	mkdir -p /run/systemd/system/default.target.wants
	ln -sf /run/systemd/system/network-device-down.service /run/systemd/system/default.target.wants/network-device-down.service
}

# fix_systemd_override_unit generates a unit specific override
fix_systemd_override_unit() {
	dropin_dir="/run/systemd/${1}.d"
	mkdir -p "${dropin_dir}"
	echo "[Service]" > "${dropin_dir}/lxc-service.conf"
	[ "${systemd_version}" -ge 247 ] && echo "ProtectProc=default" >> "${dropin_dir}/lxc-service.conf"
	[ "${systemd_version}" -ge 232 ] && echo "ProtectControlGroups=no" >> "${dropin_dir}/lxc-service.conf"
	[ "${systemd_version}" -ge 232 ] && echo "ProtectKernelTunables=no" >> "${dropin_dir}/lxc-service.conf"
}

# fix_systemd_mask_audit masks the systemd-journal-audit socket
fix_systemd_mask_audit() {
	ln -sf /dev/null /run/systemd/system/systemd-journal-audit.socket
}

## Main logic
# Exit immediately if not a LXC/LXD container or VM
if ! is_lxd_vm && ! is_lxc_container; then
	exit
fi

# Determine systemd version
for path in /usr/lib/systemd/systemd /lib/systemd/systemd; do
	[ -x "${path}" ] || continue

	systemd_version="$("${path}" --version | head -n1 | cut -d' ' -f2)"
	break
done

# Determine distro name and release
ID=""
VERSION_ID=""
if [ -e /etc/os-release ]; then
	. /etc/os-release
fi

# Apply systemd overrides
if [ "${systemd_version}" -ge 244 ]; then
	fix_systemd_override_unit system/service.d
else
	# Setup per-unit overrides
	find /etc/systemd /run/systemd /usr/lib/systemd -name "*.service" -type f | sed -E 's#/usr/lib/systemd/##;s#/etc/systemd/##g;s#/run/systemd/##g' | while read -r service_file; do
		fix_systemd_override_unit "${service_file}"
	done
fi

# Workarounds for all containers
if is_lxc_container; then
	fix_systemd_audit
	fix_networkd
fi

# Workarounds for fedora/34/cloud containers
if is_lxc_container && [ "${ID}" = "fedora" ] && [ "${VERSION_ID}" = "34" ] && which cloud-init >/dev/null; then
	fix_nm_force_up eth0
fi

# Workarounds for NetworkManager in containers
if which NetworkManager >/dev/null; then
	fix_nm_link_state eth0
fi
`
	os.MkdirAll("/etc/systemd/system-generators", 0755)
	ioutil.WriteFile("/etc/systemd/system-generators/lxc", []byte(content), 0755)
}
