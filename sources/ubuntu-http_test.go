package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUbuntuGetLatestCoreBaseImage(t *testing.T) {
	release, err := getLatestCoreBaseImage("https://images.linuxcontainers.org/images", "xenial", "amd64")
	require.NoError(t, err)
	require.NotEmpty(t, release)
}
