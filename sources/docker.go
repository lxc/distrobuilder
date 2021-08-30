package sources

import (
	"os"
	"path/filepath"

	dcapi "github.com/mudler/docker-companion/api"
	"github.com/pkg/errors"
)

type docker struct {
	common
}

// Run downloads and unpacks a docker image
func (s *docker) Run() error {
	absRootfsDir, err := filepath.Abs(s.rootfsDir)
	if err != nil {
		return errors.WithMessagef(err, "Failed to get absolute path of %s", s.rootfsDir)
	}

	// If DOCKER_REGISTRY_BASE is not set it's used default https://registry-1.docker.io
	err = dcapi.DownloadAndUnpackImage(s.definition.Source.URL, absRootfsDir, &dcapi.DownloadOpts{
		RegistryBase:     os.Getenv("DOCKER_REGISTRY_BASE"),
		RegistryUsername: os.Getenv("DOCKER_REGISTRY_BASE_USER"),
		RegistryPassword: os.Getenv("DOCKER_REGISTRY_BASE_PASS"),
		KeepLayers:       false,
	})
	if err != nil {
		return errors.WithMessage(err, "Failed to download an unpack image")
	}

	return nil
}
