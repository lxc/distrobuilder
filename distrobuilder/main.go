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
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	lxd "github.com/lxc/lxd/shared"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v2"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
	"github.com/lxc/distrobuilder/shared/version"
	"github.com/lxc/distrobuilder/sources"
)

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
  - lzop
  - xz (default)
  - zstd
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
		Short: "System container image builder for LXC and LXD",
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

	validateCmd := cmdValidate{global: &globalCmd}
	app.AddCommand(validateCmd.command())

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
	err := os.MkdirAll(c.sourceDir, 0755)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", c.sourceDir, err)
	}

	// Get the image definition
	c.definition, err = getDefinition(args[0], c.flagOptions)
	if err != nil {
		return fmt.Errorf("Failed to get definition: %w", err)
	}

	// Create cache directory if we also plan on creating LXC or LXD images
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
	exitChroot, err := shared.SetupChroot(c.sourceDir, c.definition.Environment, nil)
	if err != nil {
		return fmt.Errorf("Failed to setup chroot: %w", err)
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
		err := shared.RunScript(c.ctx, hook.Action)
		if err != nil {
			return fmt.Errorf("Failed to run post-packages: %w", err)
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
	hasLogger := c.logger != nil

	// exit all chroots otherwise we cannot remove the cache directory
	for _, exit := range shared.ActiveChroots {
		if exit != nil {
			exit()
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

		os.RemoveAll(c.flagCacheDir)
	}

	// Clean up sources directory
	if !c.flagKeepSources {
		if hasLogger {
			c.logger.Info("Removing sources directory")
		}

		os.RemoveAll(c.flagSourcesDir)
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
func addSystemdGenerator() {
	// Check if container has systemd
	if !lxd.PathExists("/etc/systemd") {
		return
	}

	content := `#!/bin/sh
# NOTE: systemctl is not available for systemd-generators
set -eu

# disable localisation (faster grep)
export LC_ALL=C

## Helper functions
# is_lxc_container succeeds if we're running inside a LXC container
is_lxc_container() {
	grep -qa container=lxc /proc/1/environ
}

is_lxc_privileged_container() {
	grep -qw 4294967295$ /proc/self/uid_map
}

# is_lxd_vm succeeds if we're running inside a LXD VM
is_lxd_vm() {
	[ -e /dev/virtio-ports/org.linuxcontainers.lxd ]
}

# is_in_path succeeds if the given file exists in on of the paths
is_in_path() {
	# Don't use $PATH as that may not include all relevant paths
	for path in /bin /sbin /usr/bin /usr/sbin /usr/local/bin /usr/local/sbin; do
		[ -e "${path}/$1" ] && return 0
	done

	return 1
}

## Fix functions
# fix_ro_paths avoids udevd issues with /sys and /proc being writable
fix_ro_paths() {
	mkdir -p "/run/systemd/system/$1.d"
	cat <<-EOF > "/run/systemd/system/$1.d/zzz-lxc-ropath.conf"
[Service]
BindReadOnlyPaths=/sys /proc
EOF
}

# fix_nm_force_up sets up a unit override to force NetworkManager to start the system connection
fix_nm_force_up() {
	# Check if the device exists
	[ -e "/sys/class/net/$1" ] || return 0

	cat <<-EOF > /run/systemd/system/network-connection-activate.service
[Unit]
Description=Activate connection
After=NetworkManager.service NetworkManager-wait-online.service

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
	[ -e "/sys/class/net/$1" ] || return 0

	ip_path=
	if [ -f /sbin/ip ]; then
		ip_path=/sbin/ip
	elif [ -f /bin/ip ]; then
		ip_path=/bin/ip
	else
		return 0
	fi

	cat <<-EOF > /run/systemd/system/network-device-down.service
[Unit]
Description=Turn off network device
Before=NetworkManager.service
Before=systemd-networkd.service

[Service]
ExecStart=-${ip_path} link set $1 down
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
	{
		echo "[Service]";
		[ "${systemd_version}" -ge 247 ] && echo "ProcSubset=all";
		[ "${systemd_version}" -ge 247 ] && echo "ProtectProc=default";
		[ "${systemd_version}" -ge 232 ] && echo "ProtectControlGroups=no";
		[ "${systemd_version}" -ge 232 ] && echo "ProtectKernelTunables=no";
		[ "${systemd_version}" -ge 239 ] && echo "NoNewPrivileges=no";
		[ "${systemd_version}" -ge 249 ] && echo "LoadCredential=";

		# Additional settings for privileged containers
		if is_lxc_privileged_container; then
			echo "ProtectHome=no";
			echo "ProtectSystem=no";
			echo "PrivateDevices=no";
			echo "PrivateTmp=no";
			[ "${systemd_version}" -ge 244 ] && echo "ProtectKernelLogs=no";
			[ "${systemd_version}" -ge 232 ] && echo "ProtectKernelModules=no";
			[ "${systemd_version}" -ge 231 ] && echo "ReadWritePaths=";
		fi
	} > "${dropin_dir}/zzz-lxc-service.conf"
}

# fix_systemd_mask masks the systemd unit
fix_systemd_mask() {
	ln -sf /dev/null "/run/systemd/system/$1"
}

# fix_systemd_udev_trigger overrides the systemd-udev-trigger.service to match the latest version
# of the file which uses "ExecStart=-" instead of "ExecStart=".
fix_systemd_udev_trigger() {
	cmd=
	if [ -f /usr/bin/udevadm ]; then
		cmd=/usr/bin/udevadm
	elif [ -f /sbin/udevadm ]; then
		cmd=/sbin/udevadm
	elif [ -f /bin/udevadm ]; then
		cmd=/bin/udevadm
	else
		return 0
	fi

	mkdir -p /run/systemd/system/systemd-udev-trigger.service.d
	cat <<-EOF > /run/systemd/system/systemd-udev-trigger.service.d/zzz-lxc-override.conf
[Service]
ExecStart=
ExecStart=-${cmd} trigger --type=subsystems --action=add
ExecStart=-${cmd} trigger --type=devices --action=add
EOF
}

# fix_systemd_sysctl overrides the systemd-sysctl.service to use "ExecStart=-" instead of "ExecStart=".
fix_systemd_sysctl() {
	cmd=/usr/lib/systemd/systemd-sysctl
	! [ -e "${cmd}" ] && cmd=/lib/systemd/systemd-sysctl
	mkdir -p /run/systemd/system/systemd-sysctl.service.d
	cat <<-EOF > /run/systemd/system/systemd-sysctl.service.d/zzz-lxc-override.conf
[Service]
ExecStart=
ExecStart=-${cmd}
EOF
}

## Main logic
# Nothing to do in LXD VM but deployed in case it is later converted to a container
is_lxd_vm && exit 0

# Exit immediately if not a LXC/LXD container
is_lxc_container || exit 0

# Determine systemd version
for path in /usr/lib/systemd/systemd /lib/systemd/systemd; do
	[ -x "${path}" ] || continue

	systemd_version="$("${path}" --version | head -n1 | cut -d' ' -f2)"
	break
done

# Determine distro name and release
ID=""
if [ -e /etc/os-release ]; then
	. /etc/os-release
fi

# Overriding some systemd features is only needed if security.nesting=false
# in which case, /dev/.lxc will be missing
if [ ! -d /dev/.lxc ]; then
	# Apply systemd overrides
	if [ "${systemd_version}" -ge 244 ]; then
		fix_systemd_override_unit system/service
	else
		# Setup per-unit overrides
		find /lib/systemd /etc/systemd /run/systemd /usr/lib/systemd -name "*.service" -type f | sed 's#/\(lib\|etc\|run\|usr/lib\)/systemd/##g'| while read -r service_file; do
			fix_systemd_override_unit "${service_file}"
		done
	fi

	# Workarounds for privileged containers.
	if { [ "${ID}" = "altlinux" ] || [ "${ID}" = "arch" ] || [ "${ID}" = "fedora" ]; } && ! is_lxc_privileged_container; then
		fix_ro_paths systemd-networkd.service
		fix_ro_paths systemd-resolved.service
	fi
fi

# Ignore failures on some units.
fix_systemd_udev_trigger
fix_systemd_sysctl

# Mask some units.
fix_systemd_mask dev-hugepages.mount
fix_systemd_mask run-ribchester-general.mount
fix_systemd_mask systemd-hwdb-update.service
fix_systemd_mask systemd-journald-audit.socket
fix_systemd_mask systemd-modules-load.service
fix_systemd_mask systemd-pstore.service
fix_systemd_mask ua-messaging.service
if [ ! -e /dev/tty1 ]; then
	fix_systemd_mask vconsole-setup-kludge@tty1.service
fi

# Workarounds for cloud containers
if { [ "${ID}" = "fedora" ] || [ "${ID}" = "rhel" ]; } && is_in_path cloud-init; then
	fix_nm_force_up eth0
fi

# Workarounds for NetworkManager in containers
if is_in_path NetworkManager; then
	if [ "${ID}" = "ol" ] || [ "${ID}" = "centos" ]; then
		fix_nm_force_up eth0
	fi

	fix_nm_link_state eth0
fi
`
	os.MkdirAll("/etc/systemd/system-generators", 0755)
	ioutil.WriteFile("/etc/systemd/system-generators/lxc", []byte(content), 0755)
}
