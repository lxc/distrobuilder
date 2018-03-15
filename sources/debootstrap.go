package sources

import (
	"os"
	"path"
	"path/filepath"
	"strings"

	"gopkg.in/flosch/pongo2.v3"

	"github.com/lxc/distrobuilder/shared"
)

// Debootstrap represents the debootstrap downloader.
type Debootstrap struct{}

// NewDebootstrap creates a new Debootstrap instance.
func NewDebootstrap() *Debootstrap {
	return &Debootstrap{}
}

// Run runs debootstrap.
func (s *Debootstrap) Run(source shared.DefinitionSource, release, arch, rootfsDir string) error {
	var args []string

	os.RemoveAll(rootfsDir)

	if source.Variant != "" {
		args = append(args, "--variant", source.Variant)
	}

	if arch != "" {
		args = append(args, "--arch", arch)
	}

	if len(source.Keys) > 0 {
		keyring, err := shared.CreateGPGKeyring(source.Keyserver, source.Keys)
		if err != nil {
			return err
		}
		defer os.RemoveAll(path.Dir(keyring))

		args = append(args, "--keyring", keyring)
	}

	args = append(args, release, rootfsDir)

	if source.URL != "" {
		args = append(args, source.URL)
	}

	// If source.Suite is set, create a symlink in /usr/share/debootstrap/scripts
	// pointing release to source.Suite.
	if source.Suite != "" {
		link := filepath.Join("/usr/share/debootstrap/scripts", release)
		err := os.Symlink(source.Suite, link)
		if err != nil {
			return err
		}
		defer os.Remove(link)
	}

	err := shared.RunCommand("debootstrap", args...)
	if err != nil {
		return err
	}

	if source.AptSources != "" {
		ctx := pongo2.Context{
			"source": source,
			// We use an anonymous struct instead of DefinitionImage because we
			// need the mapped architecture, and Release is all one
			// needs in the sources.list.
			"image": struct {
				Release string
			}{
				release,
			},
		}

		out, err := shared.RenderTemplate(source.AptSources, ctx)
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
