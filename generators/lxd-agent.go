package generators

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

type lxdAgent struct {
	common
}

// RunLXC is not supported.
func (g *lxdAgent) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	return ErrNotSupported
}

// RunLXD creates systemd unit files for the lxd-agent.
func (g *lxdAgent) RunLXD(img *image.LXDImage, target shared.DefinitionTargetLXD) error {
	initFile := filepath.Join(g.sourceDir, "sbin", "init")

	fi, err := os.Lstat(initFile)
	if err != nil {
		return fmt.Errorf("Failed to stat file %q: %w", initFile, err)
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(initFile)
		if err != nil {
			return fmt.Errorf("Failed to read link %q: %w", initFile, err)
		}

		if strings.Contains(linkTarget, "systemd") {
			return g.handleSystemd()
		}

		if strings.Contains(linkTarget, "busybox") {
			return g.getInitSystemFromInittab()
		}

		return nil
	}

	// Check if we have upstart.
	if lxd.PathExists(filepath.Join(g.sourceDir, "sbin", "initctl")) {
		return g.handleUpstart()
	}

	return g.getInitSystemFromInittab()
}

// Run does nothing.
func (g *lxdAgent) Run() error {
	return nil
}

func (g *lxdAgent) handleSystemd() error {
	systemdPath := filepath.Join("/", "lib", "systemd")
	if !lxd.PathExists(filepath.Join(g.sourceDir, systemdPath)) {
		systemdPath = filepath.Join("/", "usr", "lib", "systemd")
	}

	lxdAgentServiceUnit := fmt.Sprintf(`[Unit]
Description=LXD - agent
Documentation=https://linuxcontainers.org/lxd
ConditionPathExists=/dev/virtio-ports/org.linuxcontainers.lxd
Before=cloud-init.target cloud-init.service cloud-init-local.service
DefaultDependencies=no

[Service]
Type=notify
WorkingDirectory=-/run/lxd_agent
ExecStartPre=%s/lxd-agent-setup
ExecStart=/run/lxd_agent/lxd-agent
Restart=on-failure
RestartSec=5s
StartLimitInterval=60
StartLimitBurst=10

[Install]
WantedBy=multi-user.target
`, systemdPath)

	path := filepath.Join(g.sourceDir, systemdPath, "system", "lxd-agent.service")

	err := os.WriteFile(path, []byte(lxdAgentServiceUnit), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", path, err)
	}

	err = os.Symlink(path, filepath.Join(g.sourceDir, "/etc/systemd/system/multi-user.target.wants/lxd-agent.service"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/systemd/system/multi-user.target.wants/lxd-agent.service"), err)
	}

	lxdAgentSetupScript := `#!/bin/sh
set -eu
PREFIX="/run/lxd_agent"

# Functions.
mount_virtiofs() {
    mount -t virtiofs config "${PREFIX}/.mnt" >/dev/null 2>&1
}

mount_9p() {
    /sbin/modprobe 9pnet_virtio >/dev/null 2>&1 || true
    /bin/mount -t 9p config "${PREFIX}/.mnt" -o access=0,trans=virtio,size=1048576 >/dev/null 2>&1
}

fail() {
    umount -l "${PREFIX}" >/dev/null 2>&1 || true
    rmdir "${PREFIX}" >/dev/null 2>&1 || true
    echo "${1}"
    exit 1
}

# Setup the mount target.
umount -l "${PREFIX}" >/dev/null 2>&1 || true
mkdir -p "${PREFIX}"
mount -t tmpfs tmpfs "${PREFIX}" -o mode=0700,size=50M
mkdir -p "${PREFIX}/.mnt"

# Try virtiofs first.
mount_virtiofs || mount_9p || fail "Couldn't mount virtiofs or 9p, failing."

# Copy the data.
cp -Ra "${PREFIX}/.mnt/"* "${PREFIX}"

# Unmount the temporary mount.
umount "${PREFIX}/.mnt"
rmdir "${PREFIX}/.mnt"

# Fix up permissions.
chown -R root:root "${PREFIX}"
`

	path = filepath.Join(g.sourceDir, systemdPath, "lxd-agent-setup")

	err = os.WriteFile(path, []byte(lxdAgentSetupScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", path, err)
	}

	udevPath := filepath.Join("/", "lib", "udev", "rules.d")
	stat, err := os.Lstat(filepath.Join(g.sourceDir, "lib", "udev"))
	if err == nil && stat.Mode()&os.ModeSymlink != 0 || !lxd.PathExists(filepath.Dir(filepath.Join(g.sourceDir, udevPath))) {
		udevPath = filepath.Join("/", "usr", "lib", "udev", "rules.d")
	}

	lxdAgentRules := `ACTION=="add", SYMLINK=="virtio-ports/org.linuxcontainers.lxd", TAG+="systemd", ACTION=="add", RUN+="/bin/systemctl start lxd-agent.service"`
	err = os.WriteFile(filepath.Join(g.sourceDir, udevPath, "99-lxd-agent.rules"), []byte(lxdAgentRules), 0400)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, udevPath, "99-lxd-agent.rules"), err)
	}

	return nil
}

