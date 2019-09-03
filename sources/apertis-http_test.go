package sources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApertisHTTP_getLatestRelease(t *testing.T) {
	s := &ApertisHTTP{}

	tests := []struct {
		release string
		want    string
	}{
		{
			"17.12",
			"17.12.1",
		},
		{
			"18.03",
			"18.03.0",
		},
		{
			"18.12",
			"18.12.0",
		},
		{
			"v2019pre",
			"v2019pre.0",
		},
	}
	for _, tt := range tests {
		baseURL := fmt.Sprintf("https://images.apertis.org/release/%s", tt.release)
		require.Equal(t, tt.want, s.getLatestRelease(baseURL, tt.release))
	}
}
