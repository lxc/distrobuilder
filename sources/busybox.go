package sources

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type busybox struct {
	common
}

// Run downloads a busybox tarball.
func (s *busybox) Run() error {
	fname := fmt.Sprintf("busybox-%s.tar.bz2", s.definition.Image.Release)
	tarball := fmt.Sprintf("%s/%s", s.definition.Source.URL, fname)

	var (
		fpath string
		err   error
	)

	if s.definition.Source.SkipVerification {
		fpath, err = shared.DownloadHash(s.definition.Image, tarball, "", nil)

	} else {
		fpath, err = shared.DownloadHash(s.definition.Image, tarball, tarball+".sha256", sha256.New())
	}
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", tarball)
	}

	s.logger.Debugw("Run", "fpath", fpath)

	tempRootDir := filepath.Join(s.cacheDir, "rootfs")

	err = os.MkdirAll(tempRootDir, 0755)
	if err != nil {
		return errors.Wrapf(err, "Failed to create directory %q", tempRootDir)
	}

	// Unpack
	err = lxd.Unpack(filepath.Join(fpath, fname), tempRootDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", fname)
	}

	tempRootDir = filepath.Join(tempRootDir, fmt.Sprintf("busybox-%s", s.definition.Image.Release))

	err = shared.RunScript(fmt.Sprintf(`#!/bin/sh
set -eux

tmp_rootfs_dir=%s
rootfs_dir=%s

cwd="$(pwd)"

cd "${tmp_rootfs_dir}"
make defconfig
sed -ri 's/# CONFIG_STATIC .*/CONFIG_STATIC=y/g' .config
make

cd "${cwd}"
mkdir -p "${rootfs_dir}/bin"
mv ${tmp_rootfs_dir}/busybox "${rootfs_dir}/bin/busybox"
`, tempRootDir, s.rootfsDir))
	if err != nil {
		return errors.Wrap(err, "Failed to build busybox")
	}

	var buf bytes.Buffer

	err = lxd.RunCommandWithFds(os.Stdin, &buf, filepath.Join(s.rootfsDir, "bin", "busybox"), "--list-full")
	if err != nil {
		return errors.Wrap(err, "Failed to install busybox")
	}

	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		path := filepath.Join(s.rootfsDir, scanner.Text())

		if path == "" || path == "bin/busybox" {
			continue
		}

		s.logger.Debugf("Creating directory %q", path)

		err = os.MkdirAll(filepath.Dir(path), 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", filepath.Dir(path))
		}

		s.logger.Debugf("Creating symlink %q -> %q", path, "/bin/busybox")

		err = os.Symlink("/bin/busybox", path)
		if err != nil {
			return errors.Wrapf(err, "Failed to create symlink %q -> /bin/busybox", path)
		}
	}

	for _, path := range []string{"dev", "mnt", "proc", "root", "sys", "tmp"} {
		err := os.Mkdir(filepath.Join(s.rootfsDir, path), 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", filepath.Join(s.rootfsDir, path))
		}
	}

	return nil
}
