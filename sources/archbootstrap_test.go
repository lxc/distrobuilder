package sources

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/lxc/distrobuilder/v3/shared"
)

func TestArchbootstrapBootstrapConfig(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		arch       string
		skipVerify bool
		shouldFail bool
	}{
		{
			"https mirror",
			"https://mirror.example.com/archlinux/$repo/os/$arch",
			"x86_64",
			false,
			false,
		},
		{
			"verification skipped",
			"https://mirror.example.com/archlinux/$repo/os/$arch",
			"x86_64",
			true,
			false,
		},
		{
			"no url",
			"",
			"x86_64",
			false,
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()

			src := &archbootstrap{
				common: common{
					sourcesDir: dir,
					rootfsDir:  filepath.Join(dir, "rootfs"),
					definition: shared.Definition{
						Image: shared.DefinitionImage{
							ArchitectureMapped: tt.arch,
						},
						Source: shared.DefinitionSource{
							URL:              tt.url,
							SkipVerification: tt.skipVerify,
						},
					},
				},
			}

			confPath, err := src.bootstrapConfig()
			if tt.shouldFail {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.FileExists(t, confPath)

			content, err := os.ReadFile(confPath)
			require.NoError(t, err)
			conf := string(content)

			// The mirrorlist must be written and referenced.
			mirrorlist := filepath.Join(dir, "etc", "mirrorlist")
			require.FileExists(t, mirrorlist)

			mirrorContent, err := os.ReadFile(mirrorlist)
			require.NoError(t, err)
			require.Equal(t, "Server = "+tt.url+"\n", string(mirrorContent))

			require.Contains(t, conf, "[options]")
			require.Contains(t, conf, "Architecture = "+tt.arch)
			require.Contains(t, conf, "[core]")
			require.Contains(t, conf, "[extra]")
			require.Contains(t, conf, "Include = "+mirrorlist)

			// Verification is always on unless explicitly skipped, and
			// the database signature is optional so mirrors that do not
			// publish core.db.sig still work.
			if tt.skipVerify {
				require.Contains(t, conf, "SigLevel = Never")
			} else {
				require.Contains(t, conf, "SigLevel = Required DatabaseOptional")
			}

			// GPGDir must point inside the target rootfs, so that package
			// signature verification uses the keyring populated there
			// instead of the host keyring. Without this, pacman falls back
			// to /etc/pacman.d/gnupg on the host, which may be absent (e.g.
			// on Debian) or stale.
			require.Contains(t, conf, "GPGDir = "+
				filepath.Join(dir, "rootfs", "etc", "pacman.d", "gnupg"))
		})
	}
}
