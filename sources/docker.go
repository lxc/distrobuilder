package sources

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	imgspec "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/opencontainers/umoci/oci/cas/dir"
	"github.com/opencontainers/umoci/oci/casext"
	"github.com/opencontainers/umoci/oci/layer"
	"go.podman.io/image/v5/copy"
	"go.podman.io/image/v5/docker/reference"
	"go.podman.io/image/v5/oci/layout"
	"go.podman.io/image/v5/signature"
	"go.podman.io/image/v5/transports/alltransports"
	"go.podman.io/image/v5/types"
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

	// Parse the image reference
	imageRef, err := reference.ParseNormalizedNamed(s.definition.Source.URL)
	if err != nil {
		return fmt.Errorf("Failed to parse image reference: %w", err)
	}

	// Docker references with both a tag and digest are currently not supported
	var imageTag string
	digested, ok := imageRef.(reference.Digested)
	if ok {
		imageTag = digested.Digest().String()
	} else {
		imageTag = "latest"
		tagged, ok := imageRef.(reference.NamedTagged)
		if ok {
			imageTag = tagged.Tag()
		}
	}

	srcRef, err := alltransports.ParseImageName(fmt.Sprintf("docker://%s", s.definition.Source.URL))
	if err != nil {
		return fmt.Errorf("Failed to parse image name: %w", err)
	}

	dstRef, err := layout.ParseReference(fmt.Sprintf("%s:%s", ociPath, imageTag))
	if err != nil {
		return fmt.Errorf("Failed to parse destination reference: %w", err)
	}

	// Create policy context
	systemCtx := &types.SystemContext{
		DockerInsecureSkipTLSVerify: types.OptionalBoolFalse,
	}

	policy := &signature.Policy{
		Default: []signature.PolicyRequirement{signature.NewPRInsecureAcceptAnything()},
	}

	policyCtx, err := signature.NewPolicyContext(policy)
	if err != nil {
		return fmt.Errorf("Failed to create policy context: %w", err)
	}

	defer func() { _ = policyCtx.Destroy() }()

	copyOptions := &copy.Options{
		RemoveSignatures: true,
		SourceCtx:        systemCtx,
		DestinationCtx:   systemCtx,
	}

	ctx := context.TODO()

	// Pull image from OCI registry
	copiedManifest, err := copy.Image(ctx, policyCtx, dstRef, srcRef, copyOptions)
	if err != nil {
		return err
	}

	// Unpack OCI image
	unpackOptions := &layer.UnpackOptions{KeepDirlinks: true}

	engine, err := dir.Open(ociPath)
	if err != nil {
		return err
	}

	engineExt := casext.NewEngine(engine)

	defer func() { _ = engine.Close() }()

	var manifest imgspec.Manifest
	err = json.Unmarshal(copiedManifest, &manifest)
	if err != nil {
		return fmt.Errorf("Failed to parse manifest: %w", err)
	}

	return layer.UnpackRootfs(ctx, engineExt, absRootfsDir, manifest, unpackOptions)
}
