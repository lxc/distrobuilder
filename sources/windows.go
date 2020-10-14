package sources

import (
	"bytes"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

// Windows represents the Windows OS
type Windows struct{}

// NewWindows creates a new Windows instance.
func NewWindows() *Windows {
	return &Windows{}
}

// Run unpacks a Windows VHD file.
func (s *Windows) Run(definition shared.Definition, rootfsDir string) error {
	// URL
	u, err := url.Parse(definition.Source.URL)
	if err != nil {
		return errors.Wrap(err, "Failed to parse URL")
	}

	if u.Scheme != "file" {
		return fmt.Errorf("Scheme %q is not supported", u.Scheme)
	}

	rawFilePath := fmt.Sprintf("%s.raw", u.Path)

	if !lxd.PathExists(rawFilePath) {
		// Convert the vhd image to raw.
		err = shared.RunCommand("qemu-img", "convert", "-O", "raw", u.Path, rawFilePath)
		if err != nil {
			return errors.Wrap(err, "Failed to convert image")
		}
	}

	// Figure out the offset
	var buf bytes.Buffer

	err = lxd.RunCommandWithFds(nil, &buf, "fdisk", "-l", "-o", "Start", rawFilePath)
	if err != nil {
		return errors.Wrap(err, "Failed to list partitions")
	}

	output := strings.Split(buf.String(), "\n")
	offsetStr := strings.TrimSpace(output[len(output)-2])

	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return errors.Wrap(err, "Failed to read offset")
	}

	roRootDir := filepath.Join(os.TempDir(), "distrobuilder", "rootfs.ro")

	err = os.MkdirAll(roRootDir, 0755)
	if err != nil {
		return errors.Wrap(err, "Failed to create directory")
	}
	defer os.RemoveAll(filepath.Join(os.TempDir(), "distrobuilder"))

	// Mount the partition read-only since we don't want to accidently modify it.
	err = shared.RunCommand("mount", "-o", fmt.Sprintf("ro,loop,offset=%d", offset*512),
		rawFilePath, roRootDir)
	if err != nil {
		return errors.Wrap(err, "Failed to mount partition read-only")
	}

	// Copy the read-only rootfs to the real rootfs.
	err = shared.RunCommand("rsync", "-qa", roRootDir+"/", rootfsDir)
	if err != nil {
		return errors.Wrap(err, "Failed to copy rootfs")
	}

	return nil
}
