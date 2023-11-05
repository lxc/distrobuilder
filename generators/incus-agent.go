package generators

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/incus/shared"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

var lxdAgentSetupScript = `#!/bin/sh
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

type lxdAgent struct {
	common
}

// RunLXC is not supported.
func (g *lxdAgent) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	return ErrNotSupported
}

// RunIncus creates systemd unit files for the lxd-agent.
func (g *lxdAgent) RunIncus(img *image.IncusImage, target shared.DefinitionTargetLXD) error {
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
Documentation=https://documentation.ubuntu.com/lxd/en/latest/
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

	lxdAgentRules := `ACTION=="add", SYMLINK=="virtio-ports/org.linuxcontainers.lxd", TAG+="systemd"
SYMLINK=="virtio-ports/org.linuxcontainers.lxd", RUN+="/bin/systemctl start lxd-agent.service"
`
	err = os.WriteFile(filepath.Join(g.sourceDir, udevPath, "99-lxd-agent.rules"), []byte(lxdAgentRules), 0400)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, udevPath, "99-lxd-agent.rules"), err)
	}

	return nil
}

func (g *lxdAgent) handleOpenRC() error {
	lxdAgentScript := `#!/sbin/openrc-run

description="LXD - agent"
command=/run/lxd_agent/lxd-agent
command_background=true
pidfile=/run/lxd-agent.pid
start_stop_daemon_args="--chdir /run/lxd_agent"
required_dirs=/run/lxd_agent

depend() {
	need lxd-agent-setup
	after lxd-agent-setup
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

description="LXD - agent - setup"
command=/usr/local/bin/lxd-agent-setup
required_files=/dev/virtio-ports/org.linuxcontainers.lxd
`

	err = os.WriteFile(filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent-setup"), []byte(lxdConfigShareMountScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init.d/lxd-agent-setup"), err)
	}

	err = os.Symlink("/etc/init.d/lxd-agent-setup", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent-setup"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/runlevels/default/lxd-agent-setup"), err)
	}

	path := filepath.Join(g.sourceDir, "/usr/local/bin", "lxd-agent-setup")

	err = os.WriteFile(path, []byte(lxdAgentSetupScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", path, err)
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
