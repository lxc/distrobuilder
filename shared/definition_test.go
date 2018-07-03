package shared

import (
	"log"
	"regexp"
	"testing"

	"github.com/lxc/lxd/shared"
)

func TestSetDefinitionDefaults(t *testing.T) {
	def := Definition{}

	def.SetDefaults()

	uname, _ := shared.Uname()

	if def.Image.Architecture != uname.Machine {
		t.Fatalf("Expected image.arch to be '%s', got '%s'", uname.Machine, def.Image.Architecture)
	}

	if def.Image.Expiry != "30d" {
		t.Fatalf("Expected image.expiry to be '%s', got '%s'", "30d", def.Image.Expiry)
	}
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
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		tt.definition.SetDefaults()
		err := tt.definition.Validate()
		if !tt.shouldFail && err != nil {
			t.Fatalf("Validation failed: %s", err)
		} else if tt.shouldFail {
			if err == nil {
				t.Fatal("Expected failure")
			}
			match, _ := regexp.MatchString(tt.expected, err.Error())
			if !match {
				t.Fatalf("Validation failed: Expected '%s', got '%s'", tt.expected, err.Error())
			}
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
			Downloader:    "debootstrap",
			URL:           "https://ubuntu.com",
			Keys:          []string{"0xCODE"},
			IgnoreRelease: true,
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
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if d.Image.Release != "bionic" {
		t.Fatalf("Expected '%s', got '%s'", "bionic", d.Image.Release)
	}

	err = d.SetValue("actions.0.trigger", "post-files")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if d.Actions[0].Trigger != "post-files" {
		t.Fatalf("Expected '%s', got '%s'", "post-files", d.Actions[0].Trigger)
	}

	// Index out of bounds
	err = d.SetValue("actions.3.trigger", "post-files")
	if err == nil || err.Error() != "Index out of range" {
		t.Fatal("Expected index out of range")
	}

	// Nonsense
	err = d.SetValue("image", "[foo: bar]")
	if err == nil || err.Error() != "Unsupported type 'struct'" {
		t.Fatal("Expected unsupported assignment")
	}

	err = d.SetValue("source.ignore_release", "true")
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if !d.Source.IgnoreRelease {
		t.Fatalf("Expected '%v', got '%v'", true, d.Source.IgnoreRelease)
	}
}
