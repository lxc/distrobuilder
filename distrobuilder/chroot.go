package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"syscall"

	lxd "github.com/lxc/lxd/shared"

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
	// Create a temporary mount path
	if err := os.MkdirAll(filepath.Join(rootfs, ".distrobuilder"), 0700); err != nil {
		return err
	}

	for i, mount := range mounts {
		// Target path
		tmpTarget := filepath.Join(rootfs, ".distrobuilder", fmt.Sprintf("%d", i))

		// Create the target mountpoint
		if mount.isDir {
			if err := os.Mkdir(tmpTarget, 0755); err != nil {
				return err
			}
		} else {
			if _, err := os.Create(tmpTarget); err != nil {
				return err
			}
		}

		// Mount to the temporary path
		err := syscall.Mount(mount.source, tmpTarget, mount.fstype, mount.flags, mount.data)
		if err != nil {
			return fmt.Errorf("Failed to mount '%s': %s", mount.source, err)
		}
	}

	return nil
}

func moveMounts(mounts []chrootMount) error {
	for i, mount := range mounts {
		// Source path
		tmpSource := filepath.Join("/", ".distrobuilder", fmt.Sprintf("%d", i))

		// Resolve symlinks
		target := mount.target
		for {
			// Get information on current target
			fi, err := os.Lstat(target)
			if err != nil {
				break
			}

			// If not a symlink, we're done
			if fi.Mode()&os.ModeSymlink == 0 {
				break
			}

			// If a symlink, resolve it
			newTarget, err := os.Readlink(target)
			if err != nil {
				break
			}

			target = newTarget
		}

		// Create parent paths if missing
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}

		// Create target path
		if mount.isDir {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		} else {
			if _, err := os.Create(target); err != nil {
				return err
			}
		}

		// Move the mount to its destination
		if err := syscall.Mount(tmpSource, target, "", syscall.MS_MOVE, ""); err != nil {
			return fmt.Errorf("Failed to mount '%s': %s", mount.source, err)
		}
	}

	// Cleanup our temporary path
	if err := os.RemoveAll(filepath.Join("/", ".distrobuilder")); err != nil {
		return err
	}

	return nil
}

func killChrootProcesses(rootfs string) error {
	// List all files under /proc
	proc, err := os.Open(filepath.Join(rootfs, "proc"))
	if err != nil {
		return err
	}

	dirs, err := proc.Readdirnames(0)
	if err != nil {
		return err
	}

	// Get all processes and kill them
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
	// Mount the rootfs
	if err := syscall.Mount(rootfs, rootfs, "", syscall.MS_BIND, ""); err != nil {
		return nil, fmt.Errorf("Failed to mount '%s': %s", rootfs, err)
	}

	// Setup all other needed mounts
	mounts := []chrootMount{
		{"none", "/proc", "proc", 0, "", true},
		{"none", "/sys", "sysfs", 0, "", true},
		{"/dev", "/dev", "", syscall.MS_BIND, "", true},
		{"none", "/run", "tmpfs", 0, "", true},
		{"none", "/tmp", "tmpfs", 0, "", true},
		{"/etc/resolv.conf", "/etc/resolv.conf", "", syscall.MS_BIND, "", false},
	}

	// Keep a reference to the host rootfs and cwd
	root, err := os.Open("/")
	if err != nil {
		return nil, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	// Setup all needed mounts in a temporary location
	if err := setupMounts(rootfs, mounts); err != nil {
		return nil, fmt.Errorf("Failed to mount filesystems: %v", err)
	}

	// Chroot into the container's rootfs
	if err := syscall.Chroot(rootfs); err != nil {
		root.Close()
		return nil, err
	}

	if err := syscall.Chdir("/"); err != nil {
		return nil, err
	}

	// Move all the mounts into place
	if err := moveMounts(mounts); err != nil {
		return nil, err
	}

	// Set environment variables
	oldEnvVariables := shared.SetEnvVariables(
		[]shared.EnvVariable{
			{
				Key:   "PATH",
				Value: "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin",
				Set:   true,
			},
			{
				Key:   "SHELL",
				Value: "/bin/sh",
				Set:   true,
			},
			{
				Key:   "TERM",
				Value: "xterm",
				Set:   true,
			},
			{
				Key:   "DEBIAN_FRONTEND",
				Value: "noninteractive",
				Set:   true,
			},
		})

	// Setup policy-rc.d override
	policyCleanup := false
	if lxd.PathExists("/usr/sbin/") && !lxd.PathExists("/usr/sbin/policy-rc.d") {
		err = ioutil.WriteFile("/usr/sbin/policy-rc.d", []byte(`#!/bin/sh
exit 101
`), 0755)
		if err != nil {
			return nil, err
		}

		policyCleanup = true
	}

	return func() error {
		defer root.Close()

		// Cleanup policy-rc.d
		if policyCleanup {
			if err := os.Remove("/usr/sbin/policy-rc.d"); err != nil {
				return err
			}
		}

		// Reset old environment variables
		shared.SetEnvVariables(oldEnvVariables)

		// Switch back to the host rootfs
		if err := root.Chdir(); err != nil {
			return err
		}

		if err := syscall.Chroot("."); err != nil {
			return err
		}

		if err := syscall.Chdir(cwd); err != nil {
			return err
		}

		// This will kill all processes in the chroot and allow to cleanly
		// unmount everything.
		killChrootProcesses(rootfs)

		// And now unmount the entire tree
		syscall.Unmount(rootfs, syscall.MNT_DETACH)

		return nil
	}, nil
}

func managePackages(def shared.DefinitionPackages, actions []shared.DefinitionAction) error {
	manager := managers.Get(def.Manager)
	if manager == nil {
		return fmt.Errorf("Couldn't get manager")
	}

	if err := manager.Refresh(); err != nil {
		return err
	}

	if def.Update {
		if err := manager.Update(); err != nil {
			return err
		}

		// Run post update hook
		for _, action := range actions {
			if err := shared.RunScript(action.Action); err != nil {
				return fmt.Errorf("Failed to run post-update: %s", err)
			}
		}
	}

	if err := manager.Install(def.Install); err != nil {
		return err
	}

	if err := manager.Remove(def.Remove); err != nil {
		return err
	}

	if err := manager.Clean(); err != nil {
		return err
	}

	return nil
}
