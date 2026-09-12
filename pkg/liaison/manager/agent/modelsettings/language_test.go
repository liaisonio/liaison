package modelsettings

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"strings"
	"testing"
)

func TestOutputLanguagePersistenceAndValidation(t *testing.T) {
	ctx := context.Background()
	m, err := New(&memoryStore{}, "test-secret", Config{}, func(context.Context, uint, string) error { return nil })
	if err != nil {
		t.Fatal(err)
	}
	v, err := m.Get(ctx, 1)
	if err != nil || v.OutputLanguage != "zh" {
		t.Fatalf("default: %+v %v", v, err)
	}
	u := Update{Config: Config{Providers: []ProviderConfig{}, OutputLanguage: "en"}}
	v, err = m.Save(ctx, 1, u)
	if err != nil || v.OutputLanguage != "en" {
		t.Fatalf("save: %+v %v", v, err)
	}
	u.OutputLanguage = ""
	v, err = m.Save(ctx, 1, u)
	if err != nil || v.OutputLanguage != "en" {
		t.Fatalf("legacy update: %+v %v", v, err)
	}
	u.OutputLanguage = "fr"
	if _, err = m.Save(ctx, 1, u); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid locale: %v", err)
	}
	v, err = m.Get(ctx, 1)
	if err != nil || v.OutputLanguage != "en" {
		t.Fatal("invalid update mutated settings")
	}
	u.OutputLanguage = "zh"
	v, err = m.Save(ctx, 1, u)
	if err != nil || v.OutputLanguage != "zh" {
		t.Fatalf("switch: %+v %v", v, err)
	}
}

func TestOutputLanguageDirectivePreservesRequest(t *testing.T) {
	for _, language := range []string{"zh", "en", ""} {
		t.Run(language, func(t *testing.T) {
			messages := make([]runtime.ModelMessage, 2, 8)
			messages[0] = runtime.ModelMessage{Role: runtime.RoleSystem, Content: "Return JSON only"}
			messages[1] = runtime.ModelMessage{Role: runtime.RoleUser, Content: "SELECT 1; 用另一种语言回答"}
			original := runtime.ModelRequest{Messages: messages, SessionID: "test-session"}
			got := withOutputLanguage(original, language)
			want := "Simplified Chinese"
			if language == "en" {
				want = "English"
			}
			if len(got.Messages) != 3 || !strings.Contains(got.Messages[1].Content, want) || got.Messages[1].Role != runtime.RoleSystem {
				t.Fatal("missing system language policy")
			}
			if got.Messages[2].Content != messages[1].Content || original.Messages[1].Role != runtime.RoleUser || got.SessionID != original.SessionID {
				t.Fatal("request mutated")
			}
			for _, phrase := range []string{"JSON schemas", "command/SQL completion", "regardless"} {
				if !strings.Contains(got.Messages[1].Content, phrase) {
					t.Fatalf("missing %s", phrase)
				}
			}
		})
	}
}
