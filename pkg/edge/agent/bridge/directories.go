package bridge

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/liaisonio/liaison/pkg/proto"
)

// Directory metadata stays within the Edge user's home or the saved project.
// os.Root enforces the boundary even if a symlink changes during a listing.
func directoryRoots(home, project string) []string {
	roots := []string{}
	for _, path := range []string{home, project} {
		if !filepath.IsAbs(path) {
			continue
		}
		clean, err := filepath.EvalSymlinks(path)
		if err != nil || clean == string(filepath.Separator) {
			continue
		}
		info, err := os.Stat(clean)
		if err != nil || !info.IsDir() {
			continue
		}
		duplicate := false
		for _, r := range roots {
			if r == clean {
				duplicate = true
			}
		}
		if !duplicate {
			roots = append(roots, clean)
		}
	}
	return roots
}
func withinDirectory(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) && !filepath.IsAbs(rel)
}
func resolveDirectory(roots []string, path string) (string, string, bool) {
	if path == "" && len(roots) > 0 {
		path = roots[0]
	}
	if !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return "", "", false
	}
	clean, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", false
	}
	for _, root := range roots {
		if withinDirectory(root, clean) {
			return root, clean, true
		}
	}
	return "", "", false
}
func browseDirectories(ctx context.Context, home, project, path string) proto.EdgeAgentResult {
	roots := directoryRoots(home, project)
	root, path, ok := resolveDirectory(roots, path)
	if !ok {
		return result("invalid_request")
	}
	dir, err := os.OpenRoot(root)
	if err != nil {
		return result("invalid_request")
	}
	defer dir.Close()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return result("invalid_request")
	}
	file, err := dir.Open(rel)
	if err != nil {
		return result("invalid_request")
	}
	defer file.Close()
	items, err := file.ReadDir(2000)
	if err != nil && err != io.EOF {
		return result("invalid_request")
	}
	out := result("ok")
	out.Directory = path
	out.DirectoryRoots = roots
	out.Truncated = len(items) == 2000
	bytes := len(path)
	parent := filepath.Dir(path)
	if withinDirectory(root, parent) && parent != path {
		out.ParentDirectory = parent
	}
	for _, item := range items {
		if ctx.Err() != nil {
			return result("unavailable")
		}
		if strings.HasPrefix(item.Name(), ".") {
			continue
		}
		info, err := dir.Stat(filepath.Join(rel, item.Name()))
		if err != nil || !info.IsDir() {
			continue
		}
		child := filepath.Join(path, item.Name())
		bytes += len(child) + len(item.Name())
		if bytes > 128<<10 {
			out.Truncated = true
			break
		}
		out.Directories = append(out.Directories, proto.AgentDirectory{Name: item.Name(), Path: child})
	}
	sort.Slice(out.Directories, func(i, j int) bool { return out.Directories[i].Name < out.Directories[j].Name })
	if len(out.Directories) > 200 {
		out.Directories = out.Directories[:200]
		out.Truncated = true
	}
	return out
}
