package codex

import (
	"errors"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/process"
)

// npm's entry point is a Node script. Services frequently omit Homebrew/nvm
// from PATH, so invoke a validated absolute interpreter instead of relying on
// /usr/bin/env. No shell profile is sourced and no runtime is installed.
func launchSpec(installation discovery.Installation, directory string) (process.Spec, error) {
	spec := process.Spec{Executable: installation.Path, ResolvedExecutable: installation.ResolvedPath, Directory: directory, Args: []string{"app-server", "--listen", "stdio://"}}
	if !strings.EqualFold(filepath.Ext(installation.ResolvedPath), ".js") {
		return spec, nil
	}
	// Both the original script and interpreter must pass ownership checks.
	if err := process.Validate(spec); err != nil {
		return process.Spec{}, err
	}
	candidates := []string{filepath.Join(filepath.Dir(installation.Path), "node")}
	if path, err := exec.LookPath("node"); err == nil && filepath.IsAbs(path) {
		candidates = append(candidates, path)
	}
	candidates = append(candidates, "/opt/homebrew/bin/node", "/usr/local/bin/node", "/usr/bin/node")
	for _, path := range candidates {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			continue
		}
		node := process.Spec{Executable: path, ResolvedExecutable: resolved, Directory: directory, Args: append([]string{installation.ResolvedPath}, spec.Args...)}
		if process.Validate(node) == nil {
			return node, nil
		}
	}
	return process.Spec{}, errors.New("npm Codex requires a trusted Node executable; none found for this account")
}
