package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_detectWindowsVersion(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := detectWindowsVersion(tt.args.fileName)
			assert.Equal(t, tt.want, got)
		})
	}
}
