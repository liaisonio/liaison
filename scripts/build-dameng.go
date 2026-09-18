//go:build ignore

// Build with a user-supplied, authorized DM driver without changing go.mod.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	driver := flag.String("driver", "", "absolute directory containing the authorized driver (module dm)")
	importPath := flag.String("driver-package", "", "driver import path, defaults to the supplied module root")
	out := flag.String("out", "", "output manager binary (required)")
	check := flag.Bool("check", false, "compile and test driver integration without building a manager")
	flag.Parse()
	if !filepath.IsAbs(*driver) || (!*check && *out == "") {
		return errors.New("use -driver /absolute/path/to/dm and -out /path/to/liaison (or -check)")
	}
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(root, *driver)
	if err != nil {
		return err
	}
	if relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return errors.New("keep the user-supplied driver outside the Liaison repository")
	}
	mod, err := os.ReadFile(filepath.Join(*driver, "go.mod"))
	if err != nil {
		return err
	}
	fields := strings.Fields(string(mod))
	if len(fields) < 2 || fields[0] != "module" {
		return errors.New("the supplied driver must contain a go.mod module declaration")
	}
	modulePath := strings.Trim(fields[1], `"`)
	if *importPath == "" {
		*importPath = modulePath
	}
	if *importPath != modulePath && !strings.HasPrefix(*importPath, modulePath+"/") {
		return errors.New("driver-package must be inside the supplied module")
	}
	subdir := strings.TrimPrefix(strings.TrimPrefix(*importPath, modulePath), "/")
	if strings.Contains(subdir, "..") {
		return errors.New("invalid driver package path")
	}
	files, err := filepath.Glob(filepath.Join(*driver, subdir, "*.go"))
	if err != nil {
		return err
	}
	hasHook := false
	for _, path := range files {
		b, e := os.ReadFile(path)
		if e != nil {
			return e
		}
		hasHook = hasHook || strings.Contains(string(b), "func RegisterDialContext(")
	}
	if !hasHook {
		return errors.New("driver lacks RegisterDialContext; this version cannot safely use a Liaison connector")
	}
	temp, err := os.MkdirTemp("", "liaison-dameng-build-")
	if err != nil {
		return err
	}
	defer func() {
		if err := os.RemoveAll(temp); err != nil {
			fmt.Fprintln(os.Stderr, "could not remove build scratch directory:", err)
		}
	}()
	modfile := filepath.Join(temp, "go.mod")
	for _, name := range []string{"go.mod", "go.sum"} {
		b, e := os.ReadFile(filepath.Join(root, name))
		if e != nil {
			return e
		}
		if e = os.WriteFile(filepath.Join(temp, name), b, 0600); e != nil {
			return e
		}
	}
	template, err := os.ReadFile(filepath.Join(root, "integrations/dameng/driver.go.in"))
	if err != nil {
		return err
	}
	adapter := filepath.Join(temp, "driver.go")
	if err = os.WriteFile(adapter, []byte(strings.Replace(string(template), `dm "dm"`, `dm `+strconv.Quote(*importPath), 1)), 0600); err != nil {
		return err
	}
	overlay, err := json.Marshal(map[string]any{"Replace": map[string]string{filepath.Join(root, "pkg/dameng/driver.go"): adapter}})
	if err != nil {
		return err
	}
	overlayfile := filepath.Join(temp, "overlay.json")
	if err = os.WriteFile(overlayfile, overlay, 0600); err != nil {
		return err
	}
	command := func(args ...string) error {
		c := exec.Command("go", args...)
		c.Dir = root
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		c.Env = append(os.Environ(), "GOWORK=off")
		return c.Run()
	}
	if err = command("mod", "edit", "-modfile="+modfile, "-require="+modulePath+"@v0.0.0", "-replace="+modulePath+"="+*driver); err != nil {
		return err
	}
	flags := []string{"-mod=mod", "-modfile=" + modfile, "-overlay=" + overlayfile}
	if *check {
		return command(append([]string{"test", "-race", "-v"}, append(flags, "./pkg/dameng", "./pkg/liaison/manager/web", "./pkg/liaison/manager/controlplane", "-run", "Dameng")...)...)
	}
	return command(append([]string{"build"}, append(flags, "-trimpath", "-o", *out, "./cmd/manager")...)...)
}
