package sources

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// Debootstrap represents the debootstrap downloader.
type Debootstrap struct{}

// NewDebootstrap creates a new Debootstrap instance.
func NewDebootstrap() *Debootstrap {
	return &Debootstrap{}
}

// Run runs debootstrap.
func (s *Debootstrap) Run(definition shared.Definition, rootfsDir string) error {
	var args []string

	os.RemoveAll(rootfsDir)

	if definition.Source.Variant != "" {
		args = append(args, "--variant", definition.Source.Variant)
	}

	if definition.Image.ArchitectureMapped != "" {
		args = append(args, "--arch", definition.Image.ArchitectureMapped)
	}

	if len(definition.Source.Keys) > 0 {
		keyring, err := shared.CreateGPGKeyring(definition.Source.Keyserver, definition.Source.Keys)
		if err != nil {
			return err
		}
		defer os.RemoveAll(path.Dir(keyring))

		args = append(args, "--keyring", keyring)
	}

	// If source.ignore_release is set, debootstrap will not use image.release
	// but source.suite as the release. This is important for derivatives which
	// don't have own sources, e.g. Linux Mint.
	if definition.Source.IgnoreRelease {
		// If source.suite is set, use it when calling debootstrap
		if definition.Source.Suite != "" {
			args = append(args, definition.Source.Suite, rootfsDir)
		} else {
			args = append(args, definition.Image.Release, rootfsDir)
		}
	} else {
		args = append(args, definition.Image.Release, rootfsDir)
	}

	if definition.Source.URL != "" {
		args = append(args, definition.Source.URL)
	}

	// If definition.Source.SameAs is set, create a symlink in /usr/share/debootstrap/scripts
	// pointing release to definition.Source.Suite.
	scriptPath := filepath.Join("/usr/share/debootstrap/scripts", definition.Image.Release)
	if !lxd.PathExists(scriptPath) && definition.Source.SameAs != "" {
		err := os.Symlink(definition.Source.SameAs, scriptPath)
		if err != nil {
			return err
		}

		defer os.Remove(scriptPath)
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
