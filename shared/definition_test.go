package shared

import (
	"log"
	"testing"

	"github.com/lxc/lxd/shared"
	"github.com/stretchr/testify/require"
	yaml "gopkg.in/yaml.v2"
)

func TestSetDefinitionDefaults(t *testing.T) {
	def := Definition{}

	def.SetDefaults()

	uname, _ := shared.Uname()

	require.Equal(t, uname.Machine, def.Image.Architecture)
	require.Equal(t, "30d", def.Image.Expiry)
}

func TestValidateDefinition(t *testing.T) {
	tests := []struct {
		name       string
		definition Definition
		expected   string
		shouldFail bool
	}{
		{
			"valid Definition",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager: "apt",
				},
				Files: []DefinitionFile{
					{
						Generator: "dump",
					},
				},
				Mappings: DefinitionMappings{
					ArchitectureMap: "debian",
				},
			},
			"",
			false,
		},
		{
			"valid Definition without source.keys",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
				},
				Packages: DefinitionPackages{
					Manager: "apt",
				},
			},
			"",
			false,
		},
		{
			"valid Defintion without source.url",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
				},
				Packages: DefinitionPackages{
					Manager: "apt",
				},
			},
			"",
			false,
		},
		{
			"valid Defintion with packages.custom-manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
				},
				Packages: DefinitionPackages{
					CustomManager: &DefinitionPackagesCustomManager{
						Install: CustomManagerCmd{
							Command: "install",
						},
						Remove: CustomManagerCmd{
							Command: "remove",
						},
						Clean: CustomManagerCmd{
							Command: "clean",
						},
						Update: CustomManagerCmd{
							Command: "update",
						},
						Refresh: CustomManagerCmd{
							Command: "refresh",
						},
					},
				},
			},
			"",
			false,
		},
		{
			"invalid ArchitectureMap",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager: "apt",
				},
				Files: []DefinitionFile{
					{
						Generator: "dump",
					},
				},
				Mappings: DefinitionMappings{
					ArchitectureMap: "foo",
				},
			},
			"mappings.architecture_map must be one of .+",
			true,
		},
		{
			"invalid generator",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager: "apt",
				},
				Files: []DefinitionFile{
					{
						Generator: "foo",
					},
				},
			},
			"files\\.\\*\\.generator must be one of .+",
			true,
		},
		{
			"empty image.distribution",
			Definition{},
			"image.distribution may not be empty",
			true,
		},
		{
			"invalid source.downloader",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "foo",
				},
			},
			"source.downloader must be one of .+",
			true,
		},
		{
			"invalid package.manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager: "foo",
				},
			},
			"packages.manager must be one of .+",
			true,
		},
		{
			"missing clean command in packages.custom-manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					CustomManager: &DefinitionPackagesCustomManager{},
				},
			},
			"packages.custom-manager requires a clean command",
			true,
		},
		{
			"missing install command in packages.custom-manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					CustomManager: &DefinitionPackagesCustomManager{
						Clean: CustomManagerCmd{
							Command: "clean",
						},
					},
				},
			},
			"packages.custom-manager requires an install command",
			true,
		},
		{
			"missing remove command in packages.custom-manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					CustomManager: &DefinitionPackagesCustomManager{
						Clean: CustomManagerCmd{
							Command: "clean",
						},
						Install: CustomManagerCmd{
							Command: "install",
						},
					},
				},
			},
			"packages.custom-manager requires a remove command",
			true,
		},
		{
			"missing refresh command in packages.custom-manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					CustomManager: &DefinitionPackagesCustomManager{
						Clean: CustomManagerCmd{
							Command: "clean",
						},
						Install: CustomManagerCmd{
							Command: "install",
						},
						Remove: CustomManagerCmd{
							Command: "remove",
						},
					},
				},
			},
			"packages.custom-manager requires a refresh command",
			true,
		},
		{
			"missing update command in packages.custom-manager",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					CustomManager: &DefinitionPackagesCustomManager{
						Clean: CustomManagerCmd{
							Command: "clean",
						},
						Install: CustomManagerCmd{
							Command: "install",
						},
						Remove: CustomManagerCmd{
							Command: "remove",
						},
						Refresh: CustomManagerCmd{
							Command: "refresh",
						},
					},
				},
			},
			"packages.custom-manager requires an update command",
			true,
		},
		{
			"package.manager and package.custom-manager set",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager:       "apt",
					CustomManager: &DefinitionPackagesCustomManager{},
				},
			},
			"cannot have both packages.manager and packages.custom-manager set",
			true,
		},
		{
			"package.manager and package.custom-manager unset",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{},
			},
			"packages.manager or packages.custom-manager needs to be set",
			true,
		},
		{
			"invalid action trigger",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager: "apt",
				},
				Actions: []DefinitionAction{
					{
						Trigger: "post-build",
					},
				},
			},
			"actions\\.\\*\\.trigger must be one of .+",
			true,
		},
		{
			"invalid package action",
			Definition{
				Image: DefinitionImage{
					Distribution: "ubuntu",
					Release:      "artful",
				},
				Source: DefinitionSource{
					Downloader: "debootstrap",
					URL:        "https://ubuntu.com",
					Keys:       []string{"0xCODE"},
				},
				Packages: DefinitionPackages{
					Manager: "apt",
					Sets: []DefinitionPackagesSet{
						{
							Action: "update",
						},
					},
				},
			},
			"packages\\.\\*\\.set\\.\\*\\.action must be one of .+",
			true,
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		tt.definition.SetDefaults()
		err := tt.definition.Validate()
		if tt.shouldFail {
			require.Regexp(t, tt.expected, err)
		} else {
			require.NoError(t, err)
		}
	}
}

func TestDefinitionSetValue(t *testing.T) {
	d := Definition{
		Image: DefinitionImage{
			Distribution: "ubuntu",
			Release:      "artful",
		},
		Source: DefinitionSource{
			Downloader:       "debootstrap",
			URL:              "https://ubuntu.com",
			Keys:             []string{"0xCODE"},
			SkipVerification: true,
		},
		Packages: DefinitionPackages{
			Manager: "apt",
		},
		Actions: []DefinitionAction{
			{
				Trigger: "post-update",
				Action:  "/bin/true",
			},
			{
				Trigger: "post-packages",
				Action:  "/bin/false",
			},
		},
	}

	err := d.SetValue("image.release", "bionic")
	require.NoError(t, err)
	require.Equal(t, "bionic", d.Image.Release)

	err = d.SetValue("actions.0.trigger", "post-files")
	require.NoError(t, err)
	require.Equal(t, "post-files", d.Actions[0].Trigger)

	// Index out of bounds
	err = d.SetValue("actions.3.trigger", "post-files")
	require.EqualError(t, err, "Index out of range")

	// Nonsense
	err = d.SetValue("image", "[foo: bar]")
	require.EqualError(t, err, "Unsupported type 'struct'")

	err = d.SetValue("source.skip_verification", "true")
	require.NoError(t, err)
	require.Equal(t, true, d.Source.SkipVerification)
}

func TestDefinitionFilter(t *testing.T) {
	input := `packages:
  sets:
  - packages:
    - foo
    architectures:
    - amd64`
	def := Definition{}

	err := yaml.Unmarshal([]byte(input), &def)
	require.NoError(t, err)

	require.Contains(t, def.Packages.Sets[0].Packages, "foo")
	require.Contains(t, def.Packages.Sets[0].Architectures, "amd64")
}
