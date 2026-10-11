package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func main() {
	if err := bootstrap(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func bootstrap() error {
	binutilsURL := flag.String("binutils", "https://snapshots.sourceware.org/binutils/trunk/2026-10-11_07-26_1791703561/src/binutils-with-gold-2.47.50-ced7bfccaa6.tar.xz", "binutils source URL")
	gccURL := flag.String("gcc", "https://mirror.hostarts.dz/gcc/releases/gcc-16.2.0/gcc-16.2.0.tar.gz", "GCC source URL")
	target := flag.String("target", "", "musl target triplet (default: native architecture)")
	cc := flag.String("cc", "", "musl C compiler (default: <target>-gcc)")
	cxx := flag.String("cxx", "", "musl C++ compiler (default: <target>-g++)")
	sysroot := flag.String("sysroot", "", "required musl sysroot with /usr/include and /usr/lib")
	output := flag.String("output", "toolchain.tar.gz", "installation archive")
	jobs := flag.Int("jobs", runtime.NumCPU(), "parallel build jobs")
	flag.Parse()
	if *target == "" {
		switch runtime.GOARCH {
		case "amd64":
			*target = "x86_64-linux-musl"
		case "arm64":
			*target = "aarch64-linux-musl"
		default:
			return fmt.Errorf("specify -target for architecture %s", runtime.GOARCH)
		}
	}
	if !strings.Contains(*target, "linux-musl") || *jobs < 1 {
		return fmt.Errorf("a linux-musl target and positive -jobs are required")
	}
	if *cc == "" {
		*cc = *target + "-gcc"
	}
	if *cxx == "" {
		*cxx = *target + "-g++"
	}
	if *sysroot == "" {
		return fmt.Errorf("-sysroot is required: baseimg alone lacks musl development headers; use a complete musl sysroot")
	}
	root, err := filepath.Abs(*sysroot)
	if err != nil {
		return err
	}
	for _, path := range []string{"usr/include/stdio.h", "usr/lib/libc.a"} {
		if _, err := os.Stat(filepath.Join(root, path)); err != nil {
			return fmt.Errorf("incomplete musl sysroot: %w", err)
		}
	}
	for _, tool := range []string{*cc, *cxx, "make", "tar", "sh"} {
		if _, err := exec.LookPath(tool); err != nil {
			return fmt.Errorf("required build tool %s: %w", tool, err)
		}
	}
	work, err := os.MkdirTemp("", "bootstrap-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(work)
	stage := filepath.Join(work, "install")
	env := append(os.Environ(), "CC="+*cc, "CXX="+*cxx,
		"CFLAGS=-O2", "CXXFLAGS=-O2", "LDFLAGS=-static")
	for _, source := range []struct{ name, url string }{
		{"binutils", *binutilsURL}, {"gcc", *gccURL},
	} {
		archive := filepath.Join(work, source.name+".tar")
		if err := download(source.url, archive); err != nil {
			return err
		}
		src := filepath.Join(work, source.name)
		build := filepath.Join(work, source.name+"-build")
		for _, dir := range []string{src, build} {
			if err := os.Mkdir(dir, 0755); err != nil {
				return err
			}
		}
		if err := run(work, nil, "tar", "-xf", archive, "--strip-components=1", "-C", src); err != nil {
			return err
		}
		if source.name == "gcc" {
			// GCC's bundled script downloads its pinned GMP, MPFR, MPC and ISL sources.
			if err := run(src, nil, "sh", "contrib/download_prerequisites"); err != nil {
				return err
			}
		}
		guess := exec.Command("sh", filepath.Join(src, "config.guess"))
		buildTriplet, err := guess.Output()
		if err != nil {
			return fmt.Errorf("%s config.guess: %w", source.name, err)
		}
		args := []string{"--prefix=/usr", "--libdir=/usr/lib", "--build=" + strings.TrimSpace(string(buildTriplet)),
			"--host=" + *target, "--target=" + *target, "--disable-nls", "--disable-shared"}
		if source.name == "binutils" {
			args = append(args, "--with-sysroot=/", "--disable-gprofng", "--disable-werror")
		} else {
			args = append(args, "--with-sysroot=/", "--with-build-sysroot="+root,
				"--disable-bootstrap", "--disable-multilib", "--enable-languages=c,c++",
				"--disable-libsanitizer", "--disable-libquadmath", "--disable-libgomp")
		}
		if err := run(build, env, filepath.Join(src, "configure"), args...); err != nil {
			return err
		}
		if err := run(build, env, "make", fmt.Sprintf("-j%d", *jobs)); err != nil {
			return err
		}
		if err := run(build, env, "make", "install", "DESTDIR="+stage); err != nil {
			return err
		}
	}
	return pack(stage, *output)
}

func download(url, path string) error {
	fmt.Println("Downloading", url)
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
	if _, err := io.Copy(f, resp.Body); err != nil {
		return err
	}
	return f.Close()
}

func run(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Dir, cmd.Env = dir, env
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return nil
}

func pack(stage, output string) error {
	// Rename only after success so a failed archive does not replace an existing one.
	f, err := os.CreateTemp(filepath.Dir(output), ".toolchain-*.tar.gz")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	w := tar.NewWriter(gz)
	defer w.Close()
	if err := filepath.WalkDir(stage, func(path string, _ os.DirEntry, err error) error {
		if err != nil || path == stage {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		var link string
		if info.Mode()&os.ModeSymlink != 0 {
			link, err = os.Readlink(path)
			if err != nil {
				return err
			}
		}
		h, err := tar.FileInfoHeader(info, link)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		h.Name = filepath.ToSlash(rel)
		h.Uid, h.Gid, h.Uname, h.Gname = 0, 0, "", ""
		if err := w.WriteHeader(h); err != nil || !info.Mode().IsRegular() {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		_, err = io.Copy(w, in)
		return err
	}); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	if err := gz.Close(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), output)
}
