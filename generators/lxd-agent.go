package generators

import (
	"bufio"
	"errors"
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
	lxdAgentServiceUnit := `[Unit]
Description=LXD - agent
Documentation=https://linuxcontainers.org/lxd
ConditionPathExists=/dev/virtio-ports/org.linuxcontainers.lxd
Requires=lxd-agent-9p.service
After=lxd-agent-9p.service
Before=cloud-init.target cloud-init.service cloud-init-local.service
DefaultDependencies=no

[Service]
Type=notify
WorkingDirectory=/run/lxd_config/9p
ExecStart=/run/lxd_config/9p/lxd-agent
Restart=on-failure
RestartSec=5s
StartLimitInterval=60
StartLimitBurst=10

[Install]
WantedBy=multi-user.target
`

	systemdPath := filepath.Join("/", "lib", "systemd", "system")
	if !lxd.PathExists(filepath.Dir(filepath.Join(sourceDir, systemdPath))) {
		systemdPath = filepath.Join("/", "usr", "lib", "systemd", "system")
	}

	err := ioutil.WriteFile(filepath.Join(sourceDir, systemdPath, "lxd-agent.service"), []byte(lxdAgentServiceUnit), 0644)
	if err != nil {
		return err
	}

	err = os.Symlink(filepath.Join(sourceDir, systemdPath, "lxd-agent.service"), filepath.Join(sourceDir, "/etc/systemd/system/multi-user.target.wants/lxd-agent.service"))
	if err != nil {
		return err
	}

	lxdConfigShareMountUnit := `[Unit]
Description=LXD - agent - 9p mount
Documentation=https://linuxcontainers.org/lxd
ConditionPathExists=/dev/virtio-ports/org.linuxcontainers.lxd
After=local-fs.target
DefaultDependencies=no

[Service]
Type=oneshot
RemainAfterExit=yes
ExecStartPre=-/sbin/modprobe 9pnet_virtio
ExecStartPre=/bin/mkdir -p /run/lxd_config/9p
ExecStartPre=/bin/chmod 0700 /run/lxd_config/
ExecStart=/bin/mount -t 9p config /run/lxd_config/9p -o access=0,trans=virtio

[Install]
WantedBy=multi-user.target
`

	err = ioutil.WriteFile(filepath.Join(sourceDir, systemdPath, "lxd-agent-9p.service"), []byte(lxdConfigShareMountUnit), 0644)
	if err != nil {
		return err
	}

	err = os.Symlink(filepath.Join(sourceDir, systemdPath, "lxd-agent-9p.service"), filepath.Join(sourceDir, "/etc/systemd/system/multi-user.target.wants/lxd-agent-9p.service"))
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
command=/run/lxd_config/9p/lxd-agent
command_background=true
pidfile=/run/lxd-agent.pid
start_stop_daemon_args="--chdir /run/lxd_config/9p"
required_dirs=/run/lxd_config/9p

depend() {
	need lxd-agent-9p
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
command_args="-t 9p config /run/lxd_config/9p -o access=0,trans=virtio"
required_files=/dev/virtio-ports/org.linuxcontainers.lxd

start_pre() {
	/sbin/modprobe 9pnet_virtio || true
	checkpath -d /run/lxd_config -m 0700
	checkpath -d /run/lxd_config/9p
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

start on runlevel filesystem

pre-start script
	if ! modprobe 9pnet_virtio; then
		stop
		exit 0
	fi

	mkdir -p /run/lxd_config/9p
	chmod 0700 /run/lxd_config
end script

task

exec mount -t 9p config /run/lxd_config/9p -o access=0,trans=virtio
`

	err = ioutil.WriteFile(filepath.Join(sourceDir, "/etc/init/lxd-agent-9p"), []byte(lxdConfigShareMountScript), 0755)
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
