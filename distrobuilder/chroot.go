package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
	"go.uber.org/zap"
	"golang.org/x/sys/unix"
)

func getOverlay(logger *zap.SugaredLogger, cacheDir, sourceDir string) (func(), string, error) {
	var stat unix.Statfs_t

	// Skip overlay on xfs
	for _, dir := range []string{cacheDir, sourceDir} {
		err := unix.Statfs(dir, &stat)
		if err != nil {
			return nil, "", err
		}

		if stat.Type == unix.XFS_SUPER_MAGIC {
			return nil, "", errors.Errorf("overlay not supported on xfs")
		}
	}

	upperDir := filepath.Join(cacheDir, "upper")
	overlayDir := filepath.Join(cacheDir, "overlay")
	workDir := filepath.Join(cacheDir, "work")

	err := os.Mkdir(upperDir, 0755)
	if err != nil {
		return nil, "", err
	}

	err = os.Mkdir(overlayDir, 0755)
	if err != nil {
		return nil, "", err
	}

	err = os.Mkdir(workDir, 0755)
	if err != nil {
		return nil, "", err
	}

	opts := fmt.Sprintf("lowerdir=%s,upperdir=%s,workdir=%s", sourceDir, upperDir, workDir)

	err = unix.Mount("overlay", overlayDir, "overlay", 0, opts)
	if err != nil {
		return nil, "", err
	}

	cleanup := func() {
		unix.Sync()

		err := unix.Unmount(overlayDir, 0)
		if err != nil {
			logger.Warnw("Failed to unmount overlay", "err", err, "dir", overlayDir)
		}

		err = os.RemoveAll(upperDir)
		if err != nil {
			logger.Warnw("Failed to remove upper directory", "err", err, "dir", upperDir)

		}

		err = os.RemoveAll(workDir)
		if err != nil {
			logger.Warnw("Failed to remove work directory", "err", err, "dir", workDir)
		}

		err = os.Remove(overlayDir)
		if err != nil {
			logger.Warnw("Failed to remove overlay directory", "err", err, "dir", overlayDir)
		}
	}

	return cleanup, overlayDir, nil
}
