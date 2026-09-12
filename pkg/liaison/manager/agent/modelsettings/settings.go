package modelsettings

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	agentmodel "github.com/liaisonio/liaison/pkg/liaison/manager/agent/model"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
)

var ErrInvalid = errors.New("invalid model settings")
var ErrDisabled = errors.New("model disabled")
var ErrProbe = errors.New("model connection failed; check endpoint, model and credentials")

type Config struct {
	OutputLanguage  string           `json:"output_language,omitempty"`
	Enabled         bool             `json:"enabled"`
	BaseURL         string           `json:"base_url"`
	Model           string           `json:"model"`
	APIKey          string           `json:"api_key,omitempty"`
	DefaultProvider string           `json:"default_provider,omitempty"`
	Providers       []ProviderConfig `json:"providers,omitempty"`
}
type Update struct {
	Config
	ClearKey     bool   `json:"clear_key"`
	TestProvider string `json:"test_provider,omitempty"`
}
type View struct {
	OutputLanguage  string         `json:"output_language"`
	Enabled         bool           `json:"enabled"`
	BaseURL         string         `json:"base_url"`
	Model           string         `json:"model"`
	HasAPIKey       bool           `json:"has_api_key"`
	DefaultProvider string         `json:"default_provider"`
	Providers       []ProviderView `json:"providers"`
}
type Store interface {
	ReadAgentModelConfig(context.Context) ([]byte, error)
	WriteAgentModelConfig(context.Context, []byte) error
}
type Authorize func(context.Context, uint, string) error
type Manager struct {
	mu        sync.Mutex
	store     Store
	aead      cipher.AEAD
	fallback  Config
	authorize Authorize
}

