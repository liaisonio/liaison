package web

import (
	"bytes"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
)

// Only rewrite the small IDE entry document, never scripts or project files.
// The icon itself comes from the console's existing static asset.
func brandIDEPage(response *http.Response, themes ...string) error {
	if response.StatusCode != http.StatusOK || !strings.HasPrefix(response.Header.Get("Content-Type"), "text/html") {
		return nil
	}
	if enc := response.Header.Get("Content-Encoding"); enc != "" && enc != "identity" {
		return errors.New("unexpected IDE document encoding")
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, (1<<20)+1))
	closeErr := response.Body.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if len(body) > 1<<20 {
		return errors.New("IDE document too large")
	}
	for _, name := range []string{"favicon-dark-support.svg", "favicon.ico"} {
		body = bytes.ReplaceAll(body, []byte("/src/browser/media/"+name+`"`), []byte(`/src/browser/media/favicon.ico?liaison=6bd4b1e1"`))
	}
	if len(themes) > 0 {
		body = defaultIDETheme(body, themes[0])
	}
	response.Body = io.NopCloser(bytes.NewReader(body))
	response.ContentLength = int64(len(body))
	response.Header.Set("Content-Length", strconv.Itoa(len(body)))
	response.Header.Del("ETag")
	response.Header.Del("Last-Modified")
	response.Header.Set("Cache-Control", "no-store")
	return nil
}

var ideWorkbenchConfig = regexp.MustCompile(`<meta\s+id="vscode-workbench-web-configuration"\s+data-settings="([^"]*)"\s*/?>`)

// Workbench defaults are lower priority than user/workspace settings. Never
// write settings.json or inject model/user text into an executable script.
func defaultIDETheme(body []byte, theme string) []byte {
	if theme != "dark" && theme != "light" {
		return body
	}
	match := ideWorkbenchConfig.FindSubmatchIndex(body)
	if match == nil {
		return body
	} // Other upstream HTML is left untouched.
	var config map[string]any
	if json.Unmarshal([]byte(html.UnescapeString(string(body[match[2]:match[3]]))), &config) != nil || config == nil {
		return body
	}
	defaults, ok := config["configurationDefaults"].(map[string]any)
	if !ok {
		if config["configurationDefaults"] != nil {
			return body
		}
		defaults = map[string]any{}
	}
	if _, configured := defaults["workbench.colorTheme"]; configured {
		return body
	}
	name, background := "Dark Modern", "#1f1f1f"
	if theme == "light" {
		name, background = "Light Modern", "#ffffff"
	}
	defaults["workbench.colorTheme"] = name
	config["configurationDefaults"] = defaults
	if config["initialColorTheme"] == nil {
		config["initialColorTheme"] = map[string]any{"themeType": theme, "colors": map[string]string{"editor.background": background}}
	}
	encoded, err := json.Marshal(config)
	if err != nil {
		return body
	}
	updated := make([]byte, 0, len(body)+len(encoded))
	updated = append(updated, body[:match[2]]...)
	updated = append(updated, html.EscapeString(string(encoded))...)
	updated = append(updated, body[match[3]:]...)
	// Native theme CSS takes over once the workbench mounts. No !important rule
	// that could override a theme explicitly selected inside the editor.
	style := `<style id="liaison-ide-loading-theme">html,body{background:` + background + `;color-scheme:` + theme + `}</style>`
	return bytes.Replace(updated, []byte("</head>"), []byte(style+"</head>"), 1)
}
