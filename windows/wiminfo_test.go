package windows

import (
	"os"
	"reflect"
	"testing"
)

func TestParseWimInfo(t *testing.T) {
	tcs := []struct {
		filename string
		count    int
		info     map[int]string
	}{
		{"testdata/w10_install_wim_info.txt", 11, map[int]string{
			1:  "Windows 10 Home",
			2:  "Windows 10 Home N",
			3:  "Windows 10 Home Single Language",
			4:  "Windows 10 Education",
			5:  "Windows 10 Education N",
			6:  "Windows 10 Pro",
			7:  "Windows 10 Pro N",
			8:  "Windows 10 Pro Education",
			9:  "Windows 10 Pro Education N",
			10: "Windows 10 Pro for Workstations",
			11: "Windows 10 Pro N for Workstations",
		}},
		{"testdata/w10_boot_wim_info.txt", 2, map[int]string{
			1: "Microsoft Windows PE (x64)",
			2: "Microsoft Windows Setup (x64)",
		}},
		{"testdata/winpe_boot_wim_info.txt", 1, map[int]string{
			1: "Microsoft Windows PE (amd64)",
		}},
	}

	for _, tc := range tcs {
		t.Run(tc.filename, func(t *testing.T) {
			f, err := os.Open(tc.filename)
			if err != nil {
				t.Fatal(err)
			}

			defer f.Close()
			info, err := ParseWimInfo(f)
			if err != nil || len(info) != tc.count+1 {
				t.Fatal(err, info)
			}

			images := map[int]string{}
			for i := 1; i < len(info); i++ {
				images[i] = info.Name(i)
			}

			if !reflect.DeepEqual(images, tc.info) {
				t.Fatal(images, tc.info)
			}
		})
	}
}

func TestDetectWindowsVersion(t *testing.T) {
	tcs := []struct {
		filename       string
		windowsVersion string
		architecture   string
	}{
		{"testdata/w10_install_wim_info.txt", "w10", "amd64"},
		{"testdata/2k19_install_wim_info.txt", "2k19", "amd64"},
		{"testdata/w8_install_wim_info.txt", "w8", "amd64"},
		{"testdata/2k12r2_install_wim_info.txt", "2k12r2", "amd64"},
		{"testdata/w7_install_wim_info.txt", "w7", "x86"},
	}

	for _, tc := range tcs {
		t.Run(tc.filename, func(t *testing.T) {
			f, err := os.Open(tc.filename)
			if err != nil {
				t.Fatal(err)
			}

			defer f.Close()
			info, err := ParseWimInfo(f)
			if err != nil {
				t.Fatal(err)
			}

			windowsVersion := DetectWindowsVersion(info.Name(1))
			if windowsVersion != tc.windowsVersion {
				t.Fatal(windowsVersion, tc.windowsVersion)
			}

			architecture := DetectWindowsArchitecture(info.Architecture(1))
			if architecture != tc.architecture {
				t.Fatal(architecture, tc.architecture)
			}
		})
	}
}

func TestDetectWindowsVersionFromFilename(t *testing.T) {
	type args struct {
		fileName string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Windows 11 (1)",
			args{"Windows 11.iso"},
			"w11",
		},
		{
			"Windows 11 (2)",
			args{"Windows11.iso"},
			"w11",
		},
		{
			"Windows 11 (3)",
			args{"Win11.iso"},
			"w11",
		},
		{
			"Windows 11 (4)",
			args{"Windows_11.iso"},
			"w11",
		},
		{
			"Windows 10 (1)",
			args{"Windows 10.iso"},
			"w10",
		},
		{
			"Windows 10 (2)",
			args{"Windows10.iso"},
			"w10",
		},
		{
			"Windows 10 (3)",
			args{"Win10.iso"},
			"w10",
		},
		{
			"Windows 10 (4)",
			args{"Windows_10.iso"},
			"w10",
		},
		{
			"Windows Server 2019 (1)",
			args{"Windows_Server_2019.iso"},
			"2k19",
		},
		{
			"Windows Server 2019 (2)",
			args{"Windows Server 2019.iso"},
			"2k19",
		},
		{
			"Windows Server 2019 (3)",
			args{"WindowsServer2019.iso"},
			"2k19",
		},
		{
			"Windows Server 2019 (4)",
			args{"Windows_Server_2k19.iso"},
			"2k19",
		},
		{
			"Windows Server 2012 (1)",
			args{"Windows_Server_2012.iso"},
			"2k12",
		},
		{
			"Windows Server 2012 (2)",
			args{"Windows Server 2012.iso"},
			"2k12",
		},
		{
			"Windows Server 2012 (3)",
			args{"WindowsServer2012.iso"},
			"2k12",
		},
		{
			"Windows Server 2012 (4)",
			args{"Windows_Server_2k12.iso"},
			"2k12",
		},
		{
			"Windows 7",
			args{"cn_windows_7_professional_x86_dvd_x15-65790.iso"},
			"w7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectWindowsVersion(tt.args.fileName)
			if tt.want != got {
				t.Fatal(got, tt.want)
			}
		})
	}
}

func TestDetectWindowsArchitectureFromFilename(t *testing.T) {
	type args struct {
		fileName string
	}

	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"Windows 11 (1)",
			args{"Win10_22H2_English_x64.iso"},
			"amd64",
		},
		{
			"Windows 11 (2)",
			args{"Win10_22H2_English_arm64.iso"},
			"ARM64",
		},
		{
			"Windows 7",
			args{"cn_windows_7_professional_x86_dvd_x15-65790.iso"},
			"x86",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectWindowsArchitecture(tt.args.fileName)
			if got != tt.want {
				t.Fatal(got, tt.want)
			}
		})
	}
}
