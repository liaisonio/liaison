package webide

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// seedManagedProfile supplies defaults only when no user settings exist. Some
// standalone releases advertise browser entrypoints absent from their archive.
// For those builtins, use the bundled Node entrypoint without web fallback.
// Do not patch upstream files, disable workspace trust, or alter discovered IDEs.
func seedManagedProfile(ctx context.Context, dataDir, packageRoot string) error {
	for _, dir := range []string{dataDir, filepath.Join(dataDir, "User")} {
		if err := os.Mkdir(dir, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			return err
		}
		if err := privateDirectory(dir); err != nil {
			return err
		}
	}
	settings := filepath.Join(dataDir, "User", "settings.json")
	if _, err := os.Lstat(settings); err == nil {
		return nil // Existing settings, including JSONC, belong to the user.
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	kinds, err := managedExtensionKinds(ctx, filepath.Join(packageRoot, "lib", "vscode", "extensions"))
	if err != nil {
		return fmt.Errorf("inspect managed IDE extensions: %w", err)
	}
	if len(kinds) == 0 {
		return nil
	}
	data, err := json.MarshalIndent(map[string]any{"remote.extensionKind": kinds}, "", "  ")
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(settings), ".settings-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name()) // Only this operation's private temporary file.
	if _, err = f.Write(append(data, '\n')); err != nil {
		f.Close()
		return err
	}
	if err = f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	// Publish atomically without replacing settings created concurrently.
	if err = os.Link(f.Name(), settings); errors.Is(err, os.ErrExist) {
		return nil
	}
	return err
}

func managedExtensionKinds(ctx context.Context, extensions string) (map[string][]string, error) {
	dir, err := os.OpenRoot(extensions)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	f, err := dir.Open(".")
	if err != nil {
		return nil, err
	}
	entries, err := f.ReadDir(257)
	f.Close()
	if err != nil && err != io.EOF {
		return nil, err
	}
	if len(entries) > 256 {
		return nil, errors.New("too many builtin extensions")
	}
	kinds := map[string][]string{}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if !entry.IsDir() {
			continue
		}
		root, err := dir.OpenRoot(entry.Name())
		if err != nil {
			return nil, err
		}
		id, missing, err := missingBrowserEntrypoint(root, entry.Name())
		root.Close()
		if err != nil {
			return nil, err
		}
		if missing {
			kinds[id] = []string{"workspace", "-web"}
		}
	}
	return kinds, nil
}

func missingBrowserEntrypoint(root *os.Root, name string) (string, bool, error) {
	f, err := root.Open("package.json")
	if errors.Is(err, os.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	data, err := io.ReadAll(io.LimitReader(f, (512<<10)+1))
	f.Close()
	if err != nil {
		return "", false, err
	}
	if len(data) > 512<<10 {
		return "", false, errors.New("builtin manifest too large")
	}
	var manifest struct{ Name, Publisher, Main, Browser string }
	if err = json.Unmarshal(data, &manifest); err != nil {
		return "", false, err
	}
	if manifest.Publisher != "vscode" || manifest.Name != name || manifest.Main == "" || manifest.Browser == "" {
		return "", false, nil
	}
	main, err := extensionEntrypointExists(root, manifest.Main)
	if err != nil || !main {
		return "", false, err
	}
	browser, err := extensionEntrypointExists(root, manifest.Browser)
	return "vscode." + name, !browser, err
}

func extensionEntrypointExists(root *os.Root, path string) (bool, error) {
	for _, candidate := range []string{path, path + ".js"} {
		info, err := root.Stat(candidate)
		if err == nil && info.Mode().IsRegular() {
			return true, nil
		}
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
	}
	return false, nil
}
