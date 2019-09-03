package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestOpenWrtHTTP_getLatestServiceRelease(t *testing.T) {
	s := &OpenWrtHTTP{}

	tests := []struct {
		release string
		want    string
	}{
		{
			"17.01",
			"17.01.7",
		},
		{
			"18.06",
			"18.06.4",
		},
	}
	for _, tt := range tests {
		baseURL := "https://downloads.openwrt.org/releases/"
		require.Equal(t, tt.want, s.getLatestServiceRelease(baseURL, tt.release))
	}
}
