package shared

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/sys/unix"
	"gopkg.in/flosch/pongo2.v3"
	yaml "gopkg.in/yaml.v2"
)

// EnvVariable represents a environment variable.
type EnvVariable struct {
	Value string
	Set   bool
}

// Environment represents a set of environment variables.
type Environment map[string]EnvVariable

// Copy copies a file.
func Copy(src, dest string) error {
	var err error

	srcFile, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("Failed to open file %q: %w", src, err)
	}
	defer srcFile.Close()

	destFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("Failed to create file %q: %w", dest, err)
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, srcFile)
	if err != nil {
		return fmt.Errorf("Failed to copy file: %w", err)
	}

	return destFile.Sync()
}

// RunCommand runs a command hereby setting the SHELL and PATH env variables,
// and redirecting the process's stdout and stderr to the real stdout and stderr
// respectively.
func RunCommand(ctx context.Context, name string, arg ...string) error {
	cmd := exec.CommandContext(ctx, name, arg...)

	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RunScript runs a script hereby setting the SHELL and PATH env variables,
// and redirecting the process's stdout and stderr to the real stdout and stderr
// respectively.
func RunScript(ctx context.Context, content string) error {
	fd, err := unix.MemfdCreate("tmp", 0)
	if err != nil {
		return fmt.Errorf("Failed to create memfd: %w", err)
	}
	defer unix.Close(fd)

	_, err = unix.Write(int(fd), []byte(content))
	if err != nil {
		return fmt.Errorf("Failed to write to memfd: %w", err)
	}

	fdPath := fmt.Sprintf("/proc/self/fd/%d", fd)

	return RunCommand(ctx, fdPath)
}

// Pack creates an uncompressed tarball.
func Pack(ctx context.Context, filename, compression, path string, args ...string) (string, error) {
	err := RunCommand(ctx, "tar", append([]string{"--xattrs", "-cf", filename, "-C", path}, args...)...)
	if err != nil {
		// Clean up incomplete tarball
		os.Remove(filename)
		return "", fmt.Errorf("Failed to create tarball: %w", err)
	}

	return compressTarball(ctx, filename, compression)
}

// PackUpdate updates an existing tarball.
func PackUpdate(ctx context.Context, filename, compression, path string, args ...string) (string, error) {
	err := RunCommand(ctx, "tar", append([]string{"--xattrs", "-uf", filename, "-C", path}, args...)...)
	if err != nil {
		return "", fmt.Errorf("Failed to update tarball: %w", err)
	}

	return compressTarball(ctx, filename, compression)
}

// compressTarball compresses a tarball, or not.
func compressTarball(ctx context.Context, filename, compression string) (string, error) {
	fileExtension := ""

	switch compression {
	case "lzop", "zstd":
		// Remove the uncompressed file as the compress fails to do so.
		defer os.Remove(filename)
		fallthrough
	case "bzip2", "xz", "lzip", "lzma", "gzip":
		err := RunCommand(ctx, compression, "-f", filename)
		if err != nil {
			return "", fmt.Errorf("Failed to compress tarball %q: %w", filename, err)
		}
	}

	switch compression {
	case "lzop":
		fileExtension = "lzo"
	case "zstd":
		fileExtension = "zst"
	case "bzip2":
		fileExtension = "bz2"
	case "xz":
		fileExtension = "xz"
	case "lzip":
		fileExtension = "lz"
	case "lzma":
		fileExtension = "lzma"
	case "gzip":
		fileExtension = "gz"
	}

	if fileExtension == "" {
		return filename, nil
	}

	return fmt.Sprintf("%s.%s", filename, fileExtension), nil
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
func RenderTemplate(template string, iface interface{}) (string, error) {
	// Serialize interface
	data, err := yaml.Marshal(iface)
	if err != nil {
		return "", err
	}

	// Decode document and write it to a pongo2 Context
	var ctx pongo2.Context
	yaml.Unmarshal(data, &ctx)

	// Load template from string
	tpl, err := pongo2.FromString("{% autoescape off %}" + template + "{% endautoescape %}")
	if err != nil {
		return "", err
	}

	// Get rendered template
	ret, err := tpl.Execute(ctx)
	if err != nil {
		return ret, err
	}

	// Looks like we're nesting templates so run pongo again
	if strings.Contains(ret, "{{") || strings.Contains(ret, "{%") {
		return RenderTemplate(ret, iface)
	}

	return ret, err
}

// SetEnvVariables sets the provided environment variables and returns the
// old ones.
func SetEnvVariables(env Environment) Environment {
	oldEnv := Environment{}

	for k, v := range env {
		// Check whether the env variables are set at the moment
		oldVal, set := os.LookupEnv(k)

		// Store old env variables
		oldEnv[k] = EnvVariable{
			Value: oldVal,
			Set:   set,
		}

		if v.Set {
			os.Setenv(k, v.Value)
		} else {
			os.Unsetenv(k)
		}
	}

	return oldEnv
}

// RsyncLocal copies src to dest using rsync.
func RsyncLocal(ctx context.Context, src string, dest string) error {
	err := RunCommand(ctx, "rsync", "-aHASX", "--devices", src, dest)
	if err != nil {
		return fmt.Errorf("Failed to copy %q to %q: %w", src, dest, err)
	}

	return nil
}

// Retry retries a function up to <attempts> times. This is especially useful for networking.
func Retry(f func() error, attempts uint) error {
	var err error

	for i := uint(0); i < attempts; i++ {
		err = f()
		if err == nil {
			break
		}

		time.Sleep(time.Second)
	}

	return err
}
