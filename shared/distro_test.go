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
			"amd64",
			"x86_64",
		},
		{
			"alpinelinux",
			"x86_64",
			"x86_64",
		},
		{
			"archlinux",
			"amd64",
			"x86_64",
		},
		{
			"archlinux",
			"x86_64",
			"x86_64",
		},
		{
			"centos",
			"amd64",
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
		arch := GetArch(tt.distro, tt.arch)
		if arch != tt.expected {
			t.Fatalf("Wrong arch: Expected '%s', got '%s'", tt.expected, arch)
		}
	}
}
