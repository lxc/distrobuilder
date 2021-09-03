package generators

import (
	"fmt"
	"os"
	"strconv"

	"github.com/lxc/distrobuilder/shared"
)

func updateFileAccess(file *os.File, defFile shared.DefinitionFile) error {
	// Change file mode if needed
	if defFile.Mode != "" {
		mode, err := strconv.ParseUint(defFile.Mode, 8, 64)
		if err != nil {
			return fmt.Errorf("Failed to parse file mode: %w", err)
		}

		err = file.Chmod(os.FileMode(mode))
		if err != nil {
			return fmt.Errorf("Failed to change file mode: %w", err)
		}
	}

	// Change gid if needed
	if defFile.GID != "" {
		gid, err := strconv.Atoi(defFile.GID)
		if err != nil {
			return fmt.Errorf("Failed to parse GID: %w", err)
		}

		err = file.Chown(-1, gid)
		if err != nil {
			return fmt.Errorf("Failed to change GID: %w", err)
		}
	}

	// Change uid if needed
	if defFile.UID != "" {
		uid, err := strconv.Atoi(defFile.UID)
		if err != nil {
			return fmt.Errorf("Failed to parse UID: %w", err)
		}

		err = file.Chown(uid, -1)
		if err != nil {
			return fmt.Errorf("Failed to change UID: %w", err)
		}
	}

	return nil
}
