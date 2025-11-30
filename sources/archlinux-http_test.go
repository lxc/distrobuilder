package sources

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestArchLinuxGetLatestRelease(t *testing.T) {
	src := &archlinux{}
	src.client = http.DefaultClient

	release, err := src.getLatestRelease("https://archive.archlinux.org/iso/", "x86_64")
	require.NoError(t, err)
	require.Regexp(t, regexp.MustCompile(`^\d{4}\.\d{2}\.\d{2}$`), release)
}
