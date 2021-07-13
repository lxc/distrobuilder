package sources

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type debootstrap struct {
	common
}

// Run runs debootstrap.
func (s *debootstrap) Run() error {
	var args []string

	os.RemoveAll(s.rootfsDir)

	if s.definition.Source.Variant != "" {
		args = append(args, "--variant", s.definition.Source.Variant)
	}

	if s.definition.Image.ArchitectureMapped != "" {
		args = append(args, "--arch", s.definition.Image.ArchitectureMapped)
	}

	if s.definition.Source.SkipVerification {
		args = append(args, "--no-check-gpg")
	}

	earlyPackagesInstall := s.definition.GetEarlyPackages("install")
	earlyPackagesRemove := s.definition.GetEarlyPackages("remove")

	if len(earlyPackagesInstall) > 0 {
		args = append(args, fmt.Sprintf("--include=%s", strings.Join(earlyPackagesInstall, ",")))
	}

	if len(earlyPackagesRemove) > 0 {
		args = append(args, fmt.Sprintf("--exclude=%s", strings.Join(earlyPackagesRemove, ",")))
	}

	if len(s.definition.Source.Keys) > 0 {
		keyring, err := shared.CreateGPGKeyring(s.definition.Source.Keyserver, s.definition.Source.Keys)
		if err != nil {
			return errors.Wrap(err, "Failed to create GPG keyring")
		}
		defer os.RemoveAll(path.Dir(keyring))

		args = append(args, "--keyring", keyring)
	}

	// If source.suite is set, debootstrap will use this instead of
	// image.release as its first positional argument (SUITE). This is important
	// for derivatives which don't have their own sources, e.g. Linux Mint.
	if s.definition.Source.Suite != "" {
		args = append(args, s.definition.Source.Suite, s.rootfsDir)
	} else {
		args = append(args, s.definition.Image.Release, s.rootfsDir)
	}

	if s.definition.Source.URL != "" {
		args = append(args, s.definition.Source.URL)
	}

	// If s.definition.Source.SameAs is set, create a symlink in /usr/share/debootstrap/scripts
	// pointing release to s.definition.Source.SameAs.
	scriptPath := filepath.Join("/usr/share/debootstrap/scripts", s.definition.Image.Release)
	if !lxd.PathExists(scriptPath) && s.definition.Source.SameAs != "" {
		err := os.Symlink(s.definition.Source.SameAs, scriptPath)
		if err != nil {
			return errors.Wrap(err, "Failed to create symlink")
		}

		defer os.Remove(scriptPath)
	}

	err := shared.RunCommand("debootstrap", args...)
	if err != nil {
		return errors.Wrap(err, `Failed to run "debootstrap"`)
	}

	return nil
}
