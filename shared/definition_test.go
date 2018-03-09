package shared

import (
	"log"
	"regexp"
	"runtime"
	"testing"
)

func TestSetDefinitionDefaults(t *testing.T) {
	def := Definition{}

	SetDefinitionDefaults(&def)

	if def.Image.Arch != runtime.GOARCH {
		t.Fatalf("Expected image.arch to be '%s', got '%s'", runtime.GOARCH, def.Image.Arch)
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
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s", i, tt.name)
		err := ValidateDefinition(tt.definition)
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
