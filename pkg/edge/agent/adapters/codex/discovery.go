// Package codex describes Codex-specific discovery. It never launches Codex.
package codex

import (
	"path/filepath"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

type Adapter struct{}

func (Adapter) Kind() string { return "codex" }

func (Adapter) Layout(env discovery.Environment) (discovery.Layout, error) {
	layout := discovery.Layout{Names: []string{"codex"}}
	addBin := func(dir string) {
		if dir != "" {
			for _, name := range layout.Names {
				layout.Defaults = append(layout.Defaults, filepath.Join(dir, name))
			}
		}
	}
	switch env.OS {
	case "darwin":
		addBin("/opt/homebrew/bin")
		addBin("/usr/local/bin")
		addBin(filepath.Join(env.Home, ".local", "bin"))
		layout.Defaults = append(layout.Defaults, "/Applications/Codex.app/Contents/Resources/codex", filepath.Join(env.Home, "Applications", "Codex.app", "Contents", "Resources", "codex"))
	case "linux":
		addBin("/usr/local/bin")
		addBin("/usr/bin")
		addBin(filepath.Join(env.Home, ".local", "bin"))
	case "windows":
		layout.Names = []string{"codex.exe", "codex.cmd", "codex.bat", "codex.ps1"}
		if env.AppData != "" {
			addBin(filepath.Join(env.AppData, "npm"))
		}
		if env.LocalAppData != "" {
			addBin(filepath.Join(env.LocalAppData, "Programs", "Codex"))
			addBin(filepath.Join(env.LocalAppData, "Microsoft", "WinGet", "Links"))
		}
	default:
		return discovery.Layout{}, discovery.ErrUnsupported
	}
	if env.NpmPrefix != "" {
		if env.OS == "windows" {
			addBin(env.NpmPrefix)
		} else {
			addBin(filepath.Join(env.NpmPrefix, "bin"))
		}
	}
	volta := env.VoltaHome
	if volta == "" {
		volta = filepath.Join(env.Home, ".volta")
	}
	addBin(filepath.Join(volta, "bin"))
	if env.OS != "windows" {
		nvm := env.NVMDir
		if nvm == "" {
			nvm = filepath.Join(env.Home, ".nvm")
		}
		layout.VersionRoots = append(layout.VersionRoots, discovery.VersionRoot{Path: filepath.Join(nvm, "versions", "node"), BinSubdir: "bin"})
	} else if env.NVMDir != "" {
		layout.VersionRoots = append(layout.VersionRoots, discovery.VersionRoot{Path: env.NVMDir})
	}
	fnm := env.FNMDir
	if fnm == "" {
		switch env.OS {
		case "darwin":
			fnm = filepath.Join(env.Home, "Library", "Application Support", "fnm")
		case "linux":
			fnm = filepath.Join(env.Home, ".local", "share", "fnm")
		case "windows":
			if env.AppData != "" {
				fnm = filepath.Join(env.AppData, "fnm")
			}
		}
	}
	if fnm != "" {
		subdir := filepath.Join("installation", "bin")
		if env.OS == "windows" {
			subdir = "installation"
		}
		layout.VersionRoots = append(layout.VersionRoots, discovery.VersionRoot{Path: filepath.Join(fnm, "node-versions"), BinSubdir: subdir})
	}
	return layout, nil
}
