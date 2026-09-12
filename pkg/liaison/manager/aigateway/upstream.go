// Package aigateway contains the connector-only AI API transport foundation.
// It does not expose routes or bypass control-plane resource authorization.
package aigateway

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"
)

// ConnectorDial must dial an already-authorized application via its connector.
// The production adapter must recheck resource state; a system net.Dialer is not
// a valid production implementation. No caller-controlled address is supplied.
type ConnectorDial func(context.Context) (net.Conn, error)

type Target struct {
	Host     string
	Port     int
	TLS      bool
	BasePath string // API root, e.g. /v1; never a full URL.
}

type Upstream struct {
	client    *http.Client
	transport *http.Transport
	base      string
}

var ErrInvalidTarget = errors.New("invalid AI API target")

func NewUpstream(target Target, dial ConnectorDial) (*Upstream, error) {
	if dial == nil || target.Host == "" || strings.TrimSpace(target.Host) != target.Host ||
		strings.ContainsAny(target.Host, "/\\@?#%\r\n\t ") || target.Port < 1 || target.Port > 65535 {
		return nil, ErrInvalidTarget
	}
	if strings.Contains(target.Host, ":") && net.ParseIP(target.Host) == nil {
		return nil, ErrInvalidTarget
	}
	basePath := strings.TrimSuffix(target.BasePath, "/")
	if basePath != "" && (!strings.HasPrefix(basePath, "/") || path.Clean(basePath) != basePath ||
		strings.ContainsAny(basePath, "\\?#%\r\n\t ")) {
		return nil, ErrInvalidTarget
	}
	address := net.JoinHostPort(target.Host, strconv.Itoa(target.Port))
	scheme := "http"
	if target.TLS {
		scheme = "https"
	}
	u := &url.URL{Scheme: scheme, Host: address, Path: basePath}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			if network != "tcp" || addr != address {
				return nil, ErrInvalidTarget
			}
			conn, err := dial(ctx)
			if err == nil && conn == nil {
				return nil, errors.New("connector returned no connection")
			}
			return conn, err
		},
		TLSClientConfig:        &tls.Config{MinVersion: tls.VersionTLS12, ServerName: target.Host},
		TLSHandshakeTimeout:    10 * time.Second,
		ResponseHeaderTimeout:  30 * time.Second,
		DisableKeepAlives:      true, // Every request traverses the authorized dialer.
		DisableCompression:     true,
		MaxResponseHeaderBytes: 64 << 10,
	}
	return &Upstream{base: u.String(), transport: transport, client: &http.Client{
		Transport:     transport,
		Timeout:       5 * time.Minute,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (u *Upstream) Close() { u.transport.CloseIdleConnections() }

// Request only accepts implemented operations. It builds headers from scratch:
// client cookies, bearer keys, forwarding headers and host overrides never pass.
// Caller owns response.Body and must close it. A streaming handler must terminate
// the downstream response on read errors rather than synthesize successful EOF.
func (u *Upstream) Request(ctx context.Context, method, operation, upstreamKey string, body io.Reader) (*http.Response, error) {
	return u.RequestProtocol(ctx, method, operation, upstreamKey, "openai-compatible", body)
}

func (u *Upstream) RequestProtocol(ctx context.Context, method, operation, upstreamKey, protocol string, body io.Reader) (*http.Response, error) {
	if !(method == http.MethodGet && operation == "models" ||
		method == http.MethodPost && (protocol == "openai-compatible" && operation == "chat/completions" || protocol == "anthropic" && operation == "messages")) {
		return nil, errors.New("unsupported AI API operation")
	}
	if strings.ContainsAny(upstreamKey, "\r\n") {
		return nil, errors.New("invalid upstream credential")
	}
	req, err := http.NewRequestWithContext(ctx, method, u.base+"/"+operation, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if protocol == "anthropic" {
		req.Header.Set("anthropic-version", "2023-06-01")
		if upstreamKey != "" {
			req.Header.Set("x-api-key", upstreamKey)
		}
	} else if protocol != "openai-compatible" {
		return nil, errors.New("unsupported upstream protocol")
	} else if upstreamKey != "" {
		req.Header.Set("Authorization", "Bearer "+upstreamKey)
	}
	return u.client.Do(req)
}

type ProbeResult struct {
	State    string   `json:"state"` // compatible, auth_required, unknown, unreachable
	Protocol string   `json:"protocol,omitempty"`
	Models   []string `json:"models,omitempty"`
}

// ProbeOpenAI observes a models response, not vendor identity or inference support.
// No upstream error text is returned: it can contain credentials or internal data.
func (u *Upstream) ProbeOpenAI(ctx context.Context, key string) ProbeResult {
	return u.Probe(ctx, key, "openai-compatible")
}

func (u *Upstream) Probe(ctx context.Context, key, protocol string) ProbeResult {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	resp, err := u.RequestProtocol(ctx, http.MethodGet, "models", key, protocol, nil)
	if err != nil {
		return ProbeResult{State: "unreachable"}
	}
	defer resp.Body.Close()
	if resp.StatusCode == 401 || resp.StatusCode == 403 {
		return ProbeResult{State: "auth_required"}
	}
	if resp.StatusCode != 200 {
		return ProbeResult{State: "unknown"}
	}
	const maxBody = 1 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil || len(data) > maxBody {
		return ProbeResult{State: "unknown"}
	}
	var result struct {
		Object  string `json:"object"`
		HasMore *bool  `json:"has_more"`
		Data    *[]struct {
			ID   string `json:"id"`
			Type string `json:"type"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &result) != nil || (protocol == "openai-compatible" && result.Object != "list") || result.Data == nil {
		return ProbeResult{State: "unknown"}
	}
	if protocol == "anthropic" && result.HasMore == nil {
		return ProbeResult{State: "unknown"}
	}
	models := make([]string, 0, len(*result.Data))
	seen := map[string]bool{}
	for _, model := range *result.Data {
		if protocol == "anthropic" && model.Type != "model" {
			return ProbeResult{State: "unknown"}
		}
		if !validModel(model.ID) {
			return ProbeResult{State: "unknown"}
		}
		if !seen[model.ID] {
			models = append(models, model.ID)
			seen[model.ID] = true
		}
	}
	return ProbeResult{State: "compatible", Protocol: protocol, Models: models}
}