func (g *lxdAgent) handleOpenRC() error {
	lxdAgentScript := `#!/sbin/openrc-run

description="LXD - agent"
command=/run/lxd_config/drive/lxd-agent
command_background=true
pidfile=/run/lxd-agent.pid
start_stop_daemon_args="--chdir /run/lxd_config/drive"
required_dirs=/run/lxd_config/drive

depend() {
	want lxd-agent-virtiofs
	after lxd-agent-virtiofs
	want lxd-agent-9p
	after lxd-agent-9p
	before cloud-init
	before cloud-init-local
}
`

	err := os.WriteFile(filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent"), []byte(lxdAgentScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent"), err)
	}

	err = os.Symlink("/etc/init.d/lxd-agent", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent"), err)
	}

	lxdConfigShareMountScript := `#!/sbin/openrc-run

description="LXD - agent - 9p mount"
command=/bin/mount
command_args="-t 9p config /run/lxd_config/drive -o access=0,trans=virtio"
required_files=/dev/virtio-ports/org.linuxcontainers.lxd

start_pre() {
	/sbin/modprobe 9pnet_virtio || true
	# Don't proceed if the config drive is mounted already
	mount | grep -q /run/lxd_config/drive && return 1
	checkpath -d /run/lxd_config -m 0700
	checkpath -d /run/lxd_config/drive
}
`

	err = os.WriteFile(filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent-9p"), []byte(lxdConfigShareMountScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent-9p"), err)
	}

	err = os.Symlink("/etc/init.d/lxd-agent-9p", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent-9p"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent-9p"), err)
	}

	lxdConfigShareMountVirtioFSScript := `#!/sbin/openrc-run

	description="LXD - agent - virtio-fs mount"
	command=/bin/mount
	command_args="-t virtiofs config /run/lxd_config/drive"
	required_files=/dev/virtio-ports/org.linuxcontainers.lxd

	start_pre() {
		# Don't proceed if the config drive is mounted already
		mount | grep -q /run/lxd_config/drive && return 1
		checkpath -d /run/lxd_config -m 0700
		checkpath -d /run/lxd_config/drive
	}
	`

	err = os.WriteFile(filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent-virtiofs"), []byte(lxdConfigShareMountVirtioFSScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent-virtiofs"), err)
	}

	err = os.Symlink("/etc/init.d/lxd-agent-virtiofs", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent-virtiofs"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent-virtiofs"), err)
	}

	return nil
}

func (g *lxdAgent) handleUpstart() error {
	lxdAgentScript := `Description "LXD agent"

start on runlevel [2345]
stop on runlevel [!2345]

respawn
respawn limit 10 5

exec lxd-agent
`

	err := os.WriteFile(filepath.Join(g.sourceDir, "/etc/init/lxd-agent"), []byte(lxdAgentScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init/lxd-agent"), err)
	}

	lxdConfigShareMountScript := `Description "LXD agent 9p mount"

start on stopped lxd-agent-virtiofs

pre-start script
	if mount | grep -q /run/lxd_config/drive; then
		stop
		exit 0
	fi

	if ! modprobe 9pnet_virtio; then
		stop
		exit 0
	fi

	mkdir -p /run/lxd_config/drive
	chmod 0700 /run/lxd_config
end script

task

exec mount -t 9p config /run/lxd_config/drive -o access=0,trans=virtio
`

	err = os.WriteFile(filepath.Join(g.sourceDir, "/etc/init/lxd-agent-9p"), []byte(lxdConfigShareMountScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init/lxd-agent-9p"), err)
	}

	lxdConfigShareMountVirtioFSScript := `Description "LXD agent virtio-fs mount"

start on runlevel filesystem

pre-start script
	if mount | grep -q /run/lxd_config/drive; then
		stop
		exit 0
	fi

	mkdir -p /run/lxd_config/drive
	chmod 0700 /run/lxd_config
end script

task

exec mount -t virtiofs config /run/lxd_config/drive
`

	err = os.WriteFile(filepath.Join(g.sourceDir, "/etc/init/lxd-agent-virtiofs"), []byte(lxdConfigShareMountVirtioFSScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init/lxd-agent-virtiofs"), err)
	}

	return nil
}

func (g *lxdAgent) getInitSystemFromInittab() error {
	f, err := os.Open(filepath.Join(g.sourceDir, "etc", "inittab"))
	if err != nil {
		return fmt.Errorf("Failed to open file %q: %w", filepath.Join(g.sourceDir, "etc", "inittab"), err)
	}

	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "sysinit") && strings.Contains(scanner.Text(), "openrc") {
			return g.handleOpenRC()
		}
	}

	return errors.New("Failed to determine init system")
}
