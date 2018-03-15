package sources

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/shared"
)

// Debootstrap represents the debootstrap downloader.
type Debootstrap struct{}

// NewDebootstrap creates a new Debootstrap instance.
func NewDebootstrap() *Debootstrap {
	return &Debootstrap{}
}

// Run runs debootstrap.
func (s *Debootstrap) Run(definition shared.Definition, release, arch, rootfsDir string) error {
	var args []string

	os.RemoveAll(rootfsDir)

	if definition.Source.Variant != "" {
		args = append(args, "--variant", definition.Source.Variant)
	}

	if arch != "" {
		args = append(args, "--arch", arch)
	}

	if len(definition.Source.Keys) > 0 {
		keyring, err := shared.CreateGPGKeyring(definition.Source.Keyserver, definition.Source.Keys)
		if err != nil {
			return err
		}
		defer os.RemoveAll(path.Dir(keyring))

		args = append(args, "--keyring", keyring)
	}

	args = append(args, release, rootfsDir)

	if definition.Source.URL != "" {
		args = append(args, definition.Source.URL)
	}

	// If definition.Source.Suite is set, create a symlink in /usr/share/debootstrap/scripts
	// pointing release to definition.Source.Suite.
	if definition.Source.Suite != "" {
		link := filepath.Join("/usr/share/debootstrap/scripts", release)
		err := os.Symlink(definition.Source.Suite, link)
		if err != nil {
			return err
		}
		defer os.Remove(link)
	}

	err := shared.RunCommand("debootstrap", args...)
	if err != nil {
		return err
	}

	if definition.Source.AptSources != "" {
		// Run the template
		out, err := shared.RenderTemplate(definition.Source.AptSources, definition)
		if err != nil {
			return err
		}

		// Append final new line if missing
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}

		// Replace content of sources.list with the templated content.
		file, err := os.Create(filepath.Join(rootfsDir, "etc", "apt", "sources.list"))
		if err != nil {
			return err
		}
		defer file.Close()

		file.WriteString(out)
	}

	return nil
}
