package sources

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetLatestRelease(t *testing.T) {
	s := &openEuler{}

	tests := []struct {
		url        string
		release    string
		want       string
		shouldFail bool
	}{
		{
			"https://repo.openeuler.org/",
			"22.03",
			"22.03-LTS-SP3",
			false,
		},
		{
			"https://repo.openeuler.org/",
			"20.03",
			"20.03-LTS-SP4",
			false,
		},
		{
			"https://repo.openeuler.org/",
			"20.03-LTS",
			"20.03-LTS-SP4",
			false,
		},
		{
			"https://repo.openeuler.org/",
			"20.03-LTS-SP1",
			"20.03-LTS-SP1",
			false,
		},
		{
			"https://repo.openeuler.org/",
			"21.03",
			"21.03",
			false,
		},
		{
			"https://repo.openeuler.org/",
			"22.00", // non-existed release
			"",
			true,
		},
		{
			"https://repo.openeuler.org/",
			"BadRelease", // invalid format
			"",
			true,
		},
		{
			"https://repo.openeuler.org/",
			"", // null string
			"",
			true,
		},
		{
			"foobar", // invalid url
			"22.03",
			"",
			true,
		},
	}

	for _, test := range tests {
		release, err := s.getLatestRelease(test.url, test.release)
		if test.shouldFail {
			require.NotNil(t, err)
		} else {
			require.NoError(t, err)
			require.NotEmpty(t, release)
			require.Equal(t, test.want, release)
		}
	}
}
