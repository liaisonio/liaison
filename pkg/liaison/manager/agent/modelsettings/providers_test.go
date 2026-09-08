package modelsettings

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestMultipleProvidersPreserveAndIsolateKeys(t *testing.T) {
	ctx := context.Background()
	m, err := New(&memoryStore{}, "secret", Config{}, func(context.Context, uint, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	c := Config{Enabled: true, DefaultProvider: "deepseek", Providers: []ProviderConfig{
		{ID: "deepseek", Type: "deepseek", BaseURL: "https://api.deepseek.com/v1", Model: "a", Models: []string{"a", "b"}, APIKey: "first-key"},
		{ID: "claude", Type: "anthropic", BaseURL: "https://api.anthropic.com/v1", Model: "claude", Models: []string{"claude"}, APIKey: "second-key"},
	}}
	v, err := m.Save(ctx, 1, Update{Config: c})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	if strings.Contains(string(raw), "first-key") || strings.Contains(string(raw), "second-key") {
		t.Fatal("key disclosed")
	}
	c.Providers[0].APIKey = ""
	c.Providers[1].APIKey = ""
	c.DefaultProvider = "claude"
	if _, err = m.Save(ctx, 1, Update{Config: c}); err != nil {
		t.Fatal(err)
	}
	restored, err := m.load(ctx)
	if err != nil {
		t.Fatal(err)
	}
	active, kind := selectedConfig(restored, "")
	if kind != "anthropic" || active.APIKey != "second-key" {
		t.Fatal("default routing/key mismatch")
	}
	c.Providers[0].BaseURL = "https://untrusted.example/v1"
	if _, err = m.Save(ctx, 1, Update{Config: c}); err == nil {
		t.Fatal("key forwarded to changed endpoint")
	}
	c.Providers = c.Providers[1:]
	if _, err = m.Save(ctx, 1, Update{Config: c}); err != nil {
		t.Fatal(err)
	}
	restored, _ = m.load(ctx)
	if len(restored.Providers) != 1 {
		t.Fatal("provider not removed")
	}
	c.DefaultProvider = "missing"
	if _, err = m.Save(ctx, 1, Update{Config: c}); err == nil {
		t.Fatal("invalid default accepted")
	}
}
