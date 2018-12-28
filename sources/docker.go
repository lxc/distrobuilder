package sources

import (
	"path/filepath"

	dcapi "github.com/mudler/docker-companion/api"

	"github.com/lxc/distrobuilder/shared"
)

// DockerHTTP represents the Docker HTTP downloader.
type DockerHTTP struct{}

// NewDockerHTTP create a new DockerHTTP instance.
func NewDockerHTTP() *DockerHTTP {
	return &DockerHTTP{}
}

// Run downloads and unpacks a docker image
func (d *DockerHTTP) Run(definition shared.Definition, rootfsDir string) error {
	absRootfsDir, err := filepath.Abs(rootfsDir)
	if err != nil {
		return err
	}

	// NOTE: For now we use only docker official server but we can
	//       add a new parameter on DefinitionSource struct.
	return dcapi.DownloadImage(definition.Source.URL, absRootfsDir, "")
}
