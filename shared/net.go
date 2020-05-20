package shared

import (
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/ioprogress"
	"github.com/pkg/errors"
)

// DownloadHash downloads a file. If a checksum file is provided, it will try and
// match the hash.
func DownloadHash(def DefinitionImage, file, checksum string, hashFunc hash.Hash) (string, error) {
	var (
		client http.Client
		hashes []string
		err    error
	)
	targetDir := GetTargetDir(def)

	err = os.MkdirAll(targetDir, 0755)
	if err != nil {
		return "", err
	}

	if checksum != "" {
		if hashFunc != nil {
			hashFunc.Reset()
		}

		hashLen := 0
		if hashFunc != nil {
			hashLen = hashFunc.Size() * 2
		}

		hashes, err = downloadChecksum(targetDir, checksum, file, hashFunc, hashLen)
		if err != nil {
			return "", errors.Wrap(err, "Error while downloading checksum")
		}
	}

	imagePath := filepath.Join(targetDir, filepath.Base(file))

	stat, err := os.Stat(imagePath)
	if err == nil && stat.Size() > 0 {
		image, err := os.Open(imagePath)
		if err != nil {
			return "", err
		}
		defer image.Close()

		if checksum != "" {
			if hashFunc != nil {
				hashFunc.Reset()
			}

			_, err = io.Copy(hashFunc, image)
			if err != nil {
				return "", err
			}

			result := fmt.Sprintf("%x", hashFunc.Sum(nil))

			var hash string

			for _, h := range hashes {
				if result == h {
					hash = h
					break
				}
			}

			if hash == "" {
				return "", fmt.Errorf("Hash mismatch for %s: %s != %v", imagePath, result, hashes)
			}
		}

		return targetDir, nil
	}

	image, err := os.Create(imagePath)
	if err != nil {
		return "", err
	}
	defer image.Close()

	progress := func(progress ioprogress.ProgressData) {
		fmt.Printf("%s\r", progress.Text)
	}

	if checksum == "" {
		_, err = lxd.DownloadFileHash(&client, "", progress, nil, imagePath, file, "", nil, image)
		if err != nil && !strings.HasPrefix(err.Error(), "Hash mismatch") {
			return "", err
		}
	} else {
		// Check all file hashes in case multiple have been provided.
		for _, h := range hashes {
			if hashFunc != nil {
				hashFunc.Reset()
			}

			_, err = lxd.DownloadFileHash(&client, "", progress, nil, imagePath, file, h, hashFunc, image)
			if err == nil {
				break
			}
		}

		if err != nil {
			return "", err
		}
	}

	fmt.Println("")

	return targetDir, nil
}

// downloadChecksum downloads or opens URL, and matches fname against the
// checksums inside of the downloaded or opened file.
func downloadChecksum(targetDir string, URL string, fname string, hashFunc hash.Hash, hashLen int) ([]string, error) {
	var (
		client   http.Client
		tempFile *os.File
		err      error
	)

	// do not re-download checksum file if it's already present
	fi, err := os.Stat(filepath.Join(targetDir, URL))
	if err == nil && !fi.IsDir() {
		tempFile, err = os.Open(filepath.Join(targetDir, URL))
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile.Name())
	} else {
		tempFile, err = ioutil.TempFile(targetDir, "hash.")
		if err != nil {
			return nil, err
		}
		defer os.Remove(tempFile.Name())

		_, err = lxd.DownloadFileHash(&client, "", nil, nil, "", URL, "", hashFunc, tempFile)
		// ignore hash mismatch
		if err != nil && !strings.HasPrefix(err.Error(), "Hash mismatch") {
			return nil, err
		}
	}

	tempFile.Seek(0, 0)

	checksum := getChecksum(filepath.Base(fname), hashLen, tempFile)
	if checksum != nil {
		return checksum, nil
	}

	return nil, fmt.Errorf("Could not find checksum")
}
