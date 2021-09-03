package managers

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

type yum struct {
	common
}

func (m *yum) load() error {
	var buf bytes.Buffer
	globalFlags := []string{"-y"}

	lxd.RunCommandWithFds(nil, &buf, "yum", "--help")

	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "--allowerasing") {
			globalFlags = append(globalFlags, "--allowerasing")
			break
		}
	}

	m.commands = managerCommands{
		clean:   "yum",
		install: "yum",
		refresh: "yum",
		remove:  "yum",
		update:  "yum",
	}

	m.flags = managerFlags{
		clean: []string{
			"clean", "all",
		},
		global: globalFlags,
		install: []string{
			"install",
		},
		remove: []string{
			"remove",
		},
		refresh: []string{
			"makecache",
		},
		update: []string{
			"update",
		},
	}

	return nil
}

func (m *yum) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	return yumManageRepository(repoAction)
}

func yumManageRepository(repoAction shared.DefinitionPackagesRepository) error {
	targetFile := filepath.Join("/etc/yum.repos.d", repoAction.Name)

	if !strings.HasSuffix(targetFile, ".repo") {
		targetFile = fmt.Sprintf("%s.repo", targetFile)
	}

	if !lxd.PathExists(filepath.Dir(targetFile)) {
		err := os.MkdirAll(filepath.Dir(targetFile), 0755)
		if err != nil {
			return fmt.Errorf("Failed to create directory %q: %w", filepath.Dir(targetFile), err)
		}
	}

	f, err := os.Create(targetFile)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", targetFile, err)
	}
	defer f.Close()

	_, err = f.WriteString(repoAction.URL)
	if err != nil {
		return fmt.Errorf("Failed to write to file %q: %w", targetFile, err)
	}

	// Append final new line if missing
	if !strings.HasSuffix(repoAction.URL, "\n") {
		_, err = f.WriteString("\n")
		if err != nil {
			return fmt.Errorf("Failed to write to file %q: %w", targetFile, err)
		}
	}

	return nil
}
