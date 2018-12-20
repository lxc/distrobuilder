package shared

import (
	"bufio"
	"fmt"
	"hash"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/ioprogress"
)

// DownloadHash downloads a file. If a checksum file is provided, it will try and
// match the hash.
func DownloadHash(file, checksum string, hashFunc hash.Hash) error {
	var (
		client http.Client
		hash   string
		err    error
	)

	if checksum != "" {
		if hashFunc != nil {
			hashFunc.Reset()
		}

		hash, err = downloadChecksum(checksum, file, hashFunc)
		if err != nil {
			return fmt.Errorf("Error while downloading checksum: %s", err)
		}
	}

	imagePath := filepath.Join(os.TempDir(), filepath.Base(file))

	stat, err := os.Stat(imagePath)
	if err == nil && stat.Size() > 0 {
		image, err := os.Open(imagePath)
		if err != nil {
			return err
		}
		defer image.Close()

		if checksum != "" {
			if hashFunc != nil {
				hashFunc.Reset()
			}

			_, err = io.Copy(hashFunc, image)
			if err != nil {
				return err
			}

			result := fmt.Sprintf("%x", hashFunc.Sum(nil))
			if result != hash {
				return fmt.Errorf("Hash mismatch for %s: %s != %s", imagePath, result, hash)
			}
		}

		return nil
	}

	image, err := os.Create(imagePath)
	if err != nil {
		return err
	}
	defer image.Close()

	progress := func(progress ioprogress.ProgressData) {
		fmt.Printf("%s\r", progress.Text)
	}

	if hashFunc != nil {
		hashFunc.Reset()
	}
	_, err = lxd.DownloadFileHash(&client, "", progress, nil, imagePath, file, hash, hashFunc, image)
	if err != nil {
		if checksum == "" && strings.HasPrefix(err.Error(), "Hash mismatch") {
			return nil
		}
		return err
	}

	fmt.Println("")

	return nil
}

// downloadChecksum downloads or opens URL, and matches fname against the
// checksums inside of the downloaded or opened file.
func downloadChecksum(URL string, fname string, hashFunc hash.Hash) (string, error) {
	var (
		client   http.Client
		tempFile *os.File
		err      error
	)

	// do not re-download checksum file if it's already present
	fi, err := os.Stat(filepath.Join(os.TempDir(), URL))
	if err == nil && !fi.IsDir() {
		tempFile, err = os.Open(filepath.Join(os.TempDir(), URL))
		if err != nil {
			return "", err
		}
		defer os.Remove(tempFile.Name())
	} else {
		tempFile, err = ioutil.TempFile(os.TempDir(), "hash.")
		if err != nil {
			return "", err
		}
		defer os.Remove(tempFile.Name())

		_, err = lxd.DownloadFileHash(&client, "", nil, nil, "", URL, "", hashFunc, tempFile)
		// ignore hash mismatch
		if err != nil && !strings.HasPrefix(err.Error(), "Hash mismatch") {
			return "", err
		}
	}

	tempFile.Seek(0, 0)

	scanner := bufio.NewScanner(tempFile)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), " ")
		matched, _ := regexp.MatchString(fmt.Sprintf(".*%s", filepath.Base(fname)), s[len(s)-1])
		if matched {
			return s[0], nil
		}
	}

	return "", fmt.Errorf("Could not find checksum")
}
