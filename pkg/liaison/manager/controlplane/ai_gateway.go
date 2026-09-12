package controlplane

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/aigateway"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/liaisonio/liaison/pkg/trafficconn"
	"gorm.io/gorm"
)

var ErrAIInvalid = errors.New("invalid AI API configuration or request")
var ErrAIUnavailable = errors.New("AI API access is unavailable")
var ErrAITokenQuota = errors.New("AI key token quota exhausted")
var ErrAITokenUsage = errors.New("AI key token usage unconfirmed")

type AIService struct {
	cp     *controlPlane
	iam    *iam.IAMService
	cipher cipher.AEAD
}

// NewAIService is optional on ControlPlane mocks; production creates it once,
// before serving requests. Dependencies never change after construction.
func (cp *controlPlane) NewAIService(auth *iam.IAMService, key []byte) (*AIService, error) {
	if auth == nil || len(key) < 32 {
		return nil, errors.New("AI API encryption/auth dependency missing")
	}
	derived := sha256.Sum256(append(append([]byte{}, key...), []byte("liaison-ai-api-v1")...))
	block, err := aes.NewCipher(derived[:])
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &AIService{cp: cp, iam: auth, cipher: aead}, nil
}

type AIApplicationConfig struct {
	Protocol  string `json:"protocol"`
	BasePath  string `json:"base_path"`
	TLS       bool   `json:"tls"`
	APIKey    string `json:"api_key,omitempty"`
	ClearKey  bool   `json:"clear_key,omitempty"`
	HasAPIKey bool   `json:"has_api_key"`
}
type AIAccessConfig struct {
	Enabled          bool              `json:"enabled"`
	Models           map[string]string `json:"models"`
	ExternalProtocol string            `json:"external_protocol"`
}
type AIWorkspace struct {
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	Models    []string `json:"models"`
	CanManage bool     `json:"can_manage"`
}

// Workspace exposes public aliases only, never application configuration.
func (s *AIService) Workspace(ctx context.Context, id uint) (AIWorkspace, error) {
	p, _, err := s.access(ctx, id, "use")
	if err != nil {
		return AIWorkspace{}, err
	}
	view := AIWorkspace{Name: p.Name, Models: []string{}}
	_, err = s.actor(ctx, "accesses", "update")
	view.CanManage = err == nil
	c, err := s.cp.repo.GetAIAccess(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return view, nil
	}
	if err != nil {
		return AIWorkspace{}, err
	}
	var mappings map[string]string
	if err = json.Unmarshal([]byte(c.Models), &mappings); err != nil {
		return AIWorkspace{}, err
	}
	view.Models = aigateway.ModelAliases(mappings)
	view.Enabled = c.Enabled && p.Status == model.ProxyStatusRunning
	return view, nil
}

type AIKeyRequest struct {
	Name          string   `json:"name"`
	Models        []string `json:"models"`
	ExpiresInDays int      `json:"expires_in_days"`
	TokenLimit    *int64   `json:"token_limit"`
}
type AIKeyView struct {
	ID              uint      `json:"id"`
	Name            string    `json:"name"`
	Models          []string  `json:"models"`
	ExpiresAt       time.Time `json:"expires_at"`
	Secret          string    `json:"secret,omitempty"`
	TokenLimit      *int64    `json:"token_limit"`
	UsedTokens      int64     `json:"used_tokens"`
	RemainingTokens *int64    `json:"remaining_tokens"`
	UnknownRequests int64     `json:"unknown_requests"`
}
type AIGrant struct {
	Metered     bool
	KeyID       uint
	UserID      uint
	ProxyID     uint
	Models      map[string]string
	Protocol    string
	Upstream    *aigateway.Upstream
	UpstreamKey string
	Revalidate  func(context.Context) error
}

