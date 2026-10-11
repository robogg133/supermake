package main

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestAddMuslFiles(t *testing.T) {
	stage := t.TempDir()
	files := map[string]string{
		"usr/include/stdio.h":         "stdio header",
		"usr/include/bits/alltypes.h": "architecture types",
		"usr/lib/libc.a":              "static libc",
		"usr/lib/crt1.o":              "startup object",
	}
	for name, contents := range files {
		path := filepath.Join(stage, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0644); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	if err := addMuslFiles(w, stage); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := tar.NewReader(&buf)
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if h.Typeflag == tar.TypeDir {
			continue
		}
		name := h.Name[1:]
		want, ok := files[name]
		data, err := io.ReadAll(r)
		if !ok || err != nil || string(data) != want || h.Mode != 0644 {
			t.Fatalf("unexpected archive entry: %+v, %q, %v", h, data, err)
		}
		delete(files, name)
	}
	if len(files) != 0 {
		t.Fatalf("missing musl files: %v", files)
	}
}

func TestAddFile(t *testing.T) {
	stage := t.TempDir()
	library := filepath.Join(stage, "libc.so")
	if err := os.WriteFile(library, []byte("library"), 0755); err != nil {
		t.Fatal(err)
	}
	loader := filepath.Join(stage, "ld-musl.so")
	if err := os.Symlink(library, loader); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	w := tar.NewWriter(&buf)
	for _, entry := range []struct{ path, name string }{
		{library, "/usr/lib/libc.so"},
		{loader, "/usr/lib/ld-musl.so"},
	} {
		if err := addFile(w, entry.path, entry.name, stage); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	r := tar.NewReader(&buf)
	h, err := r.Next()
	if err != nil || h.Name != "/usr/lib/libc.so" || h.Size != 7 {
		t.Fatalf("invalid library header: %v, %v", h, err)
	}
	data, err := io.ReadAll(r)
	if err != nil || string(data) != "library" {
		t.Fatalf("invalid library contents: %q, %v", data, err)
	}
	h, err = r.Next()
	if err != nil || h.Typeflag != tar.TypeSymlink || h.Linkname != "/libc.so" {
		t.Fatalf("invalid loader link: %v, %v", h, err)
	}
}
