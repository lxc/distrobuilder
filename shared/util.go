package shared

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	lxd "github.com/lxc/lxd/shared"
)

// Copy copies a file.
func Copy(src, dest string) error {
	var err error

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return err
	}

	return destFile.Sync()
}

// RunCommand runs a command hereby setting the SHELL and PATH env variables,
// and redirecting the process's stdout and stderr to the real stdout and stderr
// respectively.
func RunCommand(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VerifyFile verifies a file using gpg.
func VerifyFile(signedFile, signatureFile string, keys []string) (bool, error) {
	var out string

	gpgDir := filepath.Join(os.TempDir(), "distrobuilder.gpg")

	err := os.MkdirAll(gpgDir, 0700)
	if err != nil {
		return false, err
	}
	defer os.RemoveAll(gpgDir)

	out, err = lxd.RunCommand("gpg", append([]string{"--homedir", gpgDir, "--recv-keys"}, keys...)...)
	if err != nil {
		return false, fmt.Errorf("Failed to receive keys: %s", out)
	}

	if signatureFile != "" {
		out, err = lxd.RunCommand("gpg", "--homedir", gpgDir, "--verify", signatureFile, signedFile)
		if err != nil {
			return false, fmt.Errorf("Failed to verify: %s", out)
		}
	} else {
		out, err = lxd.RunCommand("gpg", "--homedir", gpgDir, "--verify", signedFile)
		if err != nil {
			return false, fmt.Errorf("Failed to verify: %s", out)
		}
	}

	return true, nil
}
