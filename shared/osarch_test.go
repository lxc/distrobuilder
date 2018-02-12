package shared

import (
	"log"
	"testing"
)

func TestGetArch(t *testing.T) {
	tests := []struct {
		distro   string
		arch     string
		expected string
	}{
		{
			"alpinelinux",
			"x86_64",
			"x86_64",
		},
		{
			"centos",
			"x86_64",
			"x86_64",
		},
		{
			"debian",
			"amd64",
			"amd64",
		},
		{
			"debian",
			"x86_64",
			"amd64",
		},
		{
			"debian",
			"s390x",
			"s390x",
		},
		{
			"ubuntu",
			"amd64",
			"amd64",
		},
		{
			"ubuntu",
			"x86_64",
			"amd64",
		},
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s %s", i, tt.distro, tt.arch)
		arch, err := GetArch(tt.distro, tt.arch)
		if err != nil {
			t.Fatalf("Unexpected error: %s", err)
		}
		if arch != tt.expected {
			t.Fatalf("Wrong arch: Expected '%s', got '%s'", tt.expected, arch)
		}
	}

	_, err := GetArch("distro", "")
	if err == nil || err.Error() != "Architecture map isn't supported: distro" {
		t.Fatalf("Expected unsupported architecture map, got '%s'", err)
	}

	_, err = GetArch("debian", "arch")
	if err == nil || err.Error() != "Architecture isn't supported: arch" {
		t.Fatalf("Expected unsupported architecture, got '%s'", err)
	}
}