func New(store Store, secret string, fallback Config, authorize Authorize) (*Manager, error) {
	if store == nil || secret == "" || authorize == nil {
		return nil, ErrInvalid
	}
	key := sha256.Sum256([]byte("liaison-agent-model-settings-v1:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Manager{store: store, aead: aead, fallback: fallback, authorize: authorize}, nil
}
func (m *Manager) load(ctx context.Context) (Config, error) {
	data, err := m.store.ReadAgentModelConfig(ctx)
	if err != nil {
		return Config{}, err
	}
	if len(data) == 0 {
		return m.fallback, nil
	}
	if len(data) < m.aead.NonceSize() {
		return Config{}, ErrInvalid
	}
	raw, err := m.aead.Open(nil, data[:m.aead.NonceSize()], data[m.aead.NonceSize():], []byte("model-settings-v1"))
	if err != nil {
		return Config{}, ErrInvalid
	}
	var c Config
	err = json.Unmarshal(raw, &c)
	return c, err
}
func view(c Config) View {
	c = normalize(c)
	selected, _ := selectedConfig(c, "")
	result := View{Enabled: c.Enabled, BaseURL: selected.BaseURL, Model: selected.Model, HasAPIKey: selected.APIKey != "", DefaultProvider: c.DefaultProvider, Providers: []ProviderView{}}
	result.OutputLanguage = outputLanguage(c.OutputLanguage)
	for _, p := range c.Providers {
		result.Providers = append(result.Providers, ProviderView{ID: p.ID, Type: p.Type, BaseURL: p.BaseURL, Model: p.Model, Models: p.Models, HasAPIKey: p.APIKey != ""})
	}
	return result
}
func (m *Manager) Get(ctx context.Context, user uint) (View, error) {
	if err := m.authorize(ctx, user, "read"); err != nil {
		return View{}, err
	}
	c, err := m.load(ctx)
	return view(c), err
}
func validate(c Config) error {
	if len(c.APIKey) > 8192 || len(c.Model) > 200 || strings.ContainsAny(c.APIKey, "\r\n") {
		return ErrInvalid
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Host == "" || (u.Scheme != "https" && u.Scheme != "http") || u.User != nil || u.RawQuery != "" || u.Fragment != "" || len(c.BaseURL) > 2048 || strings.TrimSpace(c.Model) == "" {
		return ErrInvalid
	}
	return nil
}
func (m *Manager) candidate(ctx context.Context, u Update) (Config, error) {
	old, err := m.load(ctx)
	if err != nil {
		return Config{}, err
	}
	c := u.Config
	if c.OutputLanguage == "" {
		c.OutputLanguage = outputLanguage(old.OutputLanguage)
	}
	if c.OutputLanguage != "zh" && c.OutputLanguage != "en" {
		return Config{}, ErrInvalid
	}
	if u.Providers != nil {
		return mergeProviders(normalize(old), c)
	}
	c.BaseURL = strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
	c.Model = strings.TrimSpace(c.Model)
	if u.ClearKey {
		c.APIKey = ""
	} else if c.APIKey == "" {
		// Never forward a saved credential to an edited endpoint.
		if old.APIKey != "" && c.BaseURL != old.BaseURL {
			return Config{}, ErrInvalid
		}
		c.APIKey = old.APIKey
	}
	return c, validate(c)
}
func (m *Manager) Save(ctx context.Context, user uint, u Update) (View, error) {
	if err := m.authorize(ctx, user, "update"); err != nil {
		return View{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c, err := m.candidate(ctx, u)
	if err != nil {
		return View{}, err
	}
	raw, err := json.Marshal(c)
	if err != nil {
		return View{}, err
	}
	nonce := make([]byte, m.aead.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return View{}, err
	}
	encrypted := m.aead.Seal(nonce, nonce, raw, []byte("model-settings-v1"))
	if err = m.store.WriteAgentModelConfig(ctx, encrypted); err != nil {
		return View{}, err
	}
	return view(c), nil
}
func provider(c Config) (runtime.ModelProvider, error) {
	return agentmodel.NewOpenAICompatibleProvider(agentmodel.OpenAIConfig{BaseURL: c.BaseURL, Model: c.Model, APIKey: c.APIKey}, &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }})
}
func (m *Manager) Test(ctx context.Context, user uint, u Update) error {
	if err := m.authorize(ctx, user, "test"); err != nil {
		return err
	}
	if u.Providers != nil {
		id := u.TestProvider
		if id == "" {
			id = u.DefaultProvider
		}
		var selected *ProviderConfig
		for _, p := range u.Providers {
			if p.ID == id {
				copy := p
				selected = &copy
				break
			}
		}
		if selected == nil {
			return ErrInvalid
		}
		u.Providers = []ProviderConfig{*selected}
		u.DefaultProvider = id
	}
	c, err := m.candidate(ctx, u)
	if err != nil {
		return err
	}
	p, err := configuredProvider(c, u.TestProvider)
	if err != nil {
		return ErrInvalid
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	r, err := p.Generate(ctx, runtime.ModelRequest{Messages: []runtime.ModelMessage{{Role: runtime.RoleUser, Content: "Reply with OK only."}}}, nil)
	if err != nil || strings.TrimSpace(r.Text) == "" {
		return ErrProbe
	}
	return nil
}
func (m *Manager) Enabled(ctx context.Context) bool {
	c, err := m.load(ctx)
	return err == nil && c.Enabled
}
func (m *Manager) Generate(ctx context.Context, r runtime.ModelRequest, emit runtime.ModelEventSink) (runtime.ModelResponse, error) {
	c, err := m.load(ctx)
	if err != nil {
		return runtime.ModelResponse{}, err
	}
	if !c.Enabled {
		return runtime.ModelResponse{}, ErrDisabled
	}
	chosen, kind, _, err := resolveSelection(c, r.Selection)
	if err != nil {
		return runtime.ModelResponse{}, err
	}
	var p runtime.ModelProvider
	if kind == "anthropic" {
		p, err = anthropicProvider(chosen)
	} else {
		p, err = provider(chosen)
	}
	if err != nil {
		return runtime.ModelResponse{}, err
	}
	r = withOutputLanguage(r, c.OutputLanguage)
	response, err := p.Generate(ctx, r, emit)
	if err != nil {
		if ctx.Err() != nil {
			return runtime.ModelResponse{}, ctx.Err()
		}
		// Upstream error bodies must never enter persisted turns or API responses.
		return runtime.ModelResponse{}, ErrProbe
	}
	return response, nil
}
