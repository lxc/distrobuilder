package managers

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type apt struct {
	common
}

func (m *apt) load() error {
	m.commands = managerCommands{
		clean:   "apt-get",
		install: "apt-get",
		refresh: "apt-get",
		remove:  "apt-get",
		update:  "apt-get",
	}

	m.flags = managerFlags{
		clean: []string{
			"clean",
		},
		global: []string{
			"-y",
		},
		install: []string{
			"install",
		},
		remove: []string{
			"remove", "--auto-remove",
		},
		refresh: []string{
			"update",
		},
		update: []string{
			"dist-upgrade",
		},
	}

	return nil
}

func (m *apt) manageRepository(repoAction shared.DefinitionPackagesRepository) error {
	var targetFile string

	if repoAction.Name == "sources.list" {
		targetFile = filepath.Join("/etc/apt", repoAction.Name)
	} else {
		targetFile = filepath.Join("/etc/apt/sources.list.d", repoAction.Name)

		if !strings.HasSuffix(targetFile, ".list") {
			targetFile = fmt.Sprintf("%s.list", targetFile)
		}

	}

	if !lxd.PathExists(filepath.Dir(targetFile)) {
		err := os.MkdirAll(filepath.Dir(targetFile), 0755)
		if err != nil {
			return errors.Wrapf(err, "Failed to create directory %q", filepath.Dir(targetFile))
		}
	}

	f, err := os.OpenFile(targetFile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return errors.Wrapf(err, "Failed to open file %q", targetFile)
	}
	defer f.Close()

	content, err := ioutil.ReadAll(f)
	if err != nil {
		return errors.Wrapf(err, "Failed to read from file %q", targetFile)
	}

	// Truncate file if it's not generated by distrobuilder
	if !strings.HasPrefix(string(content), "# Generated by distrobuilder\n") {
		err = f.Truncate(0)
		if err != nil {
			return errors.Wrapf(err, "Failed to truncate %q", targetFile)
		}

		_, err = f.Seek(0, 0)
		if err != nil {
			return errors.Wrapf(err, "Failed to seek on file %q", targetFile)
		}

		_, err = f.WriteString("# Generated by distrobuilder\n")
		if err != nil {
			return errors.Wrapf(err, "Failed to write to file %q", targetFile)
		}
	}

	_, err = f.WriteString(repoAction.URL)
	if err != nil {
		return errors.Wrapf(err, "Failed to write to file %q", targetFile)
	}

	// Append final new line if missing
	if !strings.HasSuffix(repoAction.URL, "\n") {
		_, err = f.WriteString("\n")
		if err != nil {
			return errors.Wrapf(err, "Failed to write to file %q", targetFile)
		}
	}

	if repoAction.Key != "" {
		var reader io.Reader

		if strings.HasPrefix(repoAction.Key, "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
			reader = strings.NewReader(repoAction.Key)
		} else {
			// If only key ID is provided, we need gpg to be installed early.
			_, err := lxd.RunCommand("gpg", "--recv-keys", repoAction.Key)
			if err != nil {
				return errors.Wrap(err, "Failed to receive GPG keys")
			}

			var buf bytes.Buffer

			err = lxd.RunCommandWithFds(nil, &buf, "gpg", "--export", "--armor", repoAction.Key)
			if err != nil {
				return errors.Wrap(err, "Failed to export GPG keys")
			}

			reader = &buf
		}

		signatureFilePath := filepath.Join("/etc/apt/trusted.gpg.d", fmt.Sprintf("%s.asc", repoAction.Name))

		f, err := os.Create(signatureFilePath)
		if err != nil {
			return errors.Wrapf(err, "Failed to create file %q", signatureFilePath)
		}
		defer f.Close()

		_, err = io.Copy(f, reader)
		if err != nil {
			return errors.Wrap(err, "Failed to copy file")
		}
	}

	return nil
}
