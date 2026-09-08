package modelsettings

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

type memoryStore struct {
	mu   sync.Mutex
	data []byte
}

func (s *memoryStore) ReadAgentModelConfig(context.Context) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]byte(nil), s.data...), nil
}
func (s *memoryStore) WriteAgentModelConfig(_ context.Context, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = append([]byte(nil), data...)
	return nil
}
func TestEncryptedSettingsAndAuthorization(t *testing.T) {
	ctx := context.Background()
	store := &memoryStore{}
	denied := errors.New("forbidden")
	auth := func(_ context.Context, user uint, _ string) error {
		if user != 1 {
			return denied
		}
		return nil
	}
	m, err := New(store, "test-encryption-secret", Config{}, auth)
	if err != nil {
		t.Fatal(err)
	}
	update := Update{Config: Config{Enabled: true, BaseURL: "https://example.com/v1", Model: "model", APIKey: "test-private-key"}}
	for _, action := range []string{"get", "save", "test"} {
		switch action {
		case "get":
			_, err = m.Get(ctx, 2)
		case "save":
			_, err = m.Save(ctx, 2, update)
		case "test":
			err = m.Test(ctx, 2, update)
		}
		if !errors.Is(err, denied) {
			t.Fatalf("%s authorization: %v", action, err)
		}
	}
	got, err := m.Save(ctx, 1, update)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(got)
	if bytes.Contains(raw, []byte(update.APIKey)) || bytes.Contains(store.data, []byte(update.APIKey)) {
		t.Fatal("key disclosed")
	}
	if !got.HasAPIKey || !m.Enabled(ctx) {
		t.Fatal("settings not active")
	}
	restarted, err := New(store, "test-encryption-secret", Config{}, auth)
	if err != nil {
		t.Fatal(err)
	}
	current, err := restarted.load(ctx)
	if err != nil || current.APIKey != update.APIKey {
		t.Fatal("failed to restore encrypted configuration")
	}
	update.APIKey = ""
	update.BaseURL = "https://other.example/v1"
	if _, err = m.Save(ctx, 1, update); !errors.Is(err, ErrInvalid) {
		t.Fatal("forwarded saved key to edited endpoint")
	}
	update.ClearKey = true
	update.Enabled = false
	if _, err = m.Save(ctx, 1, update); err != nil {
		t.Fatal(err)
	}
	if m.Enabled(ctx) {
		t.Fatal("disable did not apply")
	}
	current, _ = m.load(ctx)
	if current.APIKey != "" {
		t.Fatal("clear key failed")
	}
}
func TestProbeDoesNotSaveAndErrorsAreSanitized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "secret-upstream-key", http.StatusUnauthorized)
	}))
	defer server.Close()
	m, err := New(&memoryStore{}, "secret", Config{}, func(context.Context, uint, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	update := Update{Config: Config{Enabled: true, BaseURL: server.URL, Model: "test"}}
	if err = m.Test(context.Background(), 1, update); !errors.Is(err, ErrProbe) {
		t.Fatal(err)
	}
	if m.Enabled(context.Background()) {
		t.Fatal("probe persisted settings")
	}
	if _, err = m.Save(context.Background(), 1, update); err != nil {
		t.Fatal(err)
	}
	_, err = m.Generate(context.Background(), runtime.ModelRequest{Messages: []runtime.ModelMessage{{Role: runtime.RoleUser, Content: "hello"}}}, nil)
	if !errors.Is(err, ErrProbe) {
		t.Fatalf("upstream error not sanitized: %v", err)
	}
}
