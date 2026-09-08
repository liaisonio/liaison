package modelsettings

import (
	agentmodel "github.com/liaisonio/liaison/pkg/liaison/manager/agent/model"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"net/http"
	"regexp"
	"slices"
	"strings"
	"time"
)

type ProviderConfig struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	BaseURL  string   `json:"base_url"`
	Model    string   `json:"model"`
	Models   []string `json:"models"`
	APIKey   string   `json:"api_key,omitempty"`
	ClearKey bool     `json:"clear_key,omitempty"`
}
type ProviderView struct {
	ID        string   `json:"id"`
	Type      string   `json:"type"`
	BaseURL   string   `json:"base_url"`
	Model     string   `json:"model"`
	Models    []string `json:"models"`
	HasAPIKey bool     `json:"has_api_key"`
}

var providerID = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,63}$`)

func normalize(c Config) Config {
	if c.Providers == nil && c.BaseURL != "" && c.Model != "" {
		id := "custom"
		if strings.TrimRight(c.BaseURL, "/") == "https://api.deepseek.com/v1" {
			id = "deepseek"
		}
		c.DefaultProvider = id
		c.Providers = []ProviderConfig{{ID: id, Type: id, BaseURL: c.BaseURL, Model: c.Model, Models: []string{c.Model}, APIKey: c.APIKey}}
	}
	return c
}
func mergeProviders(old Config, c Config) (Config, error) {
	if len(c.Providers) > 32 {
		return Config{}, ErrInvalid
	}
	known := map[string]ProviderConfig{}
	for _, p := range old.Providers {
		known[p.ID] = p
	}
	seen := map[string]bool{}
	c.Providers = append([]ProviderConfig(nil), c.Providers...)
	for i, p := range c.Providers {
		if !providerID.MatchString(p.ID) || seen[p.ID] {
			return Config{}, ErrInvalid
		}
		seen[p.ID] = true
		if !slices.Contains([]string{"openai", "anthropic", "gemini", "deepseek", "zhipu", "kimi", "minimax", "xiaomi", "custom"}, p.Type) {
			return Config{}, ErrInvalid
		}
		p.BaseURL = strings.TrimRight(strings.TrimSpace(p.BaseURL), "/")
		p.Model = strings.TrimSpace(p.Model)
		previous := known[p.ID]
		if p.ClearKey {
			p.APIKey = ""
		} else if p.APIKey == "" {
			if previous.APIKey != "" && (previous.BaseURL != p.BaseURL || previous.Type != p.Type) {
				return Config{}, ErrInvalid
			}
			p.APIKey = previous.APIKey
		}
		p.ClearKey = false
		if err := validate(Config{BaseURL: p.BaseURL, Model: p.Model, APIKey: p.APIKey}); err != nil {
			return Config{}, err
		}
		if len(p.Models) == 0 || len(p.Models) > 100 {
			return Config{}, ErrInvalid
		}
		p.Models = append([]string(nil), p.Models...)
		models := map[string]bool{}
		for j, name := range p.Models {
			name = strings.TrimSpace(name)
			if name == "" || len(name) > 200 || models[name] {
				return Config{}, ErrInvalid
			}
			models[name] = true
			p.Models[j] = name
		}
		if !models[p.Model] {
			return Config{}, ErrInvalid
		}
		c.Providers[i] = p
	}
	if len(c.Providers) == 0 {
		if c.Enabled {
			return Config{}, ErrInvalid
		}
		c.DefaultProvider = ""
	} else if !seen[c.DefaultProvider] {
		return Config{}, ErrInvalid
	}
	// Single-provider fields are retained for the old API but never duplicate keys in new payloads.
	c.BaseURL = ""
	c.Model = ""
	c.APIKey = ""
	return c, nil
}
func selectedConfig(c Config, id string) (Config, string) {
	if c.Providers == nil {
		return c, "custom"
	}
	if id == "" {
		id = c.DefaultProvider
	}
	for _, p := range c.Providers {
		if p.ID == id {
			return Config{Enabled: c.Enabled, BaseURL: p.BaseURL, Model: p.Model, APIKey: p.APIKey}, p.Type
		}
	}
	return Config{}, ""
}
func configuredProvider(c Config, id string) (runtime.ModelProvider, error) {
	chosen, kind := selectedConfig(c, id)
	if kind == "anthropic" {
		return anthropicProvider(chosen)
	}
	return provider(chosen)
}
func anthropicProvider(c Config) (runtime.ModelProvider, error) {
	return agentmodel.NewAnthropicProvider(agentmodel.OpenAIConfig{BaseURL: c.BaseURL, Model: c.Model, APIKey: c.APIKey}, &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
}
