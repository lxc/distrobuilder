package generators

import (
	"os"
	"strconv"

	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

func updateFileAccess(file *os.File, defFile shared.DefinitionFile) error {
	// Change file mode if needed
	if defFile.Mode != "" {
		mode, err := strconv.ParseUint(defFile.Mode, 8, 64)
		if err != nil {
			return errors.Wrap(err, "Failed to parse file mode")
		}

		err = file.Chmod(os.FileMode(mode))
		if err != nil {
			return errors.Wrap(err, "Failed to change file mode")
		}
	}

	// Change gid if needed
	if defFile.GID != "" {
		gid, err := strconv.Atoi(defFile.GID)
		if err != nil {
			return errors.Wrap(err, "Failed to parse GID")
		}

		err = file.Chown(-1, gid)
		if err != nil {
			return errors.Wrap(err, "Failed to change GID")
		}
	}

	// Change uid if needed
	if defFile.Mode != "" {
		uid, err := strconv.Atoi(defFile.UID)
		if err != nil {
			return errors.Wrap(err, "Failed to parse UID")
		}

		err = file.Chown(uid, -1)
		if err != nil {
			return errors.Wrap(err, "Failed to change UID")
		}
	}

	return nil
}
