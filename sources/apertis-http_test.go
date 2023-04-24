package sources

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApertisHTTP_getLatestRelease(t *testing.T) {
	s := &apertis{}

	tests := []struct {
		release string
		want    string
	}{
		{
			"18.12",
			"18.12.0",
		},
	}

	for _, tt := range tests {
		baseURL := fmt.Sprintf("https://images.apertis.org/release/%s", tt.release)
		release, err := s.getLatestRelease(baseURL, tt.release)
		require.NoError(t, err)
		require.Equal(t, tt.want, release)
	}
}
