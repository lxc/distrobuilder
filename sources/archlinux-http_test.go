package sources

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchLinuxGetLatestRelease(t *testing.T) {
	var src ArchLinuxHTTP

	release, err := src.getLatestRelease()
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`), release)
}
