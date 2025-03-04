package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// parsePasswdAndGroupFiles reads passwd and group files from the given root directory
// and returns mappings of names to IDs for both users and groups.
func parsePasswdAndGroupFiles(rootDir string) (map[string]string, map[string]string, error) {
	userMap, err := parsePasswdFile(filepath.Join(rootDir, "/etc/passwd"))
	if err != nil {
		return nil, nil, fmt.Errorf("parsing passwd file: %w", err)
	}

	groupMap, err := parsePasswdFile(filepath.Join(rootDir, "/etc/group"))
	if err != nil {
		return nil, nil, fmt.Errorf("parsing group file: %w", err)
	}

	return userMap, groupMap, nil
}

// parsePasswdFile reads a passwd-format file and returns a map of names to IDs.
func parsePasswdFile(path string) (map[string]string, error) {
	idMap := make(map[string]string)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}

	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}

		fields := strings.Split(line, ":")
		if len(fields) < 3 {
			continue
		}

		idMap[fields[0]] = fields[2]
	}

	return idMap, nil
}

// isNumeric is a helper function to check if a string is numeric.
func isNumeric(s string) bool {
	_, err := strconv.Atoi(s)
	return err == nil
}
