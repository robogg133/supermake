package main

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestPackIncludesOnlyStagedInstallation(t *testing.T) {
	work := t.TempDir()
	stage := filepath.Join(work, "install")
	bin := filepath.Join(stage, "usr", "bin")
	if err := os.MkdirAll(bin, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bin, "gcc"), []byte("compiler"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("gcc", filepath.Join(bin, "cc")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "source.c"), []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(work, "toolchain.tar.gz")
	if err := pack(stage, output); err != nil {
		t.Fatal(err)
	}
	f, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	r := tar.NewReader(gz)
	seen := map[string]bool{}
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		seen[h.Name] = true
		switch h.Name {
		case "usr", "usr/bin":
			if h.Typeflag != tar.TypeDir {
				t.Fatalf("not a directory: %s", h.Name)
			}
		case "usr/bin/cc":
			if h.Typeflag != tar.TypeSymlink || h.Linkname != "gcc" {
				t.Fatalf("invalid compiler symlink: %+v", h)
			}
		case "usr/bin/gcc":
			data, err := io.ReadAll(r)
			if err != nil || string(data) != "compiler" || h.Mode != 0755 {
				t.Fatalf("invalid compiler: %+v, %q, %v", h, data, err)
			}
		default:
			t.Fatalf("unexpected build artifact: %s", h.Name)
		}
	}
	if !seen["usr/bin/gcc"] || !seen["usr/bin/cc"] {
		t.Fatal("missing installed files")
	}
}
