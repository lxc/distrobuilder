package shared

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	lxd "github.com/lxc/lxd/shared"
	"gopkg.in/flosch/pongo2.v3"
)

// EnvVariable represents a environment variable.
type EnvVariable struct {
	Key   string
	Value string
	Set   bool
}

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

// RunScript runs a script hereby setting the SHELL and PATH env variables,
// and redirecting the process's stdout and stderr to the real stdout and stderr
// respectively.
func RunScript(content string) error {
	cmd := exec.Command("sh")

	cmd.Stdin = bytes.NewBufferString(content)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// VerifyFile verifies a file using gpg.
func VerifyFile(signedFile, signatureFile string, keys []string, keyserver string) (bool, error) {
	keyring, err := CreateGPGKeyring(keyserver, keys)
	if err != nil {
		return false, err
	}
	gpgDir := path.Dir(keyring)
	defer os.RemoveAll(gpgDir)

	if signatureFile != "" {
		out, err := lxd.RunCommand("gpg", "--homedir", gpgDir, "--keyring", keyring,
			"--verify", signatureFile, signedFile)
		if err != nil {
			return false, fmt.Errorf("Failed to verify: %s", out)
		}
	} else {
		out, err := lxd.RunCommand("gpg", "--homedir", gpgDir, "--keyring", keyring,
			"--verify", signedFile)
		if err != nil {
			return false, fmt.Errorf("Failed to verify: %s", out)
		}
	}

	return true, nil
}

// CreateGPGKeyring creates a new GPG keyring.
func CreateGPGKeyring(keyserver string, keys []string) (string, error) {
	gpgDir, err := ioutil.TempDir(os.TempDir(), "distrobuilder.")
	if err != nil {
		return "", fmt.Errorf("Failed to create gpg directory: %s", err)
	}

	err = os.MkdirAll(gpgDir, 0700)
	if err != nil {
		return "", err
	}

	args := []string{"--homedir", gpgDir}

	if keyserver != "" {
		args = append(args, "--keyserver", keyserver)
	}

	args = append(args, append([]string{"--recv-keys"}, keys...)...)

	out, err := lxd.RunCommand("gpg", args...)
	if err != nil {
		os.RemoveAll(gpgDir)
		return "", fmt.Errorf("Failed to create keyring: %s", out)
	}

	// Export keys to support gpg1 and gpg2
	out, err = lxd.RunCommand("gpg", "--homedir", gpgDir, "--export", "--output",
		filepath.Join(gpgDir, "distrobuilder.gpg"))
	if err != nil {
		os.RemoveAll(gpgDir)
		return "", fmt.Errorf("Failed to export keyring: %s", out)
	}

	return filepath.Join(gpgDir, "distrobuilder.gpg"), nil
}

// Pack creates an uncompressed tarball.
func Pack(filename, compression, path string, args ...string) error {
	err := RunCommand("tar", append([]string{"-cf", filename, "-C", path}, args...)...)
	if err != nil {
		return err
	}

	return compressTarball(filename, compression)
}

// PackUpdate updates an existing tarball.
func PackUpdate(filename, compression, path string, args ...string) error {
	err := RunCommand("tar", append([]string{"-uf", filename, "-C", path}, args...)...)
	if err != nil {
		return err
	}

	return compressTarball(filename, compression)
}

// compressTarball compresses a tarball, or not.
func compressTarball(filename, compression string) error {
	switch compression {
	case "lzop":
		// lzo does not remove the uncompressed file per default
		defer os.Remove(filename)
		fallthrough
	case "bzip2", "xz", "lzip", "lzma", "gzip":
		return RunCommand(compression, "-f", filename)
	}

	// Do not compress
	return nil
}

//GetExpiryDate returns an expiry date based on the creationDate and format.
func GetExpiryDate(creationDate time.Time, format string) time.Time {
	regex := regexp.MustCompile(`(?:(\d+)(s|m|h|d|w))*`)
	expiryDate := creationDate

	for _, match := range regex.FindAllStringSubmatch(format, -1) {
		// Ignore empty matches
		if match[0] == "" {
			continue
		}

		var duration time.Duration

		switch match[2] {
		case "s":
			duration = time.Second
		case "m":
			duration = time.Minute
		case "h":
			duration = time.Hour
		case "d":
			duration = 24 * time.Hour
		case "w":
			duration = 7 * 24 * time.Hour
		}

		// Ignore any error since it will be an integer.
		value, _ := strconv.Atoi(match[1])
		expiryDate = expiryDate.Add(time.Duration(value) * duration)
	}

	return expiryDate
}

// RenderTemplate renders a pongo2 template.
func RenderTemplate(template string, ctx pongo2.Context) (string, error) {
	var (
		err error
		ret string
	)

	// Load template from string
	tpl, err := pongo2.FromString("{% autoescape off %}" + template + "{% endautoescape %}")
	if err != nil {
		return ret, err
	}

	// Get rendered template
	ret, err = tpl.Execute(ctx)
	if err != nil {
		return ret, err
	}

	// Looks like we're nesting templates so run pongo again
	if strings.Contains(ret, "{{") || strings.Contains(ret, "{%") {
		return RenderTemplate(ret, ctx)
	}

	return ret, err
}

// SetEnvVariables sets the provided environment variables and returns the
// old ones.
func SetEnvVariables(env []EnvVariable) []EnvVariable {
	oldEnv := make([]EnvVariable, len(env))

	for i := 0; i < len(env); i++ {
		// Check whether the env variables are set at the moment
		oldVal, set := os.LookupEnv(env[i].Key)

		// Store old env variables
		oldEnv[i].Key = env[i].Key
		oldEnv[i].Value = oldVal
		oldEnv[i].Set = set

		if env[i].Set {
			os.Setenv(env[i].Key, env[i].Value)
		} else {
			os.Unsetenv(env[i].Key)
		}
	}

	return oldEnv
}
