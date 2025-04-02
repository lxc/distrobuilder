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

func (s *rpmbootstrap) yumordnf() (cmd string, err error) {
	// check whether yum or dnf command exists
	for _, cmd = range []string{"yum", "dnf"} {
		if err = shared.RunCommand(s.ctx, nil, nil, cmd, "--version"); err == nil {
			return
		}
	}
	cmd = ""
	err = fmt.Errorf("Command yum or dnf not found, sudo apt-get install yum or sudo apt-get install dnf and try again")
	return
}

func (s *rpmbootstrap) repodirs() (dir string, err error) {
	reposdir := path.Join(s.sourcesDir, "etc", "yum.repos.d")
	err = os.MkdirAll(reposdir, 0o755)
	if err != nil {
		return "", err
	}

	distribution := s.definition.Image.Distribution
	content := s.definition.Source.URL
	if distribution == "" || content == "" {
		err = fmt.Errorf("No valid distribution and source url specified")
		return "", err
	}

	err = os.WriteFile(path.Join(reposdir, distribution+".repo"), []byte(content), 0o644)
	if err != nil {
		return "", err
	}

	return reposdir, nil
}

// Run runs yum --installroot.
func (s *rpmbootstrap) Run() (err error) {
	cmd, err := s.yumordnf()
	if err != nil {
		return err
	}

	repodir, err := s.repodirs()
	if err != nil {
		return err
	}

	release := s.definition.Image.Release
	args := []string{
		fmt.Sprintf("--installroot=%s", s.rootfsDir),
		fmt.Sprintf("--releasever=%s", release),
		fmt.Sprintf("--setopt=reposdir=%s", repodir),
		"install", "-y",
	}

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
	if err = shared.RunCommand(s.ctx, nil, nil, cmd, args...); err != nil {
		return err
	}

	return nil
}
