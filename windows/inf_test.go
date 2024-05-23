package windows

import (
	"os"
	"testing"
)

func TestMatchClassGuid(t *testing.T) {
	tcs := []struct {
		filename string
		want     string
	}{
		{"testdata/inf01.txt", "{4D36E97B-E325-11CE-BFC1-08002BE10318}"},
		{"testdata/inf02.txt", "{4D36E97B-E325-11CE-BFC1-08002BE10318}"},
		{"testdata/inf03.txt", ""},
	}

	for _, tc := range tcs {
		t.Run("", func(t *testing.T) {
			file, err := os.Open(tc.filename)
			if err != nil {
				t.Fatal(err)
			}

			actual := MatchClassGuid(file)
			if actual != tc.want {
				t.Fatal(actual, tc.want)
			}
		})
	}
}
