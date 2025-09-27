package main

import (
	"context"
	"os"
	"testing"

	"github.com/lxc/distrobuilder/shared"
)

func lsblkOutputHelper(t *testing.T, v *vm, args [][]string) func() {
	t.Helper()
	// Prepare image file
	f, err := os.CreateTemp("", "lsblkOutput*.raw")
	if err != nil {
		t.Fatal(err)
	}

	err = f.Close()
	if err != nil {
		t.Fatal(err)
	}

	v.imageFile = f.Name()
	v.size = 2e8
	defer func() {
		if err != nil {
			os.RemoveAll(v.imageFile)
		}
	}()

	err = v.createEmptyDiskImage()
	if err != nil {
		t.Fatal(err)
	}

	v.ctx = context.Background()
	// Format disk
	if args == nil {
		err = v.createPartitions()
	} else {
		err = v.createPartitions(args...)
	}

	if err != nil {
		t.Fatal(err)
	}

	// losetup
	err = v.losetup()
	if err != nil {
		t.Fatal(err)
	}

	return func() {
		err := shared.RunCommand(v.ctx, nil, nil, "losetup", "-d", v.loopDevice)
		if err != nil {
			t.Fatal(err)
		}

		err = os.RemoveAll(v.imageFile)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestLsblkOutput(t *testing.T) {
	if os.Getuid() != 0 {
		t.Skip()
	}

	tcs := []struct {
		name string
		args [][]string
		want int
	}{
		{"DiskEmpty", [][]string{{"--zap-all"}}, 1},
		{"UEFI", nil, 3},
		{"MBR", [][]string{{"--new=1::"}, {"--gpttombr"}}, 2},
	}

	for _, tc := range tcs {
		t.Run(tc.name, func(t *testing.T) {
			v := &vm{}
			rb := lsblkOutputHelper(t, v, tc.args)
			defer rb()
			parse, num, err := v.lsblkLoopDevice()
			if err != nil || num != tc.want {
				t.Fatal(err, num, tc.want)
			}

			for i := 0; i < num; i++ {
				major, minor, err := parse(i)
				if err != nil {
					t.Fatal(err)
				}

				if major == 0 && minor == 0 {
					t.Fatal(major, minor)
				}
			}
		})
	}
}
