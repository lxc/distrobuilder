package sources

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/lxc/distrobuilder/v3/shared"
)

type archbootstrap struct {
	common
}

// bootstrapConfig writes a standalone pacman.conf and mirrorlist under
// sourcesDir, so the host configuration is never touched. It returns the
// path to the generated pacman.conf.
//
// Only the base Arch repositories are configured here. Derivative
// repositories (e.g. CachyOS's optimised ones) are added later, by the
// definition's post-unpack actions, exactly as with the bootstrap tarball.
func (s *archbootstrap) bootstrapConfig() (string, error) {
	if s.definition.Source.URL == "" {
		return "", fmt.Errorf("No source url specified")
	}

	confDir := filepath.Join(s.sourcesDir, "etc")
	err := os.MkdirAll(confDir, 0o755)
	if err != nil {
		return "", err
	}

	mirrorlist := path.Join(confDir, "mirrorlist")
	err = os.WriteFile(mirrorlist, []byte("Server = "+s.definition.Source.URL+"\n"), 0o644)
	if err != nil {
		return "", err
	}

	// SigLevel = Never when verification is skipped, e.g. while bringing
	// up a derivative whose keyring cannot be populated up front.
	// Packages are always verified; database signatures are optional
	// because not every mirror publishes them. This mirrors Arch's own
	// default (Required DatabaseOptional).
	sigLevel := "Required DatabaseOptional"
	if s.definition.Source.SkipVerification {
		sigLevel = "Never"
	}

	var conf strings.Builder
	fmt.Fprintf(&conf, "[options]\n")
	fmt.Fprintf(&conf, "Architecture = %s\n", s.definition.Image.ArchitectureMapped)
	fmt.Fprintf(&conf, "SigLevel = %s\n\n", sigLevel)
	fmt.Fprintf(&conf, "[core]\nInclude = %s\n\n", mirrorlist)
	fmt.Fprintf(&conf, "[extra]\nInclude = %s\n\n", mirrorlist)

	confPath := path.Join(confDir, "pacman.conf")
	err = os.WriteFile(confPath, []byte(conf.String()), 0o644)
	if err != nil {
		return "", err
	}

	return confPath, nil
}

// pacmanArgs returns the common argument set that points pacman at the
// target rootfs instead of the host.
func (s *archbootstrap) pacmanArgs(confPath string) []string {
	return []string{
		fmt.Sprintf("--root=%s", s.rootfsDir),
		fmt.Sprintf("--config=%s", confPath),
		fmt.Sprintf("--dbpath=%s", path.Join(s.rootfsDir, "var", "lib", "pacman")),
		fmt.Sprintf("--cachedir=%s", path.Join(s.rootfsDir, "var", "cache", "pacman", "pkg")),
	}
}

// populateKeyring initialises and populates the target rootfs keyring so
// that package signatures can be verified. It is skipped when the caller
// opted out of verification.
func (s *archbootstrap) populateKeyring() error {
	if s.definition.Source.SkipVerification {
		return nil
	}

	keyringDir := path.Join(s.rootfsDir, "etc", "pacman.d", "gnupg")

	err := os.MkdirAll(keyringDir, 0o755)
	if err != nil {
		return err
	}

	err = shared.RunCommand(s.ctx, nil, nil, "pacman-key", "--gpgdir", keyringDir, "--init")
	if err != nil {
		return fmt.Errorf("Failed to init pacman keyring: %w", err)
	}

	err = shared.RunCommand(s.ctx, nil, nil, "pacman-key", "--gpgdir", keyringDir,
		"--populate", "archlinux")
	if err != nil {
		return fmt.Errorf("Failed to populate pacman keyring: %w", err)
	}

	return nil
}

// Run runs pacman --root.
func (s *archbootstrap) Run() error {
	confPath, err := s.bootstrapConfig()
	if err != nil {
		return err
	}

	os.RemoveAll(s.rootfsDir)

	args := s.pacmanArgs(confPath)

	// pacman requires the database and cache directories to exist before
	// it will operate on a fresh root.
	for _, dir := range []string{
		path.Join(s.rootfsDir, "var", "lib", "pacman"),
		path.Join(s.rootfsDir, "var", "cache", "pacman", "pkg"),
	} {
		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
	}

	err = s.populateKeyring()
	if err != nil {
		return err
	}

	// Sync the package databases.
	err = shared.RunCommand(s.ctx, nil, nil, "pacman", append(args, "-Sy", "--noconfirm")...)
	if err != nil {
		return fmt.Errorf("Failed to sync databases: %w", err)
	}

	// The base package manager plus any packages named in
	// source.components, mirroring rpmbootstrap.
	pkgs := []string{"pacman"}

	for _, pkg := range s.definition.Source.Components {
		pkg, err = shared.RenderTemplate(pkg, s.definition)
		if err != nil {
			return err
		}

		pkgs = append(pkgs, pkg)
	}

	pkgs = append(pkgs, s.definition.GetEarlyPackages("install")...)

	installArgs := append(args, "-S", "--noconfirm", "--needed")

	for _, pkg := range s.definition.GetEarlyPackages("remove") {
		installArgs = append(installArgs, fmt.Sprintf("--ignore=%s", pkg))
	}

	installArgs = append(installArgs, pkgs...)

	err = shared.RunCommand(s.ctx, nil, nil, "pacman", installArgs...)
	if err != nil {
		return fmt.Errorf("Failed to install packages: %w", err)
	}

	return nil
}
