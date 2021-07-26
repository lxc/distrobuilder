package sources

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	lxd "github.com/lxc/lxd/shared"
	"github.com/pkg/errors"

	"github.com/lxc/distrobuilder/shared"
)

type fedora struct {
	common
}

// Run downloads a container base image and unpacks it and its layers.
func (s *fedora) Run() error {
	baseURL := fmt.Sprintf("%s/packages/Fedora-Container-Base",
		s.definition.Source.URL)

	// Get latest build
	build, err := s.getLatestBuild(baseURL, s.definition.Image.Release)
	if err != nil {
		return errors.Wrap(err, "Failed to get latest build")
	}

	fname := fmt.Sprintf("Fedora-Container-Base-%s-%s.%s.tar.xz",
		s.definition.Image.Release, build, s.definition.Image.ArchitectureMapped)

	// Download image
	sourceURL := fmt.Sprintf("%s/%s/%s/images/%s", baseURL, s.definition.Image.Release, build, fname)

	fpath, err := shared.DownloadHash(s.definition.Image, sourceURL, "", nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to download %q", sourceURL)
	}

	s.logger.Infow("Unpacking image", "file", filepath.Join(fpath, fname))

	// Unpack the base image
	err = lxd.Unpack(filepath.Join(fpath, fname), s.rootfsDir, false, false, nil)
	if err != nil {
		return errors.Wrapf(err, "Failed to unpack %q", filepath.Join(fpath, fname))
	}

	s.logger.Info("Unpacking layers")

	// Unpack the rest of the image (/bin, /sbin, /usr, etc.)
	err = s.unpackLayers(s.rootfsDir)
	if err != nil {
		return errors.Wrap(err, "Failed to unpack")
	}

	return nil
}

func (s *fedora) unpackLayers(rootfsDir string) error {
	// Read manifest file which contains the path to the layers
	file, err := os.Open(filepath.Join(rootfsDir, "manifest.json"))
	if err != nil {
		return errors.Wrapf(err, "Failed to open %q", filepath.Join(rootfsDir, "manifest.json"))
	}
	defer file.Close()

	data, err := ioutil.ReadAll(file)
	if err != nil {
		return errors.Wrapf(err, "Failed to read file %q", file.Name())
	}

	// Structure of the manifest excluding RepoTags
	var manifests []struct {
		Layers []string
		Config string
	}

	err = json.Unmarshal(data, &manifests)
	if err != nil {
		return errors.Wrap(err, "Failed to unmarshal JSON data")
	}

	pathsToRemove := []string{
		filepath.Join(rootfsDir, "manifest.json"),
		filepath.Join(rootfsDir, "repositories"),
	}

	// Unpack tarballs (or layers) which contain the rest of the rootfs, and
	// remove files not relevant to the image.
	for _, manifest := range manifests {
		for _, layer := range manifest.Layers {
			s.logger.Infow("Unpacking layer", "file", filepath.Join(rootfsDir, layer))

			err := lxd.Unpack(filepath.Join(rootfsDir, layer), rootfsDir, false, false, nil)
			if err != nil {
				return errors.Wrapf(err, "Failed to unpack %q", filepath.Join(rootfsDir, layer))
			}

			pathsToRemove = append(pathsToRemove,
				filepath.Join(rootfsDir, filepath.Dir(layer)))
		}

		pathsToRemove = append(pathsToRemove, filepath.Join(rootfsDir, manifest.Config))
	}

	// Clean up /tmp since there are unnecessary files there
	files, err := filepath.Glob(filepath.Join(rootfsDir, "tmp", "*"))
	if err != nil {
		return errors.Wrap(err, "Failed to find matching files")
	}
	pathsToRemove = append(pathsToRemove, files...)

	// Clean up /root since there are unnecessary files there
	files, err = filepath.Glob(filepath.Join(rootfsDir, "root", "*"))
	if err != nil {
		return errors.Wrap(err, "Failed to find matching files")
	}
	pathsToRemove = append(pathsToRemove, files...)

	for _, f := range pathsToRemove {
		os.RemoveAll(f)
	}

	return nil
}

func (s *fedora) getLatestBuild(URL, release string) (string, error) {
	resp, err := http.Get(fmt.Sprintf("%s/%s", URL, release))
	if err != nil {
		return "", errors.Wrapf(err, "Failed to GET %q", fmt.Sprintf("%s/%s", URL, release))
	}
	defer resp.Body.Close()

	content, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", errors.Wrap(err, "Failed to read body")
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
