package sources

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/opencontainers/umoci"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"

	"github.com/lxc/distrobuilder/shared"
)

type docker struct {
	common
}

// Run downloads and unpacks a docker image.
func (s *docker) Run() error {
	absRootfsDir, err := filepath.Abs(s.rootfsDir)
	if err != nil {
		return fmt.Errorf("Failed to get absolute path of %s: %w", s.rootfsDir, err)
	}

	// Get some temporary storage.
	ociPath, err := os.MkdirTemp("", "incus-oci-")
	if err != nil {
		return err
	}

	defer func() { _ = os.RemoveAll(ociPath) }()

	// Download from Docker Hub.
	imageTag := "latest"
	err = shared.RunCommand(
		context.TODO(),
		nil,
		nil,
		"skopeo",
		"--insecure-policy",
		"copy",
		"--remove-signatures",
		fmt.Sprintf("%s/%s", "docker://docker.io", s.definition.Source.URL),
		fmt.Sprintf("oci:%s:%s", ociPath, imageTag))
	if err != nil {
		return err
	}

	// Unpack.
	var unpackOptions layer.UnpackOptions
	unpackOptions.KeepDirlinks = true

	engine, err := dir.Open(ociPath)
	if err != nil {
		return err
	}

	engineExt := casext.NewEngine(engine)
	defer func() { _ = engine.Close() }()

	return umoci.Unpack(engineExt, imageTag, absRootfsDir, unpackOptions)
}
