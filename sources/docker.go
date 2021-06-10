package sources

import (
	"os"
	"path/filepath"

	dcapi "github.com/mudler/docker-companion/api"
)

type docker struct {
	common
}

// Run downloads and unpacks a docker image
func (s *docker) Run() error {
	absRootfsDir, err := filepath.Abs(s.rootfsDir)
	if err != nil {
		return err
	}

	// If DOCKER_REGISTRY_BASE is not set it's used default https://registry-1.docker.io
	return dcapi.DownloadAndUnpackImage(s.definition.Source.URL, absRootfsDir, &dcapi.DownloadOpts{
		RegistryBase:     os.Getenv("DOCKER_REGISTRY_BASE"),
		RegistryUsername: os.Getenv("DOCKER_REGISTRY_BASE_USER"),
		RegistryPassword: os.Getenv("DOCKER_REGISTRY_BASE_PASS"),
		KeepLayers:       false,
	})
}
