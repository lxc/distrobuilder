package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

type chrootMount struct {
	source string
	target string
	fstype string
	flags  uintptr
	data   string
	isDir  bool
}

func mountFilesystems(rootfs string, mounts []chrootMount) bool {
	ok := true

	for _, mount := range mounts {
		if mount.isDir {
			os.MkdirAll(filepath.Join(rootfs, mount.target), 0755)
		} else {
			os.Create(filepath.Join(rootfs, mount.target))
		}
		err := syscall.Mount(mount.source, filepath.Join(rootfs, mount.target),
			mount.fstype, mount.flags, mount.data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to mount '%s': %s\n", mount.source, err)
			ok = false
			break
		}
	}

	return ok
}

func killChrootProcesses(rootfs string) error {
	proc, err := os.Open(filepath.Join(rootfs, "proc"))
	if err != nil {
		return err
	}

	dirs, err := proc.Readdirnames(0)
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		match, _ := regexp.MatchString(`\d+`, dir)
		if match {
			link, _ := os.Readlink(filepath.Join(rootfs, "proc", dir, "root"))
			if link == rootfs {
				pid, _ := strconv.Atoi(dir)
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}

	return nil
}

func setupChroot(rootfs string) (func() error, error) {
	var err error

	mounts := []chrootMount{
		{rootfs, "", "tmpfs", syscall.MS_BIND, "", true},
		{"proc", "/proc", "proc", 0, "", true},
		{"sys", "/sys", "sysfs", 0, "", true},
		{"udev", "/dev", "devtmpfs", 0, "", true},
		{"shm", "/dev/shm", "tmpfs", 0, "", true},
		{"/dev/pts", "/dev/pts", "tmpfs", syscall.MS_BIND, "", true},
		{"run", "/run", "tmpfs", 0, "", true},
		{"tmp", "/tmp", "tmpfs", 0, "", true},
		{"/etc/resolv.conf", "/etc/resolv.conf", "", syscall.MS_BIND, "", false},
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	ok := mountFilesystems(rootfs, mounts)
	if !ok {
		return nil, fmt.Errorf("Failed to mount filesystems")
	}

	syscall.Mknod(filepath.Join(rootfs, "dev", "null"), 1, 3)

	root, err := os.Open("/")
	if err != nil {
		return nil, err
	}

	err = syscall.Chroot(rootfs)
	if err != nil {
		root.Close()
		return nil, err
	}

	err = syscall.Chdir("/")
	if err != nil {
		return nil, err
	}

	return func() error {
		defer root.Close()

		err = root.Chdir()
		if err != nil {
			return err
		}

		err = syscall.Chroot(".")
		if err != nil {
			return err
		}

		err = syscall.Chdir(cwd)
		if err != nil {
			return err
		}

		// This will kill all processes in the chroot and allow to cleanly
		// unmount everything.
		killChrootProcesses(rootfs)
		syscall.Unmount(rootfs, syscall.MNT_DETACH)

		return nil
	}, nil
}

func managePackages(def shared.DefinitionPackages) error {
	var err error

	manager := managers.Get(def.Manager)
	if manager == nil {
		return fmt.Errorf("Couldn't get manager")
	}

	err = manager.Refresh()
	if err != nil {
		return err
	}

	if def.Update {
		err = manager.Update()
		if err != nil {
			return err
		}
	}

	err = manager.Install(def.Install)
	if err != nil {
		return err
	}

	err = manager.Remove(def.Remove)
	if err != nil {
		return err
	}

	err = manager.Clean()
	if err != nil {
		return err
	}

	return nil
}
