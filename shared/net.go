package shared

import (
	"bufio"
	"crypto/sha256"
	"fmt"
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

// Download downloads a file. If a checksum file is provided will try and match
// the hash.
func Download(file, checksum string) error {
	var (
		client http.Client
		hash   string
		err    error
	)

	if checksum != "" {
		hash, err = downloadChecksum(checksum, file)
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
			sha256 := sha256.New()
			_, err = io.Copy(sha256, image)
			if err != nil {
				return err
			}

			result := fmt.Sprintf("%x", sha256.Sum(nil))
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

	_, err = lxd.DownloadFileSha256(&client, "", progress, nil, imagePath,
		file, hash, image)
	if err != nil {
		if checksum == "" && strings.HasPrefix(err.Error(), "Hash mismatch") {
			return nil
		}
		return err
	}

	fmt.Println("")

	return nil
}

func downloadChecksum(URL string, fname string) (string, error) {
	var client http.Client

	tempFile, err := ioutil.TempFile(os.TempDir(), "sha256.")
	if err != nil {
		return "", err
	}
	defer os.Remove(tempFile.Name())

	_, err = lxd.DownloadFileSha256(&client, "", nil, nil, "", URL, "", tempFile)
	// ignore hash mismatch
	if err != nil && !strings.HasPrefix(err.Error(), "Hash mismatch") {
		return "", err
	}

	tempFile.Seek(0, 0)

	scanner := bufio.NewScanner(tempFile)
	for scanner.Scan() {
		s := strings.Split(scanner.Text(), " ")
		//if len(s) == 2 && s[1] == fmt.Sprintf("*%s", filepath.Base(fname)) {
		matched, _ := regexp.MatchString(fmt.Sprintf(".*%s", filepath.Base(fname)), s[len(s)-1])
		if matched {
			return s[0], nil
		}
	}

	return "", fmt.Errorf("Could not find checksum")
}
