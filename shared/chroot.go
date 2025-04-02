package shared

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	incus "github.com/lxc/incus/v6/shared/util"
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

// ActiveChroots is a map of all active chroots and their exit functions.
var ActiveChroots = make(map[string]func() error)

func setupMounts(rootfs string, mounts []ChrootMount) error {
	// Create a temporary mount path
	err := os.MkdirAll(filepath.Join(rootfs, ".distrobuilder"), 0o700)
	if err != nil {
		return fmt.Errorf("Failed to create directory %q: %w", filepath.Join(rootfs, ".distrobuilder"), err)
	}

	for i, mount := range mounts {
		// Target path
		tmpTarget := filepath.Join(rootfs, ".distrobuilder", fmt.Sprintf("%d", i))

		// Create the target mountpoint
		if mount.IsDir {
			err := os.MkdirAll(tmpTarget, 0o755)
			if err != nil {
				return fmt.Errorf("Failed to create directory %q: %w", tmpTarget, err)
			}
		} else {
			f, err := os.Create(tmpTarget)
			if err != nil {
				return fmt.Errorf("Failed to create file %q: %w", tmpTarget, err)
			}

			f.Close()
		}

		// Mount to the temporary path
		err := unix.Mount(mount.Source, tmpTarget, mount.FSType, mount.Flags, mount.Data)
		if err != nil {
			return fmt.Errorf("Failed to mount '%s': %w", mount.Source, err)
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

		// If the target's parent directory is a symlink, we need to resolve that as well.
		targetDir := filepath.Dir(target)

		if incus.PathExists(targetDir) {
			// Get information on current target
			fi, err := os.Lstat(targetDir)
			if err != nil {
				return fmt.Errorf("Failed to stat directory %q: %w", targetDir, err)
			}

			// If a symlink, resolve it
			if fi.Mode()&os.ModeSymlink != 0 {
				newTarget, err := os.Readlink(targetDir)
				if err != nil {
					return fmt.Errorf("Failed to get destination of %q: %w", targetDir, err)
				}

				targetDir = newTarget
			}
		}

		// Create parent paths if missing
		err := os.MkdirAll(targetDir, 0o755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", targetDir, err)
		}

		// Create target path
		if mount.IsDir {
			err = os.MkdirAll(target, 0o755)
			if err != nil {
				return fmt.Errorf("Failed to create directory %q: %w", target, err)
			}
		} else {
			err := os.WriteFile(target, nil, 0o644)
			if err != nil {
				return fmt.Errorf("Failed to create file %q: %w", target, err)
			}
		}

		// Move the mount to its destination
		err = unix.Mount(tmpSource, target, "", unix.MS_MOVE, "")
		if err != nil {
			return fmt.Errorf("Failed to mount '%s': %w", mount.Source, err)
		}
	}

	// Cleanup our temporary path
	err := os.RemoveAll(filepath.Join("/", ".distrobuilder"))
	if err != nil {
		return fmt.Errorf("Failed to remove directory %q: %w", filepath.Join("/", ".distrobuilder"), err)
	}

	return nil
}

func killChrootProcesses(rootfs string) error {
	// List all files under /proc
	proc, err := os.Open(filepath.Join(rootfs, "proc"))
	if err != nil {
		return fmt.Errorf("Failed to open file %q: %w", filepath.Join(rootfs, "proc"), err)
	}

	dirs, err := proc.Readdirnames(0)
	if err != nil {
		return fmt.Errorf("Failed to read directory content of %q: %w", filepath.Join(rootfs, "proc"), err)
	}

	// Get all processes and kill them
	re := regexp.MustCompile(`\d+`)

	for _, dir := range dirs {
		if re.MatchString(dir) {
			link, _ := os.Readlink(filepath.Join(rootfs, "proc", dir, "root"))
			if link == rootfs {
				pid, _ := strconv.Atoi(dir)

				err = unix.Kill(pid, unix.SIGKILL)
				if err != nil {
					return fmt.Errorf("Failed killing process: %w", err)
				}
			}
		}
	}

	return nil
}

// SetupChroot sets up mount and files, a reverter and then chroots for you.
func SetupChroot(rootfs string, definition Definition, m []ChrootMount) (func() error, error) {
	// Mount the rootfs
	err := unix.Mount(rootfs, rootfs, "", unix.MS_BIND, "")
	if err != nil {
		return nil, fmt.Errorf("Failed to mount '%s': %w", rootfs, err)
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
	if len(m) > 0 {
		err = setupMounts(rootfs, append(mounts, m...))
	} else {
		err = setupMounts(rootfs, mounts)
	}

	if err != nil {
		return nil, fmt.Errorf("Failed to mount filesystems: %w", err)
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
		return nil, fmt.Errorf("Failed to populate /dev: %w", err)
	}

	// Change permission for /dev/shm
	err = unix.Chmod("/dev/shm", 0o1777)
	if err != nil {
		return nil, fmt.Errorf("Failed to chmod /dev/shm: %w", err)
	}

	var env Environment
	envs := definition.Environment

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

	if len(envs.EnvVariables) > 0 {
		imageTargets := ImageTargetUndefined | ImageTargetAll

		switch definition.Targets.Type {
		case DefinitionFilterTypeContainer:
			imageTargets |= ImageTargetContainer
		case DefinitionFilterTypeVM:
			imageTargets |= ImageTargetVM
		}

		for _, e := range envs.EnvVariables {
			if !ApplyFilter(&e, definition.Image.Release, definition.Image.ArchitectureMapped, definition.Image.Variant, definition.Targets.Type, imageTargets) {
				continue
			}

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
	if incus.PathExists("/usr/sbin/") && !incus.PathExists("/usr/sbin/policy-rc.d") {
		err = os.WriteFile("/usr/sbin/policy-rc.d", []byte(`#!/bin/sh
exit 101
`), 0o755)
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
				return fmt.Errorf("Failed to remove %q: %w", "/usr/sbin/policy-rc.d", err)
			}
		}

		// Reset old environment variables
		SetEnvVariables(oldEnv)

		// Switch back to the host rootfs
		err = root.Chdir()
		if err != nil {
			return fmt.Errorf("Failed to chdir: %w", err)
		}

		err = unix.Chroot(".")
		if err != nil {
			return fmt.Errorf("Failed to chroot: %w", err)
		}

		err = unix.Chdir(cwd)
		if err != nil {
			return fmt.Errorf("Failed to chdir: %w", err)
		}

		// This will kill all processes in the chroot and allow to cleanly
		// unmount everything.
		err = killChrootProcesses(rootfs)
		if err != nil {
			return fmt.Errorf("Failed killing chroot processes: %w", err)
		}

		// And now unmount the entire tree
		err = unix.Unmount(rootfs, unix.MNT_DETACH)
		if err != nil {
			return fmt.Errorf("Failed unmounting rootfs: %w", err)
		}

		devPath := filepath.Join(rootfs, "dev")

		// Wipe $rootfs/dev
		err := os.RemoveAll(devPath)
		if err != nil {
			return fmt.Errorf("Failed to remove directory %q: %w", devPath, err)
		}

		ActiveChroots[rootfs] = nil

		return os.MkdirAll(devPath, 0o755)
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
		{"/dev/console", 5, 1, unix.S_IFCHR | 0o640},
		{"/dev/full", 1, 7, unix.S_IFCHR | 0o666},
		{"/dev/null", 1, 3, unix.S_IFCHR | 0o666},
		{"/dev/random", 1, 8, unix.S_IFCHR | 0o666},
		{"/dev/tty", 5, 0, unix.S_IFCHR | 0o666},
		{"/dev/urandom", 1, 9, unix.S_IFCHR | 0o666},
		{"/dev/zero", 1, 5, unix.S_IFCHR | 0o666},
	}

	for _, d := range devs {
		if incus.PathExists(d.Path) {
			continue
		}

		dev := unix.Mkdev(d.Major, d.Minor)

		err := unix.Mknod(d.Path, d.Mode, int(dev))
		if err != nil {
			return fmt.Errorf("Failed to create %q: %w", d.Path, err)
		}

		// For some odd reason, unix.Mknod will not set the mode correctly.
		// This fixes that.
		err = unix.Chmod(d.Path, d.Mode)
		if err != nil {
			return fmt.Errorf("Failed to chmod %q: %w", d.Path, err)
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
			return fmt.Errorf("Failed to create link %q -> %q: %w", l.Symlink, l.Target, err)
		}
	}

	return nil
}
