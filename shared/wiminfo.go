package shared

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"strconv"
	"strings"
)

type WimInfo map[int]map[string]string

func (info WimInfo) ImageCount() int {
	return len(info) - 1
}

func (info WimInfo) Name(index int) string {
	return info[index]["Name"]
}

func (info WimInfo) MajorVersion(index int) string {
	return info[index]["Major Version"]
}

func (info WimInfo) Architecture(index int) string {
	return info[index]["Architecture"]
}

func ParseWimInfo(r io.Reader) (info WimInfo, err error) {
	scanner := bufio.NewScanner(r)
	nextSection := func() (sect map[string]string) {
		sect = map[string]string{}
		for scanner.Scan() {
			line := scanner.Text()
			if line == "" {
				break
			}

			idx := strings.IndexByte(line, ':')
			if idx == -1 {
				continue
			}

			key := strings.TrimSpace(line[:idx])
			if key == "" {
				continue
			}

			val := strings.TrimSpace(line[idx+1:])
			sect[key] = val
		}

		return
	}

	header := nextSection()
	count, err := strconv.Atoi(header["Image Count"])
	if err != nil {
		return
	}

	if count == 0 {
		err = fmt.Errorf("Failed to parse wim info")
		return
	}

	info = WimInfo{0: header}
	for i := 1; i <= count; i++ {
		index, section := 0, nextSection()
		index, err = strconv.Atoi(section["Index"])
		if err != nil {
			return
		}

		if index != i {
			err = fmt.Errorf("Failed to parse wim info: %d != %d", index, i)
			return
		}

		info[i] = section
	}

	return
}

func DetectWindowsVersion(desc string) string {
	aliases := map[string][]string{
		"2k12r2": {"2k12r2", "w2k12r2", "win2k12r2", "windows.?server.?2012?.r2"},
		"2k8r2":  {"2k8r2", "w2k8r2", "win2k8r2", "windows.?server.?2008?.r2"},
		"w8.1":   {"w8.1", "win8.1", "windows.?8.1"},
	}

	for k, v := range aliases {
		for _, alias := range v {
			if regexp.MustCompile(fmt.Sprintf("(?i)%s", alias)).MatchString(desc) {
				return k
			}
		}
	}

	aliases = map[string][]string{
		"w11":  {"w11", "win11", "windows.?11"},
		"w10":  {"w10", "win10", "windows.?10"},
		"w8":   {"w8", "win8", "windows.?8"},
		"w7":   {"w7", "win7", "windows.?7"},
		"2k19": {"2k19", "w2k19", "win2k19", "windows.?server.?2019"},
		"2k12": {"2k12", "w2k12", "win2k12", "windows.?server.?2012"},
		"2k16": {"2k16", "w2k16", "win2k16", "windows.?server.?2016"},
		"2k22": {"2k22", "w2k22", "win2k22", "windows.?server.?2022"},
		"2k3":  {"2k3", "w2k3", "win2k3", "windows.?server.?2003"},
		"2k8":  {"2k8", "w2k8", "win2k8", "windows.?server.?2008"},
		"xp":   {"xp", "wxp", "winxp", "windows.?xp"},
	}

	for k, v := range aliases {
		for _, alias := range v {
			if regexp.MustCompile(fmt.Sprintf("(?i)%s", alias)).MatchString(desc) {
				return k
			}
		}
	}

	return ""
}

func DetectWindowsArchitecture(desc string) string {
	aliases := map[string][]string{
		"amd64": {"amd64", "x64", "x86_64"},
		"ARM64": {"arm64", "aarch64"},
	}

	for k, v := range aliases {
		for _, alias := range v {
			if regexp.MustCompile(fmt.Sprintf("(?i)%s", alias)).MatchString(desc) {
				return k
			}
		}
	}

	return ""
}
