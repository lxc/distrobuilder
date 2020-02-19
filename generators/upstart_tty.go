package generators

import (
	"os"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/api"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/image"
	"github.com/lxc/distrobuilder/shared"
)

var upstartTTYJob = `start on starting tty1 or starting tty2 or starting tty3 or starting tty4 or starting tty5 or starting tty6
instance $JOB

script
    set -eu

    # Check that we're inside a container
    [ -e "/run/container_type" ] || exit 0
    [ "$(cat /run/container_type)" = "lxc" ] || exit 0

    # Load PID1's environment
    LXC_TTY=$(tr '\0' '\n' < /proc/1/environ | grep ^container_ttys= | cut -d= -f2-)

    # Check if we have any consoles setup
    if [ -z "${LXC_TTY}" ]; then
        # No TTYs setup in this container
        stop "${JOB}" >/dev/null 2>&1
        exit 0
    fi

    TTY_COUNT=$(echo ${LXC_TTY} | wc -w)
    JOB_ID="${JOB#tty}"

    if [ "${JOB_ID}" -gt "${TTY_COUNT}" ]; then
        # This console isn't available in the container
        stop "${JOB}" >/dev/null 2>&1
        exit 0
    fi

    # Allow the tty to start
    exit 0
end script
`

// UpstartTTYGenerator represents the UpstartTTY generator.
type UpstartTTYGenerator struct{}

// RunLXC creates a hostname template.
func (g UpstartTTYGenerator) RunLXC(cacheDir, sourceDir string, img *image.LXCImage,
	target shared.DefinitionTargetLXC, defFile shared.DefinitionFile) error {

	// Skip if the file exists
	if lxd.PathExists(filepath.Join(sourceDir, defFile.Path)) {
		return nil
	}

	// Store original file
	err := StoreFile(cacheDir, sourceDir, defFile.Path)
	if err != nil {
		return err
	}

	// Create new hostname file
	file, err := os.Create(filepath.Join(sourceDir, defFile.Path))
	if err != nil {
		return err
	}
	defer file.Close()

	// Write LXC specific string to the hostname file
	_, err = file.WriteString(upstartTTYJob)
	if err != nil {
		return errors.Wrap(err, "Failed to write to upstart job file")
	}

	// Add hostname path to LXC's templates file
	return img.AddTemplate(defFile.Path)
}

// RunLXD creates a hostname template.
func (g UpstartTTYGenerator) RunLXD(cacheDir, sourceDir string, img *image.LXDImage,
	target shared.DefinitionTargetLXD, defFile shared.DefinitionFile) error {

	// Skip if the file exists
	if lxd.PathExists(filepath.Join(sourceDir, defFile.Path)) {
		return nil
	}

	templateDir := filepath.Join(cacheDir, "templates")

	err := os.MkdirAll(templateDir, 0755)
	if err != nil {
		return err
	}

	file, err := os.Create(filepath.Join(templateDir, "upstart-tty.tpl"))
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(upstartTTYJob)
	if err != nil {
		return errors.Wrap(err, "Failed to write to upstart job file")
	}

	// Add to LXD templates
	img.Metadata.Templates[defFile.Path] = &api.ImageMetadataTemplate{
		Template: "upstart-tty.tpl",
		When: []string{
			"create",
		},
	}

	return err
}

// Run does nothing.
func (g UpstartTTYGenerator) Run(cacheDir, sourceDir string,
	defFile shared.DefinitionFile) error {
	return nil
}
