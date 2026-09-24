package codex

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPreviewExternalToolsFailClosed(t *testing.T) {
	for _, tc := range []struct {
		json    string
		allowed bool
	}{
		{`{"data":[],"nextCursor":null}`, true},
		{`{"data":[{"name":"remote-tools"}],"nextCursor":null}`, false},
		{`{"data":[{"runtimeStatus":"disabled","tools":{}}],"nextCursor":null}`, true},
		{`{"data":[{"runtimeStatus":"disabled","tools":{"write":{}}}]}`, false},
		{`{"data":[{"runtimeStatus":"connected","tools":{}}]}`, false},
		{`{"data":[],"nextCursor":"more"}`, false},
		{`{"data":null}`, false},
		{`{}`, false},
		{`invalid`, false},
	} {
		if got := emptyExternalToolInventory([]byte(tc.json)); got != tc.allowed {
			t.Errorf("inventory %q allowed=%v", tc.json, got)
		}
	}
}

func TestPreviewOverrides_OnlyRestrictsIntegrations(t *testing.T) {
	overrides, err := previewOverrides([]byte(`{"config":{"model":"keep-local-model","model_provider":"keep-local-provider","api_key":"never-forward-this","mcp_servers":{"server.with.dots":{"command":"private-command","env":{"TOKEN":"secret-value"}}},"plugins":{"tool@market":{"enabled":true}}}}`))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(overrides)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"keep-local-model", "keep-local-provider", "never-forward-this", "private-command", "secret-value"} {
		if strings.Contains(string(encoded), secret) {
			t.Fatal("non-policy configuration forwarded")
		}
	}
	if overrides["features.apps"] != false {
		t.Fatal("apps enabled")
	}
	for _, table := range []string{"mcp_servers", "plugins"} {
		entries := overrides[table].(map[string]any)
		for _, value := range entries {
			if value.(map[string]any)["enabled"] != false {
				t.Fatal("integration enabled")
			}
		}
	}
	for _, input := range []string{`{}`, `null`, `{"config":null}`, `invalid`} {
		if _, err := previewOverrides([]byte(input)); err == nil {
			t.Fatal("invalid config accepted")
		}
	}
}
