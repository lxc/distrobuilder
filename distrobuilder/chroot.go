package main

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"github.com/lxc/distrobuilder/managers"
	"github.com/lxc/distrobuilder/shared"
)

func prepareChroot(cacheDir string) ([]string, error) {
	type Mount struct {
		source string
		target string
		fstype string
		flags  uintptr
		data   string
	}

	var (
		err      error
		unmounts []string
	)

	mounts := []Mount{
		{filepath.Join(cacheDir, "rootfs"), "", "tmpfs", syscall.MS_BIND, ""},
		{"proc", "proc", "proc", 0, ""},
		{"sys", "sys", "sysfs", 0, ""},
		{"udev", "dev", "devtmpfs", 0, ""},
		{"shm", "/dev/shm", "tmpfs", 0, ""},
		{"/dev/pts", "/dev/pts", "tmpfs", syscall.MS_BIND, ""},
		{"run", "/run", "tmpfs", 0, ""},
		{"tmp", "/tmp", "tmpfs", 0, ""},
	}

	os.MkdirAll(filepath.Join(cacheDir, "rootfs", "dev", "pts"), 0755)

	for _, mount := range mounts {
		err = syscall.Mount(mount.source, filepath.Join(cacheDir, "rootfs", mount.target),
			mount.fstype, mount.flags, mount.data)
		if err != nil {
			return unmounts, err
		}
		unmounts = append(unmounts, filepath.Join(cacheDir, "rootfs", mount.target))
	}

	return unmounts, nil
}

func setupChroot(cacheDir string) (func() error, error) {
	var err error

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	err = shared.Copy("/etc/resolv.conf", filepath.Join(cacheDir, "rootfs", "etc", "resolv.conf"))
	if err != nil {
		return nil, err
	}

	unmounts, err := prepareChroot(cacheDir)
	if err != nil {
		for i := len(unmounts) - 1; i >= 0; i-- {
			syscall.Unmount(unmounts[i], 0)
		}
		return nil, err
	}

	syscall.Mknod(filepath.Join(cacheDir, "rootfs", "dev", "null"), 1, 3)

	root, err := os.Open("/")
	if err != nil {
		return nil, err
	}

	err = syscall.Chroot(filepath.Join(cacheDir, "rootfs"))
	if err != nil {
		root.Close()
		return nil, err
	}

	err = syscall.Chdir("/")
	if err != nil {
		return nil, err
	}

	return func() error {
		// unmount targets in reversed order
		for i := len(unmounts) - 1; i >= 0; i-- {
			syscall.Unmount(unmounts[i], 0)
		}

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

		return nil
	}, nil
}

func manageChroot(def shared.DefinitionPackages) error {
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
