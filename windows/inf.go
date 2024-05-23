package windows

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

var (
	versionRe   = regexp.MustCompile(`(?i)^\[Version\][ ]*$`)
	classGuidRe = regexp.MustCompile(`(?i)^ClassGuid[ ]*=[ ]*(.+)$`)
)

func ParseDriverClassGuid(driverName, infPath string) (classGuid string, err error) {
	// Retrieve the ClassGuid which is needed for the Windows registry entries.
	file, err := os.Open(infPath)
	if err != nil {
		err = fmt.Errorf("Failed to open driver %s inf %s: %w", driverName, infPath, err)
		return
	}

	defer func() {
		file.Close()
		if classGuid == "" {
			err = fmt.Errorf("Failed to parse driver %s classGuid %s", driverName, infPath)
		}
	}()

	classGuid = MatchClassGuid(file)
	return
}

func MatchClassGuid(r io.Reader) (classGuid string) {
	scanner := bufio.NewScanner(r)
	versionFlag := false
	for scanner.Scan() {
		line := scanner.Text()
		if !versionFlag {
			if versionRe.MatchString(line) {
				versionFlag = true
			}

			continue
		}

		matches := classGuidRe.FindStringSubmatch(line)
		if len(matches) > 1 {
			classGuid = strings.TrimSpace(matches[1])
			if classGuid != "" {
				return
			}
		}
	}

	return
}
