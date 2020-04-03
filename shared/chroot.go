package shared

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"
	"golang.org/x/sys/unix"
)

// ChrootMount defines mount args.
type ChrootMount struct {
	Source string
	Target string
	FSType string
	Flags  uintptr
	Data   string
	IsDir  bool
}

// ActiveChroots is a map of all active chroots and their exit functions
var ActiveChroots = make(map[string]func() error)

func setupMounts(rootfs string, mounts []ChrootMount) error {
	// Create a temporary mount path
	err := os.MkdirAll(filepath.Join(rootfs, ".distrobuilder"), 0700)
	if err != nil {
		return err
	}

	for i, mount := range mounts {
		// Target path
		tmpTarget := filepath.Join(rootfs, ".distrobuilder", fmt.Sprintf("%d", i))

		// Create the target mountpoint
		if mount.IsDir {
			err := os.MkdirAll(tmpTarget, 0755)
			if err != nil {
				return err
			}
		} else {
			f, err := os.Create(tmpTarget)
			if err != nil {
				return err
			}
			f.Close()
		}

		// Mount to the temporary path
		err := unix.Mount(mount.Source, tmpTarget, mount.FSType, mount.Flags, mount.Data)
		if err != nil {
			return errors.Wrapf(err, "Failed to mount '%s'", mount.Source)
		}
	}

	return nil
}

func moveMounts(mounts []ChrootMount) error {
	for i, mount := range mounts {
		// Source path
		tmpSource := filepath.Join("/", ".distrobuilder", fmt.Sprintf("%d", i))

		// Resolve symlinks
		target := mount.Target
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
		err := os.MkdirAll(filepath.Dir(target), 0755)
		if err != nil {
			return err
		}

		// Create target path
		if mount.IsDir {
			err = os.MkdirAll(target, 0755)
			if err != nil {
				return err
			}
		} else {
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			f.Close()
		}

		// Move the mount to its destination
		err = unix.Mount(tmpSource, target, "", unix.MS_MOVE, "")
		if err != nil {
			return errors.Wrapf(err, "Failed to mount '%s'", mount.Source)
		}
	}

	// Cleanup our temporary path
	err := os.RemoveAll(filepath.Join("/", ".distrobuilder"))
	if err != nil {
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
	re := regexp.MustCompile(`\d+`)

	for _, dir := range dirs {
		if re.MatchString(dir) {
			link, _ := os.Readlink(filepath.Join(rootfs, "proc", dir, "root"))
			if link == rootfs {
				pid, _ := strconv.Atoi(dir)
				unix.Kill(pid, unix.SIGKILL)
			}
		}
	}

	return nil
}

// SetupChroot sets up mount and files, a reverter and then chroots for you
func SetupChroot(rootfs string, envs DefinitionEnv, m []ChrootMount) (func() error, error) {
	// Mount the rootfs
	err := unix.Mount(rootfs, rootfs, "", unix.MS_BIND, "")
	if err != nil {
		return nil, errors.Wrapf(err, "Failed to mount '%s'", rootfs)
	}

	// Setup all other needed mounts
	mounts := []ChrootMount{
		{"none", "/proc", "proc", 0, "", true},
		{"none", "/sys", "sysfs", 0, "", true},
		{"none", "/run", "tmpfs", 0, "", true},
		{"none", "/tmp", "tmpfs", 0, "", true},
		{"none", "/dev", "tmpfs", 0, "", true},
		{"none", "/dev/shm", "tmpfs", 0, "", true},
		{"/etc/resolv.conf", "/etc/resolv.conf", "", unix.MS_BIND, "", false},
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
	if m != nil && len(m) > 0 {
		err = setupMounts(rootfs, append(mounts, m...))
	} else {
		err = setupMounts(rootfs, mounts)
	}
	if err != nil {
		return nil, errors.Wrap(err, "Failed to mount filesystems")
	}

	// Chroot into the container's rootfs
	err = unix.Chroot(rootfs)
	if err != nil {
		root.Close()
		return nil, err
	}

	err = unix.Chdir("/")
	if err != nil {
		return nil, err
	}

	// Move all the mounts into place
	err = moveMounts(append(mounts, m...))
	if err != nil {
		return nil, err
	}

	// Populate /dev directory instead of bind mounting it from the host
	err = populateDev()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to populate /dev")
	}

	// Change permission for /dev/shm
	err = unix.Chmod("/dev/shm", 01777)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to chmod /dev/shm")
	}

	var env Environment

	if envs.ClearDefaults {
		env = Environment{}
	} else {
		env = Environment{
			"PATH": EnvVariable{
				Value: "/sbin:/bin:/usr/sbin:/usr/bin:/usr/local/sbin:/usr/local/bin",
				Set:   true,
			},
			"SHELL": EnvVariable{
				Value: "/bin/sh",
				Set:   true,
			},
			"TERM": EnvVariable{
				Value: "xterm",
				Set:   true,
			},
			"DEBIAN_FRONTEND": EnvVariable{
				Value: "noninteractive",
				Set:   true,
			},
		}
	}

	if envs.EnvVariables != nil && len(envs.EnvVariables) > 0 {
		for _, e := range envs.EnvVariables {
			entry, ok := env[e.Key]
			if ok {
				entry.Value = e.Value
				entry.Set = true
			} else {
				env[e.Key] = EnvVariable{
					Value: e.Value,
					Set:   true,
				}
			}
		}
	}

	// Set environment variables
	oldEnv := SetEnvVariables(env)

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

	exitFunc := func() error {
		defer root.Close()

		// Cleanup policy-rc.d
		if policyCleanup {
			err = os.Remove("/usr/sbin/policy-rc.d")
			if err != nil {
				return err
			}
		}

		// Reset old environment variables
		SetEnvVariables(oldEnv)

		// Switch back to the host rootfs
		err = root.Chdir()
		if err != nil {
			return err
		}

		err = unix.Chroot(".")
		if err != nil {
			return err
		}

		err = unix.Chdir(cwd)
		if err != nil {
			return err
		}

		// This will kill all processes in the chroot and allow to cleanly
		// unmount everything.
		killChrootProcesses(rootfs)

		// And now unmount the entire tree
		unix.Unmount(rootfs, unix.MNT_DETACH)

		devPath := filepath.Join(rootfs, "dev")

		// Wipe $rootfs/dev
		err := os.RemoveAll(devPath)
		if err != nil {
			return err
		}

		ActiveChroots[rootfs] = nil

		return os.MkdirAll(devPath, 0755)
	}

	ActiveChroots[rootfs] = exitFunc

	return exitFunc, nil
}

