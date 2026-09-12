package modelsettings

import (
	"context"
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSelection_AllowlistAndNoSecrets(t *testing.T) {
	c := Config{Enabled: true, DefaultProvider: "one", Providers: []ProviderConfig{
		{ID: "one", Type: "custom", BaseURL: "https://one.example", APIKey: "private-one", Model: "a", Models: []string{"a", "b"}},
		{ID: "two", Type: "anthropic", BaseURL: "https://two.example", APIKey: "private-two", Model: "c", Models: []string{"c"}},
	}}
	m, err := New(&memoryStore{}, "secret", c, func(context.Context, uint, string) error { return nil })
	require.NoError(t, err)
	choices, err := m.Choices(context.Background())
	require.NoError(t, err)
	require.Len(t, choices, 3)
	raw, err := json.Marshal(choices)
	require.NoError(t, err)
	require.NotContains(t, string(raw), "private")
	require.NotContains(t, string(raw), "example")
	require.True(t, choices[0].IsDefault)
	for _, tc := range []struct {
		name      string
		selection runtime.ModelSelection
		valid     bool
	}{
		{"default", runtime.ModelSelection{}, true},
		{"alternate", runtime.ModelSelection{ProviderID: "one", Model: "b"}, true},
		{"other provider", runtime.ModelSelection{ProviderID: "two", Model: "c"}, true},
		{"cross provider", runtime.ModelSelection{ProviderID: "one", Model: "c"}, false},
		{"unknown", runtime.ModelSelection{ProviderID: "missing", Model: "b"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := m.ResolveSelection(context.Background(), tc.selection)
			if tc.valid {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, ErrInvalid)
			}
		})
	}
	selected, kind, resolved, err := resolveSelection(c, runtime.ModelSelection{ProviderID: "two", Model: "c"})
	require.NoError(t, err)
	require.Equal(t, "anthropic", kind)
	require.Equal(t, "private-two", selected.APIKey)
	require.Equal(t, "c", resolved.Model)
	c.Enabled = false
	_, _, _, err = resolveSelection(c, runtime.ModelSelection{})
	require.ErrorIs(t, err, ErrDisabled)
}

func TestSelection_RoutesActualInferenceWithoutChangingDefault(t *testing.T) {
	var received string
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Model    string `json:"model"`
			Messages []struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		received = body.Model
		if len(body.Messages) != 2 || body.Messages[0].Role != "system" || !strings.Contains(body.Messages[0].Content, "Simplified Chinese") {
			t.Error("global language missing from provider request")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		if _, err := w.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"OK\"},\"finish_reason\":null}]}\n\ndata: [DONE]\n\n")); err != nil {
			t.Error(err)
		}
	}))
	defer upstream.Close()
	c := Config{Enabled: true, DefaultProvider: "test", Providers: []ProviderConfig{{ID: "test", Type: "custom", BaseURL: upstream.URL, APIKey: "secret", Model: "a", Models: []string{"a", "b"}}}}
	m, err := New(&memoryStore{}, "secret", c, func(context.Context, uint, string) error { return nil })
	require.NoError(t, err)
	_, err = m.Generate(context.Background(), runtime.ModelRequest{Selection: runtime.ModelSelection{ProviderID: "test", Model: "b"}, Messages: []runtime.ModelMessage{{Role: runtime.RoleUser, Content: "hello"}}}, nil)
	require.NoError(t, err)
	require.Equal(t, "b", received)
	chosen, err := m.ResolveSelection(context.Background(), runtime.ModelSelection{})
	require.NoError(t, err)
	require.Equal(t, "a", chosen.Model)
}
