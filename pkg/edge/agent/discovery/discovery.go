// Package discovery finds Agent installation candidates without executing them.
// A result is not proof of identity, authentication, or permission to launch it.
package discovery

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
)

var ErrUnsupported = errors.New("unsupported agent platform")
var ErrInvalidPath = errors.New("explicit program path must be absolute")

type Candidate struct {
	Path   string
	Source string
}

type Installation struct {
	Agent        string
	Path         string
	ResolvedPath string
	Source       string
	// Wrappers require a platform-specific launch plan, never shell concatenation.
	Wrapper bool
}

type Result struct {
	Installations []Installation
	Warnings      []string
	Truncated     bool
}

// Layout contains Agent-specific names and known locations. Platform code owns
// filesystem access; adapters never execute commands during discovery.
type Layout struct {
	Names        []string
	Defaults     []string
	VersionRoots []VersionRoot
}
type VersionRoot struct{ Path, BinSubdir string }

type Adapter interface {
	Kind() string
	Layout(Environment) (Layout, error)
}

type Platform interface {
	Candidates(context.Context, Layout, string) ([]Candidate, []string, bool, error)
	Inspect(context.Context, Candidate) (Installation, error)
}

// Find preserves priority: explicit path, PATH, default locations, version
// managers. It returns every distinct target rather than silently selecting one.
func Find(ctx context.Context, platform Platform, env Environment, adapter Adapter, explicit string) (Result, error) {
	result := Result{Installations: []Installation{}}
	if platform == nil || adapter == nil {
		return result, errors.New("missing discovery dependency")
	}
	layout, err := adapter.Layout(env)
	if err != nil {
		return result, err
	}
	candidates, warnings, truncated, err := platform.Candidates(ctx, layout, explicit)
	result.Warnings, result.Truncated = warnings, truncated
	if err != nil {
		return result, err
	}
	seen := map[string]bool{}
	for _, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		installation, err := platform.Inspect(ctx, candidate)
		if err != nil {
			if candidate.Source == "explicit" {
				return result, fmt.Errorf("inspect explicit agent: %w", err)
			}
			if !os.IsNotExist(err) {
				result.Warnings = append(result.Warnings, "candidate could not be validated")
			}
			continue
		}
		key := installation.ResolvedPath
		if env.OS == "windows" {
			key = strings.ToLower(key)
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		installation.Agent = adapter.Kind()
		result.Installations = append(result.Installations, installation)
	}
	return result, nil
}
