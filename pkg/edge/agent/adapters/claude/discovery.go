package claude

import (
	"path/filepath"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
)

// Adapter discovers an existing CLI; discovery never launches or installs it.
type Adapter struct{}

func (Adapter) Kind() string { return "claude" }

func (Adapter) Layout(env discovery.Environment) (discovery.Layout, error) {
	l := discovery.Layout{Names: []string{"claude"}}
	add := func(dir string) {
		if dir != "" {
			for _, name := range l.Names {
				l.Defaults = append(l.Defaults, filepath.Join(dir, name))
			}
		}
	}
	switch env.OS {
	case "darwin":
		add("/opt/homebrew/bin")
		add("/usr/local/bin")
	case "linux":
		add("/usr/local/bin")
		add("/usr/bin")
	case "windows":
		l.Names = []string{"claude.exe", "claude.cmd", "claude.bat", "claude.ps1"}
		if env.AppData != "" {
			add(filepath.Join(env.AppData, "npm"))
		}
		if env.LocalAppData != "" {
			add(filepath.Join(env.LocalAppData, "Microsoft", "WinGet", "Links"))
		}
	default:
		return discovery.Layout{}, discovery.ErrUnsupported
	}
	if env.Home != "" {
		add(filepath.Join(env.Home, ".local", "bin"))
	}
	if env.NpmPrefix != "" {
		if env.OS == "windows" {
			add(env.NpmPrefix)
		} else {
			add(filepath.Join(env.NpmPrefix, "bin"))
		}
	}
	volta := env.VoltaHome
	if volta == "" && env.Home != "" {
		volta = filepath.Join(env.Home, ".volta")
	}
	if volta != "" {
		add(filepath.Join(volta, "bin"))
	}
	nvm := env.NVMDir
	if nvm == "" && env.Home != "" && env.OS != "windows" {
		nvm = filepath.Join(env.Home, ".nvm")
	}
	if nvm != "" {
		if env.OS == "windows" {
			l.VersionRoots = append(l.VersionRoots, discovery.VersionRoot{Path: nvm})
		} else {
			l.VersionRoots = append(l.VersionRoots, discovery.VersionRoot{Path: filepath.Join(nvm, "versions", "node"), BinSubdir: "bin"})
		}
	}
	fnm := env.FNMDir
	if fnm == "" {
		switch env.OS {
		case "darwin":
			if env.Home != "" {
				fnm = filepath.Join(env.Home, "Library", "Application Support", "fnm")
			}
		case "linux":
			if env.Home != "" {
				fnm = filepath.Join(env.Home, ".local", "share", "fnm")
			}
		case "windows":
			if env.AppData != "" {
				fnm = filepath.Join(env.AppData, "fnm")
			}
		}
	}
	if fnm != "" {
		sub := filepath.Join("installation", "bin")
		if env.OS == "windows" {
			sub = "installation"
		}
		l.VersionRoots = append(l.VersionRoots, discovery.VersionRoot{Path: filepath.Join(fnm, "node-versions"), BinSubdir: sub})
	}
	return l, nil
}
