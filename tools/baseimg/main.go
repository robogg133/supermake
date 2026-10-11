package main

import (
	"archive/tar"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/schollz/progressbar/v3"
)

func main() {
	busyboxURL := flag.String("busybox", "https://busybox.net/downloads/binaries/1.35.0-x86_64-linux-musl/busybox", "Busybox binary URL")
	muslURL := flag.String("musl", "https://musl.libc.org/releases/musl-1.2.6.tar.gz", "musl source archive URL")
	makeURL := flag.String("make", "https://ftp.gnu.org/gnu/make/make-4.4.tar.gz", "GNU Make source archive URL")
	arch := flag.String("arch", runtime.GOARCH, "image architecture (native builds only)")
	flag.Parse()
	if *arch != runtime.GOARCH {
		panic("cross-compilation is not supported")
	}
	check := func(err error) {
		if err != nil {
			panic(err)
		}
	}
	tmpDir, err := os.MkdirTemp("", "baseimg-")
	check(err)
	defer os.RemoveAll(tmpDir)

	busyboxPath := filepath.Join(tmpDir, "busybox")
	check(download(*busyboxURL, busyboxPath))
	check(os.Chmod(busyboxPath, 0755))

	stage := filepath.Join(tmpDir, "root")
	prefix := filepath.Join(stage, "usr")
	jobs := fmt.Sprintf("-j%d", runtime.NumCPU())
	for _, source := range []struct {
		name, url string
	}{
		{"musl", *muslURL},
		{"make", *makeURL},
	} {
		archive := filepath.Join(tmpDir, source.name+".tar.gz")
		check(download(source.url, archive))
		dir := filepath.Join(tmpDir, source.name)
		check(os.Mkdir(dir, 0755))
		check(run(tmpDir, "tar", "-xzf", archive, "--strip-components=1", "-C", dir))
		if source.name == "musl" {
			check(run(dir, "./configure", "--prefix="+prefix, "--syslibdir="+filepath.Join(prefix, "lib")))
			check(run(dir, "make", jobs))
			check(run(dir, "make", "install"))
		} else {
			// Static linking keeps Make independent of the host's libc and loader.
			check(run(dir, "./configure", "--prefix=/usr", "CC="+filepath.Join(prefix, "bin", "musl-gcc"), "LDFLAGS=-static"))
			check(run(dir, "make", jobs))
			check(run(dir, "make", "install", "DESTDIR="+stage))
		}
	}

	f, err := os.Create(fmt.Sprintf("%s-baseimg.tar", *arch))
	check(err)
	defer f.Close()
	w := tar.NewWriter(f)
	defer w.Close()
	for _, dir := range []string{"/usr/", "/usr/bin/", "/proc"} {
		check(w.WriteHeader(&tar.Header{Name: dir, Typeflag: tar.TypeDir, Mode: 0755}))
	}
	for _, link := range []struct{ name, target string }{
		{"/bin", "/usr/bin"},
		{"/usr/sbin", "/usr/bin"},
		{"/sbin", "/usr/sbin"},
		{"/lib", "/usr/lib"},
		{"/usr/lib64", "/usr/lib"},
	} {
		check(w.WriteHeader(&tar.Header{Name: link.name, Linkname: link.target, Typeflag: tar.TypeSymlink, Mode: 0755}))
	}
	check(addFile(w, busyboxPath, "/usr/bin/busybox", stage))
	applets, err := exec.Command(busyboxPath, "--list").Output()
	check(err)
	for _, applet := range strings.Fields(string(applets)) {
		if applet == "make" || applet == "busybox" {
			continue
		}
		check(w.WriteHeader(&tar.Header{Name: "/usr/bin/" + applet, Linkname: "/usr/bin/busybox", Typeflag: tar.TypeSymlink, Mode: 0755}))
	}
	check(addMuslFiles(w, stage))
	check(addFile(w, filepath.Join(prefix, "bin", "make"), "/usr/bin/make", stage))
	check(w.Close())
	check(f.Close())
}

func addMuslFiles(w *tar.Writer, stage string) error {
	for _, dir := range []string{"lib", "include"} {
		if err := filepath.WalkDir(filepath.Join(stage, "usr", dir), func(path string, _ os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(stage, path)
			if err != nil {
				return err
			}
			return addFile(w, path, "/"+filepath.ToSlash(rel), stage)
		}); err != nil {
			return err
		}
	}
	return nil
}

func download(url, path string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", url, resp.Status)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	reader := progressbar.NewReader(resp.Body, progressbar.Default(resp.ContentLength, filepath.Base(path)))
	if _, err := io.Copy(f, &reader); err != nil {
		return err
	}
	return f.Close()
}

func run(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func addFile(w *tar.Writer, path, name, stage string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	var target string
	if info.Mode()&os.ModeSymlink != 0 {
		target, err = os.Readlink(path)
		if err != nil {
			return err
		}
		// Installed musl loader links must point inside the image, not the build directory.
		if strings.HasPrefix(target, stage+string(filepath.Separator)) {
			target = filepath.ToSlash(strings.TrimPrefix(target, stage))
		}
	}
	header, err := tar.FileInfoHeader(info, target)
	if err != nil {
		return err
	}
	header.Name = name
	if err := w.WriteHeader(header); err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(w, f)
	return err
}
