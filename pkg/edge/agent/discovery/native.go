package discovery

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

const maxCandidates = 256
const maxVersions = 64

// Environment must describe the execution account, not a browser-selected user.
// CurrentEnvironment deliberately does not enumerate other accounts or source
// their shell profiles. Cross-account execution is outside discovery's scope.
type Environment struct {
	OS, Home, AccountID                                         string
	PathDirs                                                    []string
	NpmPrefix, NVMDir, FNMDir, VoltaHome, AppData, LocalAppData string
}

func CurrentEnvironment() (Environment, error) {
	u, err := user.Current()
	if err != nil {
		return Environment{}, fmt.Errorf("current agent account: %w", err)
	}
	if !filepath.IsAbs(u.HomeDir) {
		return Environment{}, errors.New("account home is not absolute")
	}
	env := Environment{OS: runtime.GOOS, Home: u.HomeDir, AccountID: u.Uid,
		PathDirs: filepath.SplitList(os.Getenv("PATH")), NpmPrefix: os.Getenv("NPM_CONFIG_PREFIX"),
		NVMDir: os.Getenv("NVM_DIR"), FNMDir: os.Getenv("FNM_DIR"), VoltaHome: os.Getenv("VOLTA_HOME"),
		AppData: os.Getenv("APPDATA"), LocalAppData: os.Getenv("LOCALAPPDATA")}
	if runtime.GOOS == "windows" {
		env.NVMDir = os.Getenv("NVM_HOME")
	}
	return env, nil
}

type Native struct{ env Environment }

func NewNative(env Environment) (*Native, error) {
	if env.OS != runtime.GOOS || (env.OS != "darwin" && env.OS != "linux" && env.OS != "windows") {
		return nil, ErrUnsupported
	}
	if !filepath.IsAbs(env.Home) {
		return nil, errors.New("account home is not absolute")
	}
	return &Native{env: env}, nil
}

func (n *Native) Candidates(ctx context.Context, layout Layout, explicit string) ([]Candidate, []string, bool, error) {
	if explicit != "" && !filepath.IsAbs(explicit) {
		return nil, nil, false, ErrInvalidPath
	}
	var candidates []Candidate
	var warnings []string
	seen := map[string]bool{}
	truncated := false
	add := func(path, source string) {
		// Empty/relative PATH entries must never cause searches in the cwd.
		if !filepath.IsAbs(path) {
			return
		}
		path = filepath.Clean(path)
		key := path
		if n.env.OS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			return
		}
		seen[key] = true
		if len(candidates) >= maxCandidates {
			truncated = true
			return
		}
		candidates = append(candidates, Candidate{Path: path, Source: source})
	}
	if explicit != "" {
		add(explicit, "explicit")
	}
	for _, dir := range n.env.PathDirs {
		if err := ctx.Err(); err != nil {
			return nil, nil, false, err
		}
		if !filepath.IsAbs(dir) {
			continue
		}
		for _, name := range layout.Names {
			add(filepath.Join(dir, name), "path")
		}
		if len(candidates) >= maxCandidates {
			truncated = true
			break
		}
	}
	for _, path := range layout.Defaults {
		add(path, "default")
	}
	for _, root := range layout.VersionRoots {
		if err := ctx.Err(); err != nil {
			return nil, nil, false, err
		}
		if !filepath.IsAbs(root.Path) {
			continue
		}
		if len(candidates) >= maxCandidates {
			truncated = true
			break
		}
		file, err := os.Open(root.Path)
		if err != nil {
			if !os.IsNotExist(err) {
				warnings = append(warnings, "version directory unavailable")
			}
			continue
		}
		// One directory level only, with a bounded read. Never recursively walk.
		entries, readErr := file.ReadDir(maxVersions + 1)
		closeErr := file.Close()
		if readErr != nil && !errors.Is(readErr, io.EOF) {
			warnings = append(warnings, "version directory read failed")
			continue
		}
		if closeErr != nil {
			warnings = append(warnings, "version directory close failed")
		}
		if len(entries) > maxVersions {
			entries = entries[:maxVersions]
			truncated = true
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			for _, name := range layout.Names {
				add(filepath.Join(root.Path, entry.Name(), root.BinSubdir, name), "version-manager")
			}
		}
	}
	return candidates, warnings, truncated, nil
}

func (n *Native) Inspect(ctx context.Context, candidate Candidate) (Installation, error) {
	if err := ctx.Err(); err != nil {
		return Installation{}, err
	}
	if !filepath.IsAbs(candidate.Path) {
		return Installation{}, ErrInvalidPath
	}
	resolved, err := filepath.EvalSymlinks(candidate.Path)
	if err != nil {
		return Installation{}, err
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return Installation{}, err
	}
	if !info.Mode().IsRegular() {
		return Installation{}, errors.New("not a regular program file")
	}
	if n.env.OS != "windows" && info.Mode().Perm()&0111 == 0 {
		return Installation{}, errors.New("program has no execute permission")
	}
	if n.env.OS != "windows" && info.Mode().Perm()&0002 != 0 {
		return Installation{}, errors.New("world-writable program rejected")
	}
	ext := strings.ToLower(filepath.Ext(candidate.Path))
	return Installation{Path: candidate.Path, ResolvedPath: resolved, Source: candidate.Source, Wrapper: ext == ".cmd" || ext == ".bat" || ext == ".ps1"}, nil
}
