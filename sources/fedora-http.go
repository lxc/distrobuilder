package sources

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	lxd "github.com/lxc/lxd/shared"

	"github.com/lxc/distrobuilder/shared"
)

// FedoraHTTP represents the Fedora HTTP downloader.
type FedoraHTTP struct{}

// NewFedoraHTTP creates a new FedoraHTTP instance.
func NewFedoraHTTP() *FedoraHTTP {
	return &FedoraHTTP{}
}

// Run downloads a container base image and unpacks it and its layers.
func (s *FedoraHTTP) Run(definition shared.Definition, rootfsDir string) error {
	baseURL := fmt.Sprintf("%s/packages/Fedora-Container-Base",
		definition.Source.URL)

	// Get latest build
	build, err := s.getLatestBuild(baseURL, definition.Image.Release)
	if err != nil {
		return err
	}

	fname := fmt.Sprintf("Fedora-Container-Base-%s-%s.%s.tar.xz",
		definition.Image.Release, build, definition.Image.ArchitectureMapped)

	// Download image
	fpath, err := shared.DownloadHash(definition.Image, fmt.Sprintf("%s/%s/%s/images/%s",
		baseURL, definition.Image.Release, build, fname), "", nil)
	if err != nil {
		return err
	}

	// Unpack the base image
	err = lxd.Unpack(filepath.Join(fpath, fname), rootfsDir, false, false, nil)
	if err != nil {
		return err
	}

	// Unpack the rest of the image (/bin, /sbin, /usr, etc.)
	return s.unpackLayers(rootfsDir)
}

func (s *FedoraHTTP) unpackLayers(rootfsDir string) error {
	// Read manifest file which contains the path to the layers
	file, err := os.Open(filepath.Join(rootfsDir, "manifest.json"))
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return err
	}

	// Structure of the manifest excluding RepoTags
	var manifests []struct {
		Layers []string
		Config string
	}

	err = json.Unmarshal(data, &manifests)
	if err != nil {
		return err
	}

	pathsToRemove := []string{
		filepath.Join(rootfsDir, "manifest.json"),
		filepath.Join(rootfsDir, "repositories"),
	}

	// Unpack tarballs (or layers) which contain the rest of the rootfs, and
	// remove files not relevant to the image.
	for _, manifest := range manifests {
		for _, layer := range manifest.Layers {
			err := lxd.Unpack(filepath.Join(rootfsDir, layer), rootfsDir, false, false, nil)
			if err != nil {
				return err
			}

			pathsToRemove = append(pathsToRemove,
				filepath.Join(rootfsDir, filepath.Dir(layer)))
		}

		pathsToRemove = append(pathsToRemove, filepath.Join(rootfsDir, manifest.Config))
	}

	// Clean up /tmp since there are unnecessary files there
	files, err := filepath.Glob(filepath.Join(rootfsDir, "tmp", "*"))
	if err != nil {
		return err
	}
	pathsToRemove = append(pathsToRemove, files...)

	// Clean up /root since there are unnecessary files there
	files, err = filepath.Glob(filepath.Join(rootfsDir, "root", "*"))
	if err != nil {
		return err
	}
	pathsToRemove = append(pathsToRemove, files...)

	for _, f := range pathsToRemove {
		os.RemoveAll(f)
	}

	return nil
}

func (s *FedoraHTTP) getLatestBuild(URL, release string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", URL, release))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Builds are formatted in one of two ways:
	//   - <yyyy><mm><dd>.<build_number>
	//   - <yyyy><mm><dd>.n.<build_number>
	re := regexp.MustCompile(`\d{8}\.(n\.)?\d`)

	// Find all builds
	matches := re.FindAllString(string(content), -1)

	if len(matches) == 0 {
		return "", errors.New("Unable to find latest build")
	}

	// Sort builds
	sort.Strings(matches)

	// Return latest build
	return matches[len(matches)-1], nil
}
