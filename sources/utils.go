package sources

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	incus "github.com/lxc/incus/v6/shared/util"
)

// downloadChecksum downloads or opens URL, and matches fname against the
// checksums inside of the downloaded or opened file.
func downloadChecksum(ctx context.Context, client *http.Client, targetDir string, URL string, fname string, hashFunc hash.Hash, hashLen int) ([]string, error) {
	var (
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
		tempFile, err = os.CreateTemp(targetDir, "hash.")
		if err != nil {
			return nil, err
		}

		defer os.Remove(tempFile.Name())

		done := make(chan struct{})
		defer close(done)

		_, err = incus.DownloadFileHash(ctx, client, "distrobuilder", nil, nil, "", URL, "", hashFunc, tempFile)
		// ignore hash mismatch
		if err != nil && !strings.HasPrefix(err.Error(), "Hash mismatch") {
			return nil, err
		}
	}

	_, err = tempFile.Seek(0, 0)
	if err != nil {
		return nil, fmt.Errorf("Failed setting offset in file %q: %w", tempFile.Name(), err)
	}

	checksum := getChecksum(filepath.Base(fname), hashLen, tempFile)
	if checksum != nil {
		return checksum, nil
	}

	return nil, errors.New("Could not find checksum")
}

func getChecksum(fname string, hashLen int, r io.Reader) []string {
	scanner := bufio.NewScanner(r)

	var matches []string
	var result []string

	regex := regexp.MustCompile("[[:xdigit:]]+")

	for scanner.Scan() {
		if !strings.Contains(scanner.Text(), fname) {
			continue
		}

		for _, s := range strings.Split(scanner.Text(), " ") {
			if !regex.MatchString(s) {
				continue
			}

			if hashLen == 0 || hashLen == len(strings.TrimSpace(s)) {
				matches = append(matches, scanner.Text())
			}
		}
	}

	// Check common checksum file (pattern: "<hash> <filename>") with the exact filename
	for _, m := range matches {
		fields := strings.Split(m, " ")

		if strings.TrimSpace(fields[len(fields)-1]) == fname {
			result = append(result, strings.TrimSpace(fields[0]))
		}
	}

	if len(result) > 0 {
		return result
	}

	// Check common checksum file (pattern: "<hash> <filename>") which contains the filename
	for _, m := range matches {
		fields := strings.Split(m, " ")

		if strings.Contains(strings.TrimSpace(fields[len(fields)-1]), fname) {
			result = append(result, strings.TrimSpace(fields[0]))
		}
	}

	if len(result) > 0 {
		return result
	}

	// Special case: CentOS
	for _, m := range matches {
		for _, s := range strings.Split(m, " ") {
			if !regex.MatchString(s) {
				continue
			}

			if hashLen == 0 || hashLen == len(strings.TrimSpace(s)) {
				result = append(result, s)
			}
		}
	}

	if len(result) > 0 {
		return result
	}

	return nil
}

func gpgCommandContext(ctx context.Context, gpgDir string, args ...string) (cmd *exec.Cmd) {
	cmd = exec.CommandContext(ctx, "gpg", append([]string{"--homedir", gpgDir}, args...)...)
	cmd.Env = append(os.Environ(), "LANG=C.UTF-8")
	return cmd
}

func showFingerprint(ctx context.Context, gpgDir string, publicKey string) (fingerprint string, err error) {
	cmd := gpgCommandContext(ctx, gpgDir, "--show-keys")
	cmd.Stdin = strings.NewReader(publicKey)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stdout
	err = cmd.Run()
	if err != nil {
		err = fmt.Errorf("%s", stdout.String())
		return fingerprint, err
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if notFingerprint(line) {
			continue
		}

		fingerprint = line
		return fingerprint, err
	}

	if fingerprint == "" {
		err = fmt.Errorf("failed to get fingerprint from public key: %s, %v", publicKey, lines)
		return fingerprint, err
	}

	return fingerprint, err
}

func notFingerprint(line string) bool {
	return len(line) != 40 ||
		strings.HasPrefix(line, "/") ||
		strings.HasPrefix(line, "-") ||
		strings.HasPrefix(line, "pub") ||
		strings.HasPrefix(line, "sub") ||
		strings.HasPrefix(line, "uid")
}

