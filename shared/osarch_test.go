package shared

import (
	"log"
	"testing"

	"github.com/stretchr/testify/require"
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
	}

	for i, tt := range tests {
		log.Printf("Running test #%d: %s %s", i, tt.distro, tt.arch)
		arch, err := GetArch(tt.distro, tt.arch)
		require.NoError(t, err)
		require.Equal(t, tt.expected, arch)
	}

	_, err := GetArch("distro", "")
	require.EqualError(t, err, "Architecture map isn't supported: distro")

	_, err = GetArch("debian", "arch")
	require.EqualError(t, err, "Architecture isn't supported: arch")
}
