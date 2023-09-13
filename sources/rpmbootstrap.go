package sources

import (
	"fmt"
	"os"
	"path"

	"github.com/lxc/distrobuilder/shared"
)

type rpmbootstrap struct {
	common
}

func (s *rpmbootstrap) repodirs() (dir string, err error) {
	// check whether yum command exists
	if err = shared.RunCommand(s.ctx, nil, nil, "yum", "--version"); err != nil {
		err = fmt.Errorf("yum command not found, sudo apt-get install yum to install it and try again: %w", err)
		return
	}

	reposdir := path.Join(s.sourcesDir, "etc", "yum.repos.d")
	err = os.MkdirAll(reposdir, 0755)
	if err != nil {
		return "", err
	}

	distribution := s.definition.Image.Distribution
	content := s.definition.Source.URL
	if distribution == "" || content == "" {
		err = fmt.Errorf("No valid distribution and source url specified")
		return "", err
	}

	err = os.WriteFile(path.Join(reposdir, distribution+".repo"), []byte(content), 0644)
	if err != nil {
		return "", err
	}

	return reposdir, nil
}

// Run runs yum --installroot.
func (s *rpmbootstrap) Run() error {
	repodir, err := s.repodirs()
	if err != nil {
		return err
	}

	release := s.definition.Image.Release
	args := []string{fmt.Sprintf("--installroot=%s", s.rootfsDir),
		fmt.Sprintf("--releasever=%s", release),
		fmt.Sprintf("--setopt=reposdir=%s", repodir),
		"install", "-y"}

	os.RemoveAll(s.rootfsDir)
	earlyPackagesRemove := s.definition.GetEarlyPackages("remove")

	for _, pkg := range earlyPackagesRemove {
		args = append(args, fmt.Sprintf("--exclude=%s", pkg))
	}

	pkgs := []string{"yum", "dnf"}
	components := s.definition.Source.Components

	for _, pkg := range components {
		pkg, err = shared.RenderTemplate(pkg, s.definition)
		if err != nil {
			return err
		}

		pkgs = append(pkgs, pkg)
	}

	earlyPackagesInstall := s.definition.GetEarlyPackages("install")
	pkgs = append(pkgs, earlyPackagesInstall...)
	args = append(args, pkgs...)

	// Install
	if err = shared.RunCommand(s.ctx, nil, nil, "yum", args...); err != nil {
		return err
	}

	return nil
}
