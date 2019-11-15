package sources

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenWrtHTTP_getLatestServiceRelease(t *testing.T) {
	s := &OpenWrtHTTP{}

	tests := []struct {
		release string
		want    *regexp.Regexp
	}{
		{
			"17.01",
			regexp.MustCompile(`17\.01\.\d+`),
		},
		{
			"18.06",
			regexp.MustCompile(`18\.06\.\d+`),
		},
	}
	for _, tt := range tests {
		baseURL := "https://downloads.openwrt.org/releases/"
		require.Regexp(t, tt.want, s.getLatestServiceRelease(baseURL, tt.release))
	}
}
