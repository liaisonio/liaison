package webide

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func builtinFixture(t *testing.T, extensions, name, publisher, main, browser string, files ...string) string {
	t.Helper()
	dir := filepath.Join(extensions, name)
	require.NoError(t, os.MkdirAll(dir, 0700))
	data, err := json.Marshal(map[string]string{"name": name, "publisher": publisher, "main": main, "browser": browser})
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), data, 0600))
	for _, file := range files {
		path := filepath.Join(dir, file)
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0700))
		require.NoError(t, os.WriteFile(path, []byte("// fixture"), 0600))
	}
	return dir
}

func TestManagedExtensionKindsOnlyRepairsMissingBrowserEntrypoints(t *testing.T) {
	dir := t.TempDir()
	builtinFixture(t, dir, "missing-browser", "vscode", "./main", "./browser", "main.js")
	builtinFixture(t, dir, "complete", "vscode", "./main", "./browser", "main.js", "browser.js")
	builtinFixture(t, dir, "explicit", "vscode", "./main.js", "./browser.js", "main.js", "browser.js")
	builtinFixture(t, dir, "node-only", "vscode", "./main.js", "", "main.js")
	builtinFixture(t, dir, "web-only", "vscode", "", "./browser.js", "browser.js")
	builtinFixture(t, dir, "broken", "vscode", "./main.js", "./browser.js")
	builtinFixture(t, dir, "third-party", "other", "./main.js", "./browser.js", "main.js")
	kinds, err := managedExtensionKinds(t.Context(), dir)
	require.NoError(t, err)
	require.Equal(t, map[string][]string{"vscode.missing-browser": {"workspace", "-web"}}, kinds)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = managedExtensionKinds(ctx, dir)
	require.ErrorIs(t, err, context.Canceled)
}

func TestManagedExtensionKindsRejectsEscapingEntrypoints(t *testing.T) {
	for _, link := range []bool{false, true} {
		t.Run(map[bool]string{false: "traversal", true: "symlink"}[link], func(t *testing.T) {
			dir := t.TempDir()
			outside := filepath.Join(dir, "outside.js")
			require.NoError(t, os.WriteFile(outside, nil, 0600))
			main := "../outside.js"
			if link {
				main = "./main.js"
			}
			builtin := builtinFixture(t, dir, "escape", "vscode", main, "./browser.js")
			if link {
				require.NoError(t, os.Symlink(outside, filepath.Join(builtin, "main.js")))
			}
			_, err := managedExtensionKinds(t.Context(), dir)
			require.Error(t, err)
		})
	}
}

func TestManagedProfileSeedsPrivateDefaultsWithoutReplacingUserSettings(t *testing.T) {
	base := t.TempDir()
	data := filepath.Join(base, "data")
	packageRoot := filepath.Join(base, "package")
	builtinFixture(t, filepath.Join(packageRoot, "lib", "vscode", "extensions"), "git-base", "vscode", "./main.js", "./browser.js", "main.js")
	require.NoError(t, seedManagedProfile(t.Context(), data, packageRoot))
	settings := filepath.Join(data, "User", "settings.json")
	content, err := os.ReadFile(settings)
	require.NoError(t, err)
	require.JSONEq(t, `{"remote.extensionKind":{"vscode.git-base":["workspace","-web"]}}`, string(content))
	info, err := os.Stat(settings)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	custom := []byte("// user JSONC\n{\"editor.fontSize\": 14}\n")
	require.NoError(t, os.WriteFile(settings, custom, 0600))
	require.NoError(t, seedManagedProfile(t.Context(), data, packageRoot))
	content, err = os.ReadFile(settings)
	require.NoError(t, err)
	require.Equal(t, custom, content)
	entries, err := os.ReadDir(filepath.Dir(settings))
	require.NoError(t, err)
	require.Len(t, entries, 1, "temporary defaults must be cleaned up")
}

func TestManagedProfileRejectsRedirectedUserDirectory(t *testing.T) {
	base := t.TempDir()
	data := filepath.Join(base, "data")
	require.NoError(t, os.Mkdir(data, 0700))
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(data, "User")))
	require.Error(t, seedManagedProfile(t.Context(), data, "unused"))
	entries, err := os.ReadDir(outside)
	require.NoError(t, err)
	require.Empty(t, entries)
}
