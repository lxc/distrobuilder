package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestParseAddDir(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		src     string
		dst     string
		wantErr bool
	}{
		{name: "source only", spec: "/tmp/local", src: "/tmp/local", dst: ""},
		{name: "with destination", spec: "/tmp/local:local", src: "/tmp/local", dst: "local"},
		{name: "nested destination", spec: "/tmp/x:boot/local", src: "/tmp/x", dst: "boot/local"},
		{name: "empty source", spec: ":local", wantErr: true},
		{name: "absolute destination", spec: "/tmp/x:/etc", wantErr: true},
		{name: "parent escape", spec: "/tmp/x:..", wantErr: true},
		{name: "parent escape nested", spec: "/tmp/x:../etc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src, dst, err := parseAddDir(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.spec)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if src != tt.src || dst != tt.dst {
				t.Fatalf("got (%q, %q), want (%q, %q)", src, dst, tt.src, tt.dst)
			}
		})
	}
}

func TestInjectFiles(t *testing.T) {
	overlay := t.TempDir()
	srcDir := t.TempDir()

	err := os.WriteFile(filepath.Join(srcDir, "install.bat"), []byte("echo hi\n"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	unattend := filepath.Join(t.TempDir(), "custom.xml")
	err = os.WriteFile(unattend, []byte("<unattend/>"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	c := &cmdRepackWindows{
		global:       &cmdGlobal{logger: logrus.New()},
		flagUnattend: unattend,
		flagAddDirs:  []string{srcDir + ":local"},
	}

	err = c.injectFiles(overlay)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(overlay, "Autounattend.xml"))
	if err != nil {
		t.Fatalf("unattend not copied: %v", err)
	}

	if string(data) != "<unattend/>" {
		t.Fatalf("unattend content mismatch: %q", data)
	}

	data, err = os.ReadFile(filepath.Join(overlay, "local", "install.bat"))
	if err != nil {
		t.Fatalf("add-dir content not copied: %v", err)
	}

	if string(data) != "echo hi\n" {
		t.Fatalf("add-dir content mismatch: %q", data)
	}

	// Overwrite semantics: a second inject with different content wins.
	err = os.WriteFile(unattend, []byte("<unattend>2</unattend>"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = c.injectFiles(overlay)
	if err != nil {
		t.Fatal(err)
	}

	data, _ = os.ReadFile(filepath.Join(overlay, "Autounattend.xml"))
	if string(data) != "<unattend>2</unattend>" {
		t.Fatalf("overwrite failed: %q", data)
	}
}

func TestParseAddDriversDir(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		subpath string
		dst     string
		wantErr bool
	}{
		{name: "directory defaults to base", spec: "guest-agent", subpath: "guest-agent", dst: "guest-agent"},
		{name: "file defaults to base", spec: "virtio-win-gt-x64.msi", subpath: "virtio-win-gt-x64.msi", dst: "virtio-win-gt-x64.msi"},
		{name: "explicit destination", spec: "guest-agent:tools/guest-agent", subpath: "guest-agent", dst: "tools/guest-agent"},
		{name: "nested source", spec: "sub/dir", subpath: "sub/dir", dst: "dir"},
		{name: "empty", spec: "", wantErr: true},
		{name: "root", spec: ".", wantErr: true},
		{name: "absolute source", spec: "/etc", wantErr: true},
		{name: "escape source", spec: "../..", wantErr: true},
		{name: "absolute destination", spec: "guest-agent:/etc", wantErr: true},
		{name: "escaping destination", spec: "guest-agent:../etc", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subpath, dst, err := parseAddDriversDir(tt.spec)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error for %q", tt.spec)
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if subpath != tt.subpath || dst != tt.dst {
				t.Fatalf("got (%q, %q), want (%q, %q)", subpath, dst, tt.subpath, tt.dst)
			}
		})
	}
}

func TestInjectFilesFromDriversISO(t *testing.T) {
	cacheDir := t.TempDir()
	driversDir := filepath.Join(cacheDir, "drivers")
	err := os.MkdirAll(filepath.Join(driversDir, "guest-agent"), 0o755)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(driversDir, "guest-agent", "qemu-ga-x86_64.msi"), []byte("msi"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	err = os.WriteFile(filepath.Join(driversDir, "virtio-win-gt-x64.msi"), []byte("gt"), 0o644)
	if err != nil {
		t.Fatal(err)
	}

	overlay := t.TempDir()
	c := &cmdRepackWindows{
		global:             &cmdGlobal{logger: logrus.New(), flagCacheDir: cacheDir},
		flagAddDriversDirs: []string{"guest-agent", "virtio-win-gt-x64.msi"},
	}

	err = c.injectFiles(overlay)
	if err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(overlay, "guest-agent", "qemu-ga-x86_64.msi"))
	if err != nil {
		t.Fatalf("guest-agent not copied: %v", err)
	}

	if string(data) != "msi" {
		t.Fatalf("guest-agent content mismatch: %q", data)
	}

	data, err = os.ReadFile(filepath.Join(overlay, "virtio-win-gt-x64.msi"))
	if err != nil {
		t.Fatalf("guest tools msi not copied: %v", err)
	}

	if string(data) != "gt" {
		t.Fatalf("guest tools msi content mismatch: %q", data)
	}

	// A missing subpath is reported, not silently skipped.
	c.flagAddDriversDirs = []string{"nope"}
	err = c.injectFiles(overlay)
	if err == nil {
		t.Fatal("expected an error for a missing subpath")
	}
}
