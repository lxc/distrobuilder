package generators

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	incus "github.com/lxc/incus/shared/util"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

var incusAgentSetupScript = `#!/bin/sh
set -eu
PREFIX="/run/incus_agent"
CDROM="/dev/disk/by-id/scsi-0QEMU_QEMU_CD-ROM_incus_agent"

# Functions.
mount_virtiofs() {
    mount -t virtiofs config "${PREFIX}.mnt" >/dev/null 2>&1
}

mount_9p() {
    modprobe 9pnet_virtio >/dev/null 2>&1 || true
    mount -t 9p config "${PREFIX}.mnt" -o access=0,trans=virtio,size=1048576 >/dev/null 2>&1
}

mount_cdrom() {
    mount "${CDROM}" "${PREFIX}.mnt" >/dev/null 2>&1
}

fail() {
    # Check if we already have an agent in place.
    # This will typically be true during restart in the case of a cdrom-based setup.
    if [ -x "${PREFIX}/incus-agent" ]; then
        echo "${1}, re-using existing agent"
        exit 0
    fi

    # Cleanup and fail.
    umount -l "${PREFIX}" >/dev/null 2>&1 || true
    eject "${CDROM}" >/dev/null 2>&1 || true
    rmdir "${PREFIX}" >/dev/null 2>&1 || true
    echo "${1}, failing"

    exit 1
}

# Try getting an agent drive.
mkdir -p "${PREFIX}.mnt"
mount_9p || mount_virtiofs || mount_cdrom || fail "Couldn't mount 9p or cdrom"

# Setup the mount target.
umount -l "${PREFIX}" >/dev/null 2>&1 || true
mkdir -p "${PREFIX}"
mount -t tmpfs tmpfs "${PREFIX}" -o mode=0700,size=50M

# Copy the data.
cp -Ra "${PREFIX}.mnt/"* "${PREFIX}"

# Unmount the temporary mount.
umount "${PREFIX}.mnt"
rmdir "${PREFIX}.mnt"

# Eject the cdrom in case it's present.
eject "${CDROM}" >/dev/null 2>&1 || true

# Fix up permissions.
chown -R root:root "${PREFIX}"

# Legacy.
if [ ! -e "${PREFIX}/incus-agent" ] && [ -e "${PREFIX}/lxd-agent" ]; then
    ln -s lxd-agent "${PREFIX}"/incus-agent
fi

exit 0
`

type incusAgent struct {
	common
}

// RunLXC is not supported.
func (g *incusAgent) RunLXC(img *image.LXCImage, target shared.DefinitionTargetLXC) error {
	return ErrNotSupported
}

// RunIncus creates systemd unit files for the agent.
func (g *incusAgent) RunIncus(img *image.IncusImage, target shared.DefinitionTargetIncus) error {
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
func (g *incusAgent) Run() error {
	return nil
}

func (g *incusAgent) handleSystemd() error {
	systemdPath := filepath.Join("/", "lib", "systemd")
	if !incus.PathExists(filepath.Join(g.sourceDir, systemdPath)) {
		systemdPath = filepath.Join("/", "usr", "lib", "systemd")
	}

	incusAgentServiceUnit := fmt.Sprintf(`[Unit]
Description=Incus - agent
Documentation=https://linuxcontainers.org/incus/docs/main/
Before=multi-user.target cloud-init.target cloud-init.service cloud-init-local.service
DefaultDependencies=no

[Service]
Type=notify
WorkingDirectory=-/run/incus_agent
ExecStartPre=%s/incus-agent-setup
ExecStart=/run/incus_agent/incus-agent
Restart=on-failure
RestartSec=5s
StartLimitInterval=60
StartLimitBurst=10
`, systemdPath)

	path := filepath.Join(g.sourceDir, systemdPath, "system", "incus-agent.service")

	err := os.WriteFile(path, []byte(incusAgentServiceUnit), 0644)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", path, err)
	}

	path = filepath.Join(g.sourceDir, systemdPath, "incus-agent-setup")

	err = os.WriteFile(path, []byte(incusAgentSetupScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", path, err)
	}

	udevPath := filepath.Join("/", "lib", "udev", "rules.d")
	stat, err := os.Lstat(filepath.Join(g.sourceDir, "lib", "udev"))
	if err == nil && stat.Mode()&os.ModeSymlink != 0 || !incus.PathExists(filepath.Dir(filepath.Join(g.sourceDir, udevPath))) {
		udevPath = filepath.Join("/", "usr", "lib", "udev", "rules.d")
	}

	incusAgentRules := `SYMLINK=="virtio-ports/org.linuxcontainers.incus", TAG+="systemd", ENV{SYSTEMD_WANTS}+="incus-agent.service"

# Legacy.
SYMLINK=="virtio-ports/org.linuxcontainers.lxd", TAG+="systemd", ENV{SYSTEMD_WANTS}+="incus-agent.service"
`
	err = os.WriteFile(filepath.Join(g.sourceDir, udevPath, "99-incus-agent.rules"), []byte(incusAgentRules), 0400)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, udevPath, "99-incus-agent.rules"), err)
	}

	return nil
}

func (g *incusAgent) handleOpenRC() error {
	incusAgentScript := `#!/sbin/openrc-run

description="Incus - agent"
command=/run/incus_agent/incus-agent
command_background=true
pidfile=/run/incus-agent.pid
start_stop_daemon_args="--chdir /run/incus_agent"
required_dirs=/run/incus_agent

depend() {
	need incus-agent-setup
	after incus-agent-setup
	before cloud-init
	before cloud-init-local
}
`

	err := os.WriteFile(filepath.Join(g.sourceDir, "/etc/init.d/incus-agent"), []byte(incusAgentScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init.d/incus-agent"), err)
	}

	err = os.Symlink("/etc/init.d/incus-agent", filepath.Join(g.sourceDir, "/etc/runlevels/default/incus-agent"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/runlevels/default/incus-agent"), err)
	}

	incusConfigShareMountScript := `#!/sbin/openrc-run

description="Incus - agent - setup"
command=/usr/local/bin/incus-agent-setup
required_dirs=/dev/virtio-ports/
`

	err = os.WriteFile(filepath.Join(g.sourceDir, "/etc/init.d/incus-agent-setup"), []byte(incusConfigShareMountScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", filepath.Join(g.sourceDir, "/etc/init.d/incus-agent-setup"), err)
	}

	err = os.Symlink("/etc/init.d/incus-agent-setup", filepath.Join(g.sourceDir, "/etc/runlevels/default/incus-agent-setup"))
	if err != nil {
		return fmt.Errorf("Failed to create symlink %q: %w", filepath.Join(g.sourceDir, "/etc/runlevels/default/incus-agent-setup"), err)
	}

	path := filepath.Join(g.sourceDir, "/usr/local/bin", "incus-agent-setup")

	err = os.WriteFile(path, []byte(incusAgentSetupScript), 0755)
	if err != nil {
		return fmt.Errorf("Failed to write file %q: %w", path, err)
	}

	return nil
}

func (g *incusAgent) getInitSystemFromInittab() error {
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