func populateDev() error {
	devs := []struct {
		Path  string
		Major uint32
		Minor uint32
		Mode  uint32
	}{
		{"/dev/console", 5, 1, unix.S_IFCHR | 0640},
		{"/dev/full", 1, 7, unix.S_IFCHR | 0666},
		{"/dev/null", 1, 3, unix.S_IFCHR | 0666},
		{"/dev/random", 1, 8, unix.S_IFCHR | 0666},
		{"/dev/tty", 5, 0, unix.S_IFCHR | 0666},
		{"/dev/urandom", 1, 9, unix.S_IFCHR | 0666},
		{"/dev/zero", 1, 5, unix.S_IFCHR | 0666},
	}

	for _, d := range devs {
		if lxd.PathExists(d.Path) {
			continue
		}

		dev := unix.Mkdev(d.Major, d.Minor)

		err := unix.Mknod(d.Path, d.Mode, int(dev))
		if err != nil {
			return errors.Wrapf(err, "Failed to create %q", d.Path)
		}

		// For some odd reason, unix.Mknod will not set the mode correctly.
		// This fixes that.
		err = unix.Chmod(d.Path, d.Mode)
		if err != nil {
			return errors.Wrapf(err, "Failed to chmod %q", d.Path)
		}
	}

	symlinks := []struct {
		Symlink string
		Target  string
	}{
		{"/dev/fd", "/proc/self/fd"},
		{"/dev/stdin", "/proc/self/fd/0"},
		{"/dev/stdout", "/proc/self/fd/1"},
		{"/dev/stderr", "/proc/self/fd/2"},
	}

	for _, l := range symlinks {
		err := os.Symlink(l.Target, l.Symlink)
		if err != nil {
			return errors.Wrapf(err, "Failed to create link %q -> %q", l.Symlink, l.Target)
		}
	}

	return nil
}
