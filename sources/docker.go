package sources

import (
	"os"
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

	// If DOCKER_REGISTRY_BASE is not set it's used default https://registry-1.docker.io
	return dcapi.DownloadAndUnpackImage(definition.Source.URL, absRootfsDir, &dcapi.DownloadOpts{
		RegistryBase: os.Getenv("DOCKER_REGISTRY_BASE"),
		KeepLayers:   false,
	})
}
