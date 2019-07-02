package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUbuntuGetLatestCoreBaseImage(t *testing.T) {
	release := getLatestCoreBaseImage("https://images.linuxcontainers.org/images", "xenial", "amd64")
	require.NotEmpty(t, release)
}
