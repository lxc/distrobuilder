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

func setupMounts(rootfs string, mounts []chrootMount) error {
	err := os.MkdirAll(filepath.Join(rootfs, ".distrobuilder"), 0700)
	if err != nil {
		return err
	}

	for i, mount := range mounts {
		tmpTarget := filepath.Join(rootfs, ".distrobuilder", fmt.Sprintf("%d", i))
		if mount.isDir {
			err := os.MkdirAll(tmpTarget, 0755)
			if err != nil {
				return err
			}
		} else {
			_, err = os.Create(tmpTarget)
			if err != nil {
				return err
			}
		}

		err := syscall.Mount(mount.source, tmpTarget, mount.fstype, mount.flags, mount.data)
		if err != nil {
			return fmt.Errorf("Failed to mount '%s': %s", mount.source, err)
		}
	}

	return nil
}

func moveMounts(mounts []chrootMount) error {
	for i, mount := range mounts {
		tmpSource := filepath.Join("/", ".distrobuilder", fmt.Sprintf("%d", i))
		if mount.isDir {
			err := os.MkdirAll(mount.target, 0755)
			if err != nil {
				return err
			}
		} else {
			_, err := os.Create(mount.target)
			if err != nil {
				return err
			}
		}

		err := syscall.Mount(tmpSource, mount.target, "", syscall.MS_MOVE, "")
		if err != nil {
			return fmt.Errorf("Failed to mount '%s': %s", mount.source, err)
		}
	}

	err := os.RemoveAll(filepath.Join("/", ".distrobuilder"))
	if err != nil {
		return err
	}

	return nil

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
		{rootfs, "/", "", syscall.MS_BIND, "", true},
		{"none", "/proc", "proc", 0, "", true},
		{"none", "/sys", "sysfs", 0, "", true},
		{"/dev", "/dev", "", syscall.MS_BIND, "", true},
		{"none", "/run", "tmpfs", 0, "", true},
		{"none", "/tmp", "tmpfs", 0, "", true},
		{"/etc/resolv.conf", "/etc/resolv.conf", "", syscall.MS_BIND, "", false},
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = setupMounts(rootfs, mounts)
	if err != nil {
		return nil, fmt.Errorf("Failed to mount filesystems: %v", err)
	}

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

	err = moveMounts(mounts)
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
