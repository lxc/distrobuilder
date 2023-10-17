package sources

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenWrtHTTP_getLatestServiceRelease(t *testing.T) {
	s := &openwrt{}

	tests := []struct {
		release string
		want    *regexp.Regexp
	}{
		{
			"21.02",
			regexp.MustCompile(`21\.02\.\d+`),
		},
		{
			"22.03",
			regexp.MustCompile(`22\.03\.\d+`),
		},
		{
			"23.05",
			regexp.MustCompile(`23\.05\.\d+`),
		},
	}

	for _, tt := range tests {
		baseURL := "https://downloads.openwrt.org/releases/"
		release, err := s.getLatestServiceRelease(baseURL, tt.release)
		require.NoError(t, err)
		require.Regexp(t, tt.want, release)
	}
}
