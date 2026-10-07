package web

import (
	"encoding/json"
	"html"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIDEThemeDefaults(t *testing.T) {
	for _, theme := range []string{"dark", "light"} {
		t.Run(theme, func(t *testing.T) {
			body := `<head><meta id="vscode-workbench-web-configuration" data-settings="{&quot;remoteAuthority&quot;:&quot;local&quot;,&quot;configurationDefaults&quot;:{&quot;editor.fontSize&quot;:14}}"></head><body></body>`
			got := defaultIDETheme([]byte(body), theme)
			match := ideWorkbenchConfig.FindSubmatch(got)
			var config map[string]any
			require.NoError(t, json.Unmarshal([]byte(html.UnescapeString(string(match[1]))), &config))
			require.Equal(t, "local", config["remoteAuthority"])
			defaults := config["configurationDefaults"].(map[string]any)
			require.Equal(t, float64(14), defaults["editor.fontSize"])
			require.Equal(t, strings.ToUpper(theme[:1])+theme[1:]+" Modern", defaults["workbench.colorTheme"])
			require.Contains(t, string(got), "color-scheme:"+theme)
			require.NotContains(t, string(got), "<script")
			require.Equal(t, got, defaultIDETheme(got, theme))
		})
	}
	for _, body := range []string{`<head>other app</head>`, `<meta id="vscode-workbench-web-configuration" data-settings="bad-json">`, `<meta id="vscode-workbench-web-configuration" data-settings="{&quot;configurationDefaults&quot;:{&quot;workbench.colorTheme&quot;:&quot;User Theme&quot;}}">`} {
		require.Equal(t, []byte(body), defaultIDETheme([]byte(body), "dark"))
	}
	unsafe := `<head><meta id="vscode-workbench-web-configuration" data-settings="{}"></head>`
	require.Equal(t, []byte(unsafe), defaultIDETheme([]byte(unsafe), `dark</style><script>alert(1)</script>`))
}

func TestIDEBrandPreservesTitleAndReplacesBothIcons(t *testing.T) {
	body := `<title>project-a</title><link rel="icon" href="./_static/src/browser/media/favicon-dark-support.svg"><link rel="alternate icon" href="./_static/src/browser/media/favicon.ico">`
	r := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html; charset=utf-8"}, "ETag": {"old"}}, Body: io.NopCloser(strings.NewReader(body))}
	require.NoError(t, brandIDEPage(r))
	got, err := io.ReadAll(r.Body)
	require.NoError(t, err)
	require.Contains(t, string(got), "<title>project-a</title>")
	require.Equal(t, 2, strings.Count(string(got), "favicon.ico?liaison=6bd4b1e1"))
	require.Equal(t, int64(len(got)), r.ContentLength)
	require.Empty(t, r.Header.Get("ETag"))
}

func TestIDEBrandRejectsOversizedEntry(t *testing.T) {
	r := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"text/html"}}, Body: io.NopCloser(strings.NewReader(strings.Repeat("x", (1<<20)+1)))}
	require.Error(t, brandIDEPage(r))
}
