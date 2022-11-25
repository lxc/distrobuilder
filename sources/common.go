package sources

import (
	"context"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/lxc/distrobuilder/shared"
	lxd "github.com/lxc/lxd/shared"
	"github.com/lxc/lxd/shared/ioprogress"
	"github.com/sirupsen/logrus"
)

type common struct {
	logger     *logrus.Logger
	definition shared.Definition
	rootfsDir  string
	cacheDir   string
	sourcesDir string
	ctx        context.Context
}

func (s *common) init(ctx context.Context, logger *logrus.Logger, definition shared.Definition, rootfsDir string, cacheDir string, sourcesDir string) {
	s.logger = logger
	s.definition = definition
	s.rootfsDir = rootfsDir
	s.cacheDir = cacheDir
	s.sourcesDir = sourcesDir
	s.ctx = ctx
}

func (s *common) getTargetDir() string {
	dir := filepath.Join(s.sourcesDir, fmt.Sprintf("%s-%s-%s", s.definition.Image.Distribution, s.definition.Image.Release, s.definition.Image.ArchitectureMapped))
	dir = strings.Replace(dir, " ", "", -1)
	dir = strings.ToLower(dir)

	return dir
}

// DownloadHash downloads a file. If a checksum file is provided, it will try and
// match the hash.
func (s *common) DownloadHash(def shared.DefinitionImage, file, checksum string, hashFunc hash.Hash) (string, error) {
	var (
		client http.Client
		hashes []string
		err    error
	)

	destDir := s.getTargetDir()

	err = os.MkdirAll(destDir, 0755)
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

		err := shared.Retry(func() error {
			hashes, err = downloadChecksum(s.ctx, destDir, checksum, file, hashFunc, hashLen)
			return err
		}, 3)
		if err != nil {
			return "", fmt.Errorf("Error while downloading checksum: %w", err)
		}
	}

	imagePath := filepath.Join(destDir, filepath.Base(file))

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

		return destDir, nil
	}

	image, err := os.Create(imagePath)
	if err != nil {
		return "", err
	}
	defer image.Close()

	progress := func(progress ioprogress.ProgressData) {
		fmt.Printf("%s\r", progress.Text)
	}

	done := make(chan struct{})
	defer close(done)

	if checksum == "" {
		err = shared.Retry(func() error {
			_, err = lxd.DownloadFileHash(s.ctx, &client, "distrobuilder", progress, nil, imagePath, file, "", nil, image)
			if err != nil {
				os.Remove(imagePath)
			}

			return err
		}, 3)
	} else {
		// Check all file hashes in case multiple have been provided.
		err = shared.Retry(func() error {
			for _, h := range hashes {
				if hashFunc != nil {
					hashFunc.Reset()
				}

				_, err = lxd.DownloadFileHash(s.ctx, &client, "distrobuilder", progress, nil, imagePath, file, h, hashFunc, image)
				if err == nil {
					break
				}
			}
			if err != nil {
				os.Remove(imagePath)
			}

			return err
		}, 3)
	}
	if err != nil {
		return "", err
	}

	fmt.Println("")

	return destDir, nil
}

// GetSignedContent verifies the provided file, and returns its decrypted (plain) content.
func (s *common) GetSignedContent(signedFile string) ([]byte, error) {
	keyring, err := s.CreateGPGKeyring()
	if err != nil {
		return nil, err
	}

	gpgDir := path.Dir(keyring)
	defer os.RemoveAll(gpgDir)

	out, err := exec.Command("gpg", "--homedir", gpgDir, "--keyring", keyring,
		"--decrypt", signedFile).Output()
	if err != nil {
		return nil, fmt.Errorf("Failed to get file content: %s: %w", out, err)
	}

	return out, nil
}

// VerifyFile verifies a file using gpg.
func (s *common) VerifyFile(signedFile, signatureFile string) (bool, error) {
	keyring, err := s.CreateGPGKeyring()
	if err != nil {
		return false, err
	}
	gpgDir := path.Dir(keyring)
	defer os.RemoveAll(gpgDir)

	var out strings.Builder

	if signatureFile != "" {
		err := shared.RunCommand(s.ctx, nil, &out, "gpg", "--homedir", gpgDir, "--keyring", keyring,
			"--verify", signatureFile, signedFile)
		if err != nil {
			return false, fmt.Errorf("Failed to verify: %s: %w", out.String(), err)
		}
	} else {
		err := shared.RunCommand(s.ctx, nil, &out, "gpg", "--homedir", gpgDir, "--keyring", keyring,
			"--verify", signedFile)
		if err != nil {
			return false, fmt.Errorf("Failed to verify: %s: %w", out.String(), err)
		}
	}

	return true, nil
}

// CreateGPGKeyring creates a new GPG keyring.
func (s *common) CreateGPGKeyring() (string, error) {
	err := os.MkdirAll(s.getTargetDir(), 0700)
	if err != nil {
		return "", err
	}

	gpgDir, err := os.MkdirTemp(s.getTargetDir(), "gpg.")
	if err != nil {
		return "", fmt.Errorf("Failed to create gpg directory: %w", err)
	}

	err = os.MkdirAll(gpgDir, 0700)
	if err != nil {
		return "", err
	}

	var ok bool

	for i := 0; i < 3; i++ {
		ok, err = recvGPGKeys(s.ctx, gpgDir, s.definition.Source.Keyserver, s.definition.Source.Keys)
		if ok {
			break
		}

		time.Sleep(2 * time.Second)
	}

	if !ok {
		return "", err
	}

	var out strings.Builder

	// Export keys to support gpg1 and gpg2
	err = shared.RunCommand(s.ctx, nil, &out, "gpg", "--homedir", gpgDir, "--export", "--output",
		filepath.Join(gpgDir, "distrobuilder.gpg"))
	if err != nil {
		os.RemoveAll(gpgDir)
		return "", fmt.Errorf("Failed to export keyring: %s: %w", out.String(), err)
	}

	return filepath.Join(gpgDir, "distrobuilder.gpg"), nil
}
