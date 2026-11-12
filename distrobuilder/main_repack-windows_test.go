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
