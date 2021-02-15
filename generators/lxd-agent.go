package generators

import (
	"bufio"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
)

// LXDAgentGenerator represents the lxd-agent generator.
type LXDAgentGenerator struct{}

// RunLXC is not supported.
func (g LXDAgentGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage, target shared.DefinitionTargetLXC, defFile shared.DefinitionFile) error {
	return ErrNotSupported
}

// RunLXD creates systemd unit files for the lxd-agent.
func (g LXDAgentGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage, target shared.DefinitionTargetLXD, defFile shared.DefinitionFile) error {
	initFile := filepath.Join(sourceDir, "sbin", "init")

	fi, err := os.Lstat(initFile)
	if err != nil {
		return err
	}

	if fi.Mode()&os.ModeSymlink != 0 {
		linkTarget, err := os.Readlink(initFile)
		if err != nil {
			return err
		}

		if strings.Contains(linkTarget, "systemd") {
			return g.handleSystemd(sourceDir)
		}

		if strings.Contains(linkTarget, "busybox") {
			return g.getInitSystemFromInittab(sourceDir)
		}

		return nil
	}

	// Check if we have upstart.
	if lxd.PathExists(filepath.Join(sourceDir, "sbin", "initctl")) {
		return g.handleUpstart(sourceDir)
	}

	return g.getInitSystemFromInittab(sourceDir)
}

// Run does nothing.
func (g LXDAgentGenerator) Run(cacheDir, sourceDir string,
	defFile shared.DefinitionFile) error {
	return nil
}

func (g LXDAgentGenerator) handleSystemd(sourceDir string) error {
	systemdPath := filepath.Join("/", "lib", "systemd")
	if !lxd.PathExists(filepath.Join(sourceDir, systemdPath)) {
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

	err := ioutil.WriteFile(filepath.Join(sourceDir, systemdPath, "system", "lxd-agent.service"), []byte(lxdAgentServiceUnit), 0644)
	if err != nil {
		return err
	}

	err = os.Symlink(filepath.Join(sourceDir, systemdPath, "system", "lxd-agent.service"), filepath.Join(sourceDir, "/etc/systemd/system/multi-user.target.wants/lxd-agent.service"))
	if err != nil {
		return err
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
    /bin/mount -t 9p config "${PREFIX}/.mnt" -o access=0,trans=virtio >/dev/null 2>&1
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

	err = ioutil.WriteFile(filepath.Join(sourceDir, systemdPath, "lxd-agent-setup"), []byte(lxdAgentSetupScript), 0755)
	if err != nil {
		return err
	}

	udevPath := filepath.Join("/", "lib", "udev", "rules.d")
	stat, err := os.Lstat(filepath.Join(sourceDir, "lib", "udev"))
	if err == nil && stat.Mode()&os.ModeSymlink != 0 || !lxd.PathExists(filepath.Dir(filepath.Join(sourceDir, udevPath))) {
		udevPath = filepath.Join("/", "usr", "lib", "udev", "rules.d")
	}

	lxdAgentRules := `ACTION=="add", SYMLINK=="virtio-ports/org.linuxcontainers.lxd", TAG+="systemd", ACTION=="add", RUN+="/bin/systemctl start lxd-agent.service"`
	err = ioutil.WriteFile(filepath.Join(sourceDir, udevPath, "99-lxd-agent.rules"), []byte(lxdAgentRules), 0400)
	if err != nil {
		return err
	}

	return nil
}

func (g LXDAgentGenerator) handleOpenRC(sourceDir string) error {
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

	err := ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init.d/lxd-agent"), []byte(lxdAgentScript), 0755)
	if err != nil {
		return err
	}

	err = os.Symlink("/etc/init.d/lxd-agent", filepath.Join(sourceDir, "/etc/runlevels/default/lxd-agent"))
	if err != nil {
		return err
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

	err = ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init.d/lxd-agent-9p"), []byte(lxdConfigShareMountScript), 0755)
	if err != nil {
		return err
	}

	err = os.Symlink("/etc/init.d/lxd-agent-9p", filepath.Join(sourceDir, "/etc/runlevels/default/lxd-agent-9p"))
	if err != nil {
		return err
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

	err = ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init.d/lxd-agent-virtiofs"), []byte(lxdConfigShareMountVirtioFSScript), 0755)
	if err != nil {
		return err
	}

	err = os.Symlink("/etc/init.d/lxd-agent-virtiofs", filepath.Join(sourceDir, "/etc/runlevels/default/lxd-agent-virtiofs"))
	if err != nil {
		return err
	}

	return nil
}

func (g LXDAgentGenerator) handleUpstart(sourceDir string) error {
	lxdAgentScript := `Description "LXD agent"

start on runlevel [2345]
stop on runlevel [!2345]

respawn
respawn limit 10 5

exec lxd-agent
`

	err := ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init/lxd-agent"), []byte(lxdAgentScript), 0755)
	if err != nil {
		return err
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

	err = ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init/lxd-agent-9p"), []byte(lxdConfigShareMountScript), 0755)
	if err != nil {
		return err
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

	err = ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init/lxd-agent-virtiofs"), []byte(lxdConfigShareMountVirtioFSScript), 0755)
	if err != nil {
		return err
	}

	return nil
}

func (g LXDAgentGenerator) getInitSystemFromInittab(sourceDir string) error {
	f, err := os.Open(filepath.Join(sourceDir, "etc", "inittab"))
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "sysinit") && strings.Contains(scanner.Text(), "openrc") {
			return g.handleOpenRC(sourceDir)
		}
	}

	return errors.New("Failed to determine init system")
}