func (s *AIService) actor(ctx context.Context, resource, action string) (uint, error) {
	id, ok := actorUserID(ctx)
	if !ok {
		return 0, iam.ErrForbidden
	}
	u, err := s.iam.GetUserByID(id)
	if err != nil || u.Status != model.UserStatusActive {
		return 0, iam.ErrForbidden
	}
	if err = s.iam.RequireResourcePermission(u, resource, action); err != nil {
		return 0, err
	}
	return id, nil
}
func (s *AIService) application(ctx context.Context, id uint, action string) (*model.Application, error) {
	if _, err := s.actor(ctx, "applications", action); err != nil {
		return nil, err
	}
	if err := requireVisibleResource(ctx, s.cp.repo, resourceApplication, uint64(id)); err != nil {
		return nil, iam.ErrForbidden
	}
	app, err := s.cp.repo.GetApplicationByID(id)
	if err != nil {
		return nil, err
	}
	if app.ApplicationType != model.ApplicationTypeLLM || len(app.EdgeIDs) != 1 {
		return nil, ErrAIInvalid
	}
	if err := requireVisibleResource(ctx, s.cp.repo, resourceConnector, uint64(app.EdgeIDs[0])); err != nil {
		return nil, iam.ErrForbidden
	}
	return app, nil
}
func (s *AIService) access(ctx context.Context, id uint, action string) (*model.Proxy, *model.Application, error) {
	if _, err := s.actor(ctx, "accesses", action); err != nil {
		return nil, nil, err
	}
	if err := requireVisibleResource(ctx, s.cp.repo, resourceAccess, uint64(id)); err != nil {
		return nil, nil, iam.ErrForbidden
	}
	p, err := s.cp.repo.GetProxyByID(id)
	if err != nil {
		return nil, nil, err
	}
	if p.AccessProtocol != model.AccessProtocolAI {
		return nil, nil, ErrAIInvalid
	}
	app, err := s.application(ctx, p.ApplicationID, "read")
	return p, app, err
}
func aiFingerprint(app *model.Application, c *AIApplicationConfig) string {
	return aigateway.KeyDigest(fmt.Sprintf("%d|%v|%s|%d|%s|%s|%t", app.ID, app.EdgeIDs, app.IP, app.Port, c.Protocol, c.BasePath, c.TLS))
}
func aiAppView(v *model.AIApplication) AIApplicationConfig {
	return AIApplicationConfig{Protocol: v.Protocol, BasePath: v.BasePath, TLS: v.TLS, HasAPIKey: v.EncryptedKey != ""}
}
func (s *AIService) GetApplication(ctx context.Context, id uint) (AIApplicationConfig, error) {
	if _, err := s.application(ctx, id, "update"); err != nil {
		return AIApplicationConfig{}, err
	}
	v, err := s.cp.repo.GetAIApplication(ctx, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return AIApplicationConfig{Protocol: "openai-compatible", BasePath: "/v1"}, nil
	}
	if err != nil {
		return AIApplicationConfig{}, err
	}
	return aiAppView(v), nil
}
func (s *AIService) SaveApplication(ctx context.Context, id uint, c AIApplicationConfig) (AIApplicationConfig, error) {
	app, err := s.application(ctx, id, "update")
	if err != nil {
		return AIApplicationConfig{}, err
	}
	if c.Protocol != "openai-compatible" && c.Protocol != "anthropic" || len(c.APIKey) > 8192 || strings.ContainsAny(c.APIKey, "\r\n") {
		return AIApplicationConfig{}, ErrAIInvalid
	}
	u, err := aigateway.NewUpstream(aigateway.Target{Host: app.IP, Port: app.Port, TLS: c.TLS, BasePath: c.BasePath}, func(context.Context) (net.Conn, error) { return nil, ErrAIUnavailable })
	if err != nil {
		return AIApplicationConfig{}, ErrAIInvalid
	}
	u.Close()
	fingerprint := aiFingerprint(app, &c)
	old, err := s.cp.repo.GetAIApplication(ctx, id)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return AIApplicationConfig{}, err
	}
	encrypted := ""
	if old != nil && old.EncryptedKey != "" && !c.ClearKey && c.APIKey == "" {
		if old.TargetFingerprint != fingerprint {
			return AIApplicationConfig{}, ErrAIInvalid
		}
		encrypted = old.EncryptedKey
	}
	if c.APIKey != "" && !c.ClearKey {
		nonce := make([]byte, s.cipher.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			return AIApplicationConfig{}, err
		}
		encrypted = base64.StdEncoding.EncodeToString(s.cipher.Seal(nonce, nonce, []byte(c.APIKey), []byte(fingerprint)))
	}
	v := &model.AIApplication{ApplicationID: id, Protocol: c.Protocol, BasePath: c.BasePath, TLS: c.TLS, EncryptedKey: encrypted, TargetFingerprint: fingerprint}
	if err = s.cp.repo.SaveAIApplication(ctx, v); err != nil {
		return AIApplicationConfig{}, err
	}
	return aiAppView(v), nil
}
func (s *AIService) GetAccess(ctx context.Context, id uint) (AIAccessConfig, error) {
	if _, _, err := s.access(ctx, id, "update"); err != nil {
		return AIAccessConfig{}, err
	}
	v, err := s.cp.repo.GetAIAccess(ctx, id)
	c := AIAccessConfig{ExternalProtocol: "openai-compatible", Models: map[string]string{}}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	c.Enabled = v.Enabled
	err = json.Unmarshal([]byte(v.Models), &c.Models)
	return c, err
}
func (s *AIService) SaveAccess(ctx context.Context, id uint, c AIAccessConfig) (AIAccessConfig, error) {
	if _, _, err := s.access(ctx, id, "update"); err != nil {
		return c, err
	}
	if c.ExternalProtocol != "" && c.ExternalProtocol != "openai-compatible" || len(c.Models) > 100 || c.Enabled && len(c.Models) == 0 {
		return c, ErrAIInvalid
	}
	aliases := aigateway.ModelAliases(c.Models)
	if len(aigateway.AllowedModels(aliases, c.Models)) != len(c.Models) {
		return c, ErrAIInvalid
	}
	data, err := json.Marshal(c.Models)
	if err != nil {
		return c, err
	}
	c.ExternalProtocol = "openai-compatible"
	err = s.cp.repo.SaveAIAccess(ctx, &model.AIAccess{ProxyID: id, Enabled: c.Enabled, Models: string(data)})
	return c, err
}
func (s *AIService) Keys(ctx context.Context, id uint) ([]AIKeyView, error) {
	if _, _, err := s.access(ctx, id, "use"); err != nil {
		return nil, err
	}
	user, _ := actorUserID(ctx)
	keys, err := s.cp.repo.ListAIKeys(ctx, id, user)
	if err != nil {
		return nil, err
	}
	result := make([]AIKeyView, 0, len(keys))
	for _, k := range keys {
		var scope []string
		if err = json.Unmarshal([]byte(k.Models), &scope); err != nil {
			return nil, err
		}
		usage, err := s.cp.repo.GetAIKeyUsage(ctx, id, user, k.ID)
		if err != nil {
			return nil, err
		}
		view := AIKeyView{ID: k.ID, Name: k.Name, Models: scope, ExpiresAt: k.ExpiresAt, TokenLimit: usage.TokenLimit, UsedTokens: usage.UsedTokens, UnknownRequests: usage.UnknownRequests}
		if usage.TokenLimit != nil && usage.UnknownRequests == 0 {
			remaining := max(int64(0), *usage.TokenLimit-usage.UsedTokens)
			view.RemainingTokens = &remaining
		}
		result = append(result, view)
	}
	return result, nil
}
func (s *AIService) CreateKey(ctx context.Context, id uint, c AIKeyRequest) (AIKeyView, error) {
	var view AIKeyView
	if _, _, err := s.access(ctx, id, "use"); err != nil {
		return view, err
	}
	if !validAITokenLimit(c.TokenLimit) || len(strings.TrimSpace(c.Name)) == 0 || len(c.Name) > 100 || c.ExpiresInDays < 1 || c.ExpiresInDays > 365 || len(c.Models) == 0 || len(c.Models) > 100 {
		return view, ErrAIInvalid
	}
	access, err := s.GetAccess(ctx, id)
	if err != nil {
		return view, err
	}
	if len(aigateway.AllowedModels(c.Models, access.Models)) != len(c.Models) {
		return view, ErrAIInvalid
	}
	secret, digest, err := aigateway.NewKey()
	if err != nil {
		return view, err
	}
	data, err := json.Marshal(c.Models)
	if err != nil {
		return view, err
	}
	user, _ := actorUserID(ctx)
	key := &model.AIKey{ProxyID: id, UserID: user, Name: strings.TrimSpace(c.Name), Digest: digest, Models: string(data), ExpiresAt: time.Now().Add(time.Duration(c.ExpiresInDays) * 24 * time.Hour)}
	key.TokenLimit = c.TokenLimit
	if err = s.cp.repo.CreateAIKey(ctx, key); err != nil {
		return view, err
	}
	return AIKeyView{ID: key.ID, Name: key.Name, Models: c.Models, ExpiresAt: key.ExpiresAt, Secret: secret, TokenLimit: key.TokenLimit, RemainingTokens: key.TokenLimit}, nil
}

