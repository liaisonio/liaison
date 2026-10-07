// Package webide manages IDE installations independently of Agent runtimes.
package webide

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const Release = "4.139.1"
const archiveLimit int64 = 512 << 20
const unpackLimit int64 = 2 << 30

// Installer is the installation dependency consumed by the runtime. It has no
// knowledge of Manager identity, database models, browser sessions or billing.
type Installer interface {
	Install(context.Context, string) (string, error)
}

// PackageInstaller accepts only the pinned release. An operator can stage its
// official archive at <package-root>/offline/<package-name>.tar.gz. A present
// but invalid offline archive fails closed; it never silently downloads instead.
type PackageInstaller struct{}

func (PackageInstaller) Install(ctx context.Context, root string) (string, error) {
	cache := filepath.Join(root, "offline", packageName()+".tar.gz")
	info, err := os.Lstat(cache)
	if errors.Is(err, os.ErrNotExist) {
		return Install(ctx, root)
	}
	if err != nil {
		return "", fmt.Errorf("inspect offline package: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > archiveLimit {
		return "", errors.New("invalid offline package")
	}
	f, err := os.Open(cache)
	if err != nil {
		return "", fmt.Errorf("open offline package: %w", err)
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return "", errors.New("offline package changed while opening")
	}
	return InstallOffline(ctx, root, f)
}

// InstallOffline applies exactly the same pinned digest and extraction limits
// as online installation. Callers cannot supply a version, digest or target name.
func InstallOffline(ctx context.Context, root string, archive io.Reader) (string, error) {
	digest, supported := releaseDigests[platform()]
	if !supported {
		return "", errors.New("unsupported platform")
	}
	if archive == nil {
		return "", errors.New("offline archive is required")
	}
	return installArchive(ctx, root, packageName(), digest, archive)
}

// Pinned release digests from the upstream GitHub release asset API. Browser
// requests cannot supply a download URL, digest, version or executable path.
var releaseDigests = map[string]string{
	"linux-amd64": "53029be6c5781b7bca49b815fcc9a2a3fc111813ad8c9965b2c0f0d2985a0674",
	"linux-arm64": "0edb4b60d9c4744b2dd14b0911e3c2e6dd8c6f3c13bd58bda23ae744e59e7df1",
	"macos-amd64": "7b3e644460cdc08027d5d305f04117bffd554b6e7983eca568fe3e8048917670",
	"macos-arm64": "be45844038d9c48f012e8a716765dc189432100b6fff801c93ab75ef36dbe8e6",
}

func platform() string {
	osName := runtime.GOOS
	if osName == "darwin" {
		osName = "macos"
	}
	return osName + "-" + runtime.GOARCH
}
func packageName() string { return "code-server-" + Release + "-" + platform() }

// Install verifies before extraction and publishes by rename. An unsuccessful
// operation cannot replace an existing installation. No package scripts run.
func Install(ctx context.Context, root string) (string, error) {
	digest, supported := releaseDigests[platform()]
	if !supported {
		return "", errors.New("unsupported platform")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	url := "https://github.com/coder/code-server/releases/download/v" + Release + "/" + packageName() + ".tar.gz"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}
	client := &http.Client{Timeout: 10 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || req.URL.Scheme != "https" {
			return errors.New("unsafe download redirect")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("download IDE: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("IDE download status %d", resp.StatusCode)
	}
	return installArchive(ctx, root, packageName(), digest, resp.Body)
}

func installArchive(ctx context.Context, root, name, digest string, src io.Reader) (string, error) {
	if !filepath.IsAbs(root) || filepath.Base(name) != name || name == "." || name == ".." {
		return "", errors.New("invalid installation path")
	}
	if len(digest) != 64 {
		return "", errors.New("invalid package digest")
	}
	if _, err := hex.DecodeString(digest); err != nil {
		return "", errors.New("invalid package digest")
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	root = canonical
	target := filepath.Join(root, name)
	if _, err := os.Lstat(target); err == nil {
		return "", errors.New("installation already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	stage, err := os.MkdirTemp(root, ".install-")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(stage) // Only this operation's freshly allocated staging directory.
	archive, err := os.OpenFile(filepath.Join(stage, "package.tar.gz"), os.O_RDWR|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return "", err
	}
	defer archive.Close()
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(archive, h), io.LimitReader(&contextReader{ctx, src}, archiveLimit+1))
	if err != nil {
		return "", err
	}
	if n > archiveLimit {
		return "", errors.New("package too large")
	}
	if hex.EncodeToString(h.Sum(nil)) != digest {
		return "", errors.New("package checksum mismatch")
	}
	if _, err = archive.Seek(0, io.SeekStart); err != nil {
		return "", err
	}
	unpacked := filepath.Join(stage, "unpacked")
	if err = os.Mkdir(unpacked, 0700); err != nil {
		return "", err
	}
	if err = extractArchive(ctx, unpacked, name, archive); err != nil {
		return "", err
	}
	program := filepath.Join(unpacked, "bin", "code-server")
	resolved, err := filepath.EvalSymlinks(program)
	if err != nil || !within(unpacked, resolved) {
		return "", errors.New("package executable unavailable")
	}
	info, err := os.Stat(resolved)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0111 == 0 {
		return "", errors.New("package executable invalid")
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	if err = os.Rename(unpacked, target); err != nil {
		return "", err
	}
	return filepath.Join(target, "bin", "code-server"), nil
}

type contextReader struct {
	ctx context.Context
	src io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.src.Read(p)
}
func within(root, p string) bool {
	rel, err := filepath.Rel(root, p)
	return err == nil && !filepath.IsAbs(rel) && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func extractArchive(ctx context.Context, dest, prefix string, src io.Reader) error {
	canonical, canonicalErr := filepath.EvalSymlinks(dest)
	if canonicalErr != nil {
		return canonicalErr
	}
	dest = canonical
	gz, err := gzip.NewReader(src)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	root, err := os.OpenRoot(dest)
	if err != nil {
		return err
	}
	defer root.Close()
	type link struct{ name, target string }
	links := []link{}
	seen := map[string]bool{}
	var total int64
	for count := 0; ; count++ {
		if err = ctx.Err(); err != nil {
			return err
		}
		h, readErr := tr.Next()
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return readErr
		}
		if count >= 200000 {
			return errors.New("too many package entries")
		}
		clean := path.Clean(h.Name)
		if strings.Contains(h.Name, "\\") || strings.HasPrefix(h.Name, "/") || (clean != prefix && !strings.HasPrefix(clean, prefix+"/")) {
			return errors.New("unsafe package path")
		}
		name := strings.TrimPrefix(strings.TrimPrefix(clean, prefix), "/")
		if name == "" {
			if h.Typeflag != tar.TypeDir {
				return errors.New("invalid package root")
			}
			continue
		}
		if seen[name] {
			return errors.New("duplicate package entry")
		}
		seen[name] = true
		if err = root.MkdirAll(path.Dir(name), 0755); err != nil {
			return err
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = root.MkdirAll(name, 0755)
		case tar.TypeReg, tar.TypeRegA:
			if h.Size < 0 || h.Size > unpackLimit-total {
				return errors.New("expanded package too large")
			}
			total += h.Size
			mode := os.FileMode(0644)
			if h.Mode&0111 != 0 {
				mode = 0755
			}
			var f *os.File
			f, err = root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if err == nil {
				_, copyErr := io.Copy(f, &contextReader{ctx, tr})
				err = errors.Join(copyErr, f.Close())
			}
		case tar.TypeSymlink:
			target := path.Clean(path.Join(path.Dir(name), h.Linkname))
			if h.Linkname == "" || strings.HasPrefix(h.Linkname, "/") || strings.Contains(h.Linkname, "\\") || target == ".." || strings.HasPrefix(target, "../") {
				return errors.New("unsafe package symlink")
			}
			links = append(links, link{name, h.Linkname})
		default:
			return errors.New("unsupported package entry")
		}
		if err != nil {
			return err
		}
	}
	// Links are created last, so extraction never traverses archive symlinks.
	for _, l := range links {
		if err = root.Symlink(l.target, l.name); err != nil {
			return err
		}
	}
	for _, l := range links {
		resolved, e := filepath.EvalSymlinks(filepath.Join(dest, l.name))
		if e != nil || !within(dest, resolved) {
			return errors.New("unresolvable package symlink")
		}
	}
	return nil
}
