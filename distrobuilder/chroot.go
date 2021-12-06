package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

func getOverlay(logger *logrus.Logger, cacheDir, sourceDir string) (func(), string, error) {
	var stat unix.Statfs_t

	// Skip overlay on xfs and zfs
	for _, dir := range []string{cacheDir, sourceDir} {
		err := unix.Statfs(dir, &stat)
		if err != nil {
			return nil, "", err
		}

		switch stat.Type {
		case unix.XFS_SUPER_MAGIC:
			return nil, "", errors.New("overlay not supported on xfs")
		case 0x2fc12fc1:
			return nil, "", errors.New("overlay not supported on zfs")
		}
	}

	upperDir := filepath.Join(cacheDir, "upper")
	overlayDir := filepath.Join(cacheDir, "overlay")
	workDir := filepath.Join(cacheDir, "work")

	err := os.Mkdir(upperDir, 0755)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create directory %q: %w", upperDir, err)
	}

	err = os.Mkdir(overlayDir, 0755)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create directory %q: %w", overlayDir, err)
	}

	err = os.Mkdir(workDir, 0755)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to create directory %q: %w", workDir, err)
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", sourceDir, upperDir, workDir)

	err = unix.Mount("overlay", overlayDir, "overlay", 0, opts)
	if err != nil {
		return nil, "", fmt.Errorf("Failed to mount overlay: %w", err)
	}

	cleanup := func() {
		unix.Sync()

		err := unix.Unmount(overlayDir, 0)
		if err != nil {
			logger.WithFields(logrus.Fields{"err": err, "dir": overlayDir}).Warn("Failed to unmount overlay directory")
		}

		err = os.RemoveAll(upperDir)
		if err != nil {
			logger.WithFields(logrus.Fields{"err": err, "dir": upperDir}).Warn("Failed to remove upper directory")

		}

		err = os.RemoveAll(workDir)
		if err != nil {
			logger.WithFields(logrus.Fields{"err": err, "dir": workDir}).Warn("Failed to remove work directory")

		}

		err = os.Remove(overlayDir)
		if err != nil {
			logger.WithFields(logrus.Fields{"err": err, "dir": overlayDir}).Warn("Failed to remove overlay directory")

		}
	}

	return cleanup, overlayDir, nil
}