func validAITokenLimit(limit *int64) bool {
	return limit == nil || (*limit >= 0 && *limit <= 1_000_000_000_000)
}

func (s *AIService) UpdateKeyQuota(ctx context.Context, id, keyID uint, limit *int64) error {
	if !validAITokenLimit(limit) {
		return ErrAIInvalid
	}
	if _, _, err := s.access(ctx, id, "update"); err != nil {
		return err
	}
	user, _ := actorUserID(ctx)
	err := s.cp.repo.UpdateAIKeyQuota(ctx, id, user, keyID, limit)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return iam.ErrForbidden
	}
	return err
}

// CheckKeyQuota is deliberately outside Grant's stream revalidation: already
// admitted requests may finish. Every new inference reads settled lifetime use.
func (s *AIService) CheckKeyQuota(ctx context.Context, grant *AIGrant) error {
	if grant == nil {
		return iam.ErrForbidden
	}
	if grant.KeyID == 0 {
		return nil
	} // Playground uses dashboard identity.
	usage, err := s.cp.repo.GetAIKeyUsage(ctx, grant.ProxyID, grant.UserID, grant.KeyID)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return iam.ErrForbidden
	}
	if err != nil {
		return err
	}
	if usage.TokenLimit == nil {
		return nil
	}
	grant.Metered = true
	if usage.UsedTokens >= *usage.TokenLimit {
		return ErrAITokenQuota
	}
	if usage.UnknownRequests > 0 {
		return ErrAITokenUsage
	}
	return nil
}
func (s *AIService) RevokeKey(ctx context.Context, id, keyID uint) error {
	if _, _, err := s.access(ctx, id, "use"); err != nil {
		return err
	}
	user, _ := actorUserID(ctx)
	return s.cp.repo.RevokeAIKey(ctx, id, user, keyID)
}
func (s *AIService) active(ctx context.Context, id uint) (*model.Proxy, *model.Application, *model.AIAccess, error) {
	p, app, err := s.access(ctx, id, "use")
	if err != nil {
		return nil, nil, nil, err
	}
	state, _ := s.cp.proxyEffectiveStatus(p, app)
	if state != proxyEffectiveStatusActive {
		return nil, nil, nil, ErrAIUnavailable
	}
	a, err := s.cp.repo.GetAIAccess(ctx, id)
	if err != nil {
		return nil, nil, nil, err
	}
	if !a.Enabled {
		return nil, nil, nil, ErrAIUnavailable
	}
	return p, app, a, nil
}
func (s *AIService) Grant(ctx context.Context, id uint, secret string) (*AIGrant, error) {
	if len(secret) != 50 || !strings.HasPrefix(secret, "lia_ai_") {
		return nil, iam.ErrForbidden
	}
	digest := aigateway.KeyDigest(secret)
	key, err := s.cp.repo.GetAIKey(ctx, digest)
	if err != nil || key.ProxyID != id {
		return nil, iam.ErrForbidden
	}
	ctx = context.WithValue(ctx, "user_id", key.UserID)
	_, app, a, err := s.active(ctx, id)
	if err != nil {
		return nil, err
	}
	var scope []string
	var mappings map[string]string
	if json.Unmarshal([]byte(key.Models), &scope) != nil || json.Unmarshal([]byte(a.Models), &mappings) != nil {
		return nil, ErrAIUnavailable
	}
	check := func(next context.Context) error {
		next = context.WithValue(next, "user_id", key.UserID)
		if _, err := s.cp.repo.GetAIKey(next, digest); err != nil {
			return iam.ErrForbidden
		}
		_, current, currentAccess, err := s.active(next, id)
		if err != nil {
			return err
		}
		if current.ID != app.ID || current.IP != app.IP || current.Port != app.Port || fmt.Sprint(current.EdgeIDs) != fmt.Sprint(app.EdgeIDs) {
			return ErrAIUnavailable
		}
		if currentAccess.Models != a.Models {
			return ErrAIUnavailable
		}
		return nil
	}
	upstream, credential, protocol, err := s.upstream(ctx, app, id, check)
	if err != nil {
		return nil, err
	}
	return &AIGrant{KeyID: key.ID, UserID: key.UserID, ProxyID: id, Models: aigateway.AllowedModels(scope, mappings), Protocol: protocol, Upstream: upstream, UpstreamKey: credential, Revalidate: check}, nil
}
func (s *AIService) upstream(ctx context.Context, app *model.Application, proxyID uint, check func(context.Context) error) (*aigateway.Upstream, string, string, error) {
	cfg, err := s.cp.repo.GetAIApplication(ctx, app.ID)
	if err != nil {
		return nil, "", "", err
	}
	view := aiAppView(cfg)
	fingerprint := aiFingerprint(app, &view)
	if fingerprint != cfg.TargetFingerprint {
		return nil, "", "", ErrAIUnavailable
	}
	key := ""
	if cfg.EncryptedKey != "" {
		raw, err := base64.StdEncoding.DecodeString(cfg.EncryptedKey)
		if err != nil || len(raw) < s.cipher.NonceSize() {
			return nil, "", "", ErrAIUnavailable
		}
		plain, err := s.cipher.Open(nil, raw[:s.cipher.NonceSize()], raw[s.cipher.NonceSize():], []byte(fingerprint))
		if err != nil {
			return nil, "", "", ErrAIUnavailable
		}
		key = string(plain)
	}
	u, err := aigateway.NewUpstream(aigateway.Target{Host: app.IP, Port: app.Port, TLS: cfg.TLS, BasePath: cfg.BasePath}, func(dialCtx context.Context) (net.Conn, error) {
		if err := check(dialCtx); err != nil {
			return nil, err
		}
		if s.cp.frontierBound == nil {
			return nil, ErrAIUnavailable
		}
		stream, err := s.cp.frontierBound.OpenStream(dialCtx, uint64(app.EdgeIDs[0]))
		if err != nil {
			return nil, err
		}
		conn := trafficconn.TargetConn(stream, s.cp.trafficRecorder, proxyID, app.ID)
		data, err := json.Marshal(proto.Dst{Addr: net.JoinHostPort(app.IP, fmt.Sprint(app.Port)), ApplicationID: app.ID, ProxyID: proxyID})
		if err != nil {
			conn.Close()
			return nil, err
		} // Release failed tunnel; marshal error takes precedence.
		framed := make([]byte, 4+len(data))
		binary.BigEndian.PutUint32(framed, uint32(len(data)))
		copy(framed[4:], data)
		stop := context.AfterFunc(dialCtx, func() { conn.Close() }) // Cancellation must interrupt handshake writes.
		n, err := conn.Write(framed)
		stop()
		if err != nil || n != len(framed) {
			conn.Close()
			if err == nil {
				err = io.ErrShortWrite
			}
			return nil, err
		}
		return conn, nil
	})
	return u, key, cfg.Protocol, err
}
func (s *AIService) Probe(ctx context.Context, id uint) (aigateway.ProbeResult, error) {
	app, err := s.application(ctx, id, "update")
	if err != nil {
		return aigateway.ProbeResult{}, err
	}
	check := func(next context.Context) error {
		uid, _ := actorUserID(ctx)
		next = context.WithValue(next, "user_id", uid)
		current, err := s.application(next, id, "update")
		if err != nil {
			return err
		}
		if current.IP != app.IP || current.Port != app.Port || fmt.Sprint(current.EdgeIDs) != fmt.Sprint(app.EdgeIDs) {
			return ErrAIUnavailable
		}
		return nil
	}
	u, key, protocol, err := s.upstream(ctx, app, 0, check)
	if err != nil {
		return aigateway.ProbeResult{}, err
	}
	defer u.Close()
	return u.Probe(ctx, key, protocol), nil
}
func (s *AIService) Requests(ctx context.Context, id uint) ([]model.AIRequest, error) {
	if _, _, err := s.access(ctx, id, "use"); err != nil {
		return nil, err
	}
	user, _ := actorUserID(ctx)
	return s.cp.repo.ListAIRequests(ctx, id, user, 50)
}