func listFingerprints(ctx context.Context, gpgDir string) (fingerprints []string, err error) {
	cmd := gpgCommandContext(ctx, gpgDir, "--list-keys")
	var buffer bytes.Buffer
	cmd.Stdout = &buffer
	err = cmd.Run()
	if err != nil {
		return fingerprints, err
	}

	lines := strings.Split(buffer.String(), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if notFingerprint(line) {
			continue
		}

		fingerprints = append(fingerprints, line)
	}

	return fingerprints, err
}

func importPublicKeys(ctx context.Context, gpgDir string, publicKeys []string) error {
	fingerprints := make([]string, len(publicKeys))
	for i, f := range publicKeys {
		var err error
		fingerprints[i], err = showFingerprint(ctx, gpgDir, f)
		if err != nil {
			return err
		}

		cmd := gpgCommandContext(ctx, gpgDir, "--import")
		cmd.Stdin = strings.NewReader(f)

		var buffer bytes.Buffer
		cmd.Stderr = &buffer
		err = cmd.Run()
		if err != nil {
			err = fmt.Errorf("failed to run: %s: %s",
				strings.Join(cmd.Args, " "), strings.TrimSpace(buffer.String()))
			return err
		}
	}

	importedFingerprints, err := listFingerprints(ctx, gpgDir)
	if err != nil {
		return err
	}

	for i, fingerprint := range fingerprints {
		if !slices.Contains(importedFingerprints, fingerprint) {
			return fmt.Errorf("fingerprint %s of publickey %s not imported", fingerprint, publicKeys[i])
		}
	}

	return nil
}

func recvFingerprints(ctx context.Context, gpgDir string, keyserver string, fingerprints []string) error {
	args := []string{}
	if keyserver != "" {
		args = append(args, "--keyserver", keyserver)
		httpProxy := getEnvHttpProxy()
		if httpProxy != "" {
			args = append(args, "--keyserver-options",
				fmt.Sprintf("http-proxy=%s", httpProxy))
		}
	}

	args = append(args, "--recv-keys")
	cmd := gpgCommandContext(ctx, gpgDir, append(args, fingerprints...)...)
	var buffer bytes.Buffer
	cmd.Stderr = &buffer

	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("Failed to run: %s: %s", strings.Join(cmd.Args, " "), strings.TrimSpace(buffer.String()))
	}

	// Verify output
	var importedKeys []string
	var missingKeys []string
	lines := strings.Split(buffer.String(), "\n")

	for _, l := range lines {
		if strings.HasPrefix(l, "gpg: key ") && (strings.HasSuffix(l, " imported") || strings.HasSuffix(l, " not changed")) {
			key := strings.Split(l, " ")
			importedKeys = append(importedKeys, strings.Split(key[2], ":")[0])
		}
	}

	// Figure out which key(s) couldn't be imported
	if len(importedKeys) < len(fingerprints) {
		for _, j := range fingerprints {
			found := false
			for _, k := range importedKeys {
				if strings.HasSuffix(j, k) {
					found = true
				}
			}

			if !found {
				missingKeys = append(missingKeys, j)
			}
		}

		return fmt.Errorf("Failed to import keys: %s", strings.Join(missingKeys, " "))
	}

	return nil
}

func recvGPGKeys(ctx context.Context, gpgDir string, keyserver string, keys []string) (bool, error) {
	var fingerprints []string
	var publicKeys []string

	for _, k := range keys {
		if strings.HasPrefix(strings.TrimSpace(k), "-----BEGIN PGP PUBLIC KEY BLOCK-----") {
			publicKeys = append(publicKeys, strings.TrimSpace(k))
		} else {
			fingerprints = append(fingerprints, strings.TrimSpace(k))
		}
	}

	err := importPublicKeys(ctx, gpgDir, publicKeys)
	if err != nil {
		return false, err
	}

	err = recvFingerprints(ctx, gpgDir, keyserver, fingerprints)
	if err != nil {
		return false, err
	}

	return true, nil
}

func getEnvHttpProxy() (httpProxy string) {
	for _, key := range []string{
		"http_proxy",
		"HTTP_PROXY", "https_proxy", "HTTPS_PROXY",
	} {
		httpProxy = os.Getenv(key)
		if httpProxy != "" {
			return httpProxy
		}
	}

	return httpProxy
}
