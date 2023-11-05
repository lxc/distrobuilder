package managers

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	incus "github.com/lxc/incus/shared/util"

	"github.com/lxc/distrobuilder/shared"
)

type yum struct {
	common
}

func (m *yum) load() error {
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
		global: []string{
			"-y",
		},
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

	var buf bytes.Buffer

	err := shared.RunCommand(m.ctx, nil, &buf, "yum", "--help")
	if err != nil {
		return fmt.Errorf("Failed running yum: %w", err)
	}

	scanner := bufio.NewScanner(&buf)

	for scanner.Scan() {
		if strings.Contains(scanner.Text(), "--allowerasing") {
			m.flags.global = append(m.flags.global, "--allowerasing")
			continue
		}

		if strings.Contains(scanner.Text(), "--nobest") {
			m.flags.update = append(m.flags.update, "--nobest")
		}
	}

	return nil
}

func (m *yum) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	// Run rpmdb --rebuilddb
	err := shared.RunCommand(m.ctx, nil, nil, "rpmdb", "--rebuilddb")
	if err != nil {
		return fmt.Errorf("failed to run rpmdb --rebuilddb: %w", err)
	}

	return yumManageRepository(repoAction)
}

func yumManageRepository(repoAction shared.DefinitionPackagesRepository) error {
	targetFile := filepath.Join("/etc/yum.repos.d", repoAction.Name)

	if !strings.HasSuffix(targetFile, ".repo") {
		targetFile = fmt.Sprintf("%s.repo", targetFile)
	}

	if !incus.PathExists(filepath.Dir(targetFile)) {
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