func (s *AIService) TokenUsage(ctx context.Context, id uint) (*model.LLMTokenUsageReport, error) {
	if _, _, err := s.access(ctx, id, "use"); err != nil {
		return nil, err
	}
	user, _ := actorUserID(ctx)
	return s.cp.repo.GetLLMTokenUsage(ctx, id, user, time.Now().UTC().Add(-30*24*time.Hour))
}

func (s *AIService) DebugGrant(ctx context.Context, id uint) (*AIGrant, error) {
	_, app, a, err := s.active(ctx, id)
	if err != nil {
		return nil, err
	}
	user, _ := actorUserID(ctx)
	var models map[string]string
	if json.Unmarshal([]byte(a.Models), &models) != nil {
		return nil, ErrAIUnavailable
	}
	check := func(next context.Context) error {
		next = context.WithValue(next, "user_id", user)
		_, current, currentAccess, err := s.active(next, id)
		if err != nil {
			return err
		}
		if current.ID != app.ID || current.IP != app.IP || current.Port != app.Port || fmt.Sprint(current.EdgeIDs) != fmt.Sprint(app.EdgeIDs) {
			return ErrAIUnavailable
		}
		if currentAccess.Models != a.Models {
			return ErrAIUnavailable
		}
		return nil
	}
	u, key, protocol, err := s.upstream(ctx, app, id, check)
	if err != nil {
		return nil, err
	}
	return &AIGrant{UserID: user, ProxyID: id, Models: models, Protocol: protocol, Upstream: u, UpstreamKey: key, Revalidate: check}, nil
}
func (s *AIService) Record(ctx context.Context, record *model.AIRequest) error {
	return s.cp.repo.RecordAIRequest(ctx, record)
}
