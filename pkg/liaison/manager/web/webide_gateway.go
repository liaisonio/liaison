package web

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/webgateway"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type ideGrant struct {
	token   string
	expires time.Time
	ticket  bool
	project string
	theme   string
}
type ideEndpoint struct {
	mu                               sync.Mutex
	access, instance, origin, prefix string
	grants                           map[string]ideGrant
	server                           *http.Server
	listener                         net.Listener
	ctx                              context.Context
	cancel                           context.CancelFunc
	closed                           bool
	pendingHandoffs                  int
	closeOnce                        sync.Once
	closeErr                         error
}
type webIDEGateway struct {
	mu        sync.Mutex
	web       *web
	conf      *config.Configuration
	endpoints map[string]*ideEndpoint
	cancel    context.CancelFunc
	ctx       context.Context
	closed    bool
	workers   sync.WaitGroup
	gateOnce  sync.Once
	gates     [32]chan struct{}
}

func newWebIDEGateway(web *web, conf *config.Configuration) *webIDEGateway {
	ctx, cancel := context.WithCancel(context.Background())
	g := &webIDEGateway{web: web, conf: conf, endpoints: map[string]*ideEndpoint{}, ctx: ctx, cancel: cancel}
	g.workers.Add(1)
	go g.reapLoop()
	return g
}
func (g *webIDEGateway) tlsConfig() (*tls.Config, error) {
	c := g.conf.Manager
	u, err := url.Parse(c.ServerURL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || !c.Listen.TLS.Enable || (!c.WebIDE.SharedOrigin && (c.WebIDE.PortStart < 1024 || c.WebIDE.PortEnd > 65535 || c.WebIDE.PortEnd < c.WebIDE.PortStart || c.WebIDE.PortEnd-c.WebIDE.PortStart > 63)) {
		return nil, errors.New("configure WebIDE HTTPS port range")
	}
	result := &tls.Config{MinVersion: tls.VersionTLS12}
	for _, pair := range c.Listen.TLS.Certs {
		cert, err := tls.LoadX509KeyPair(pair.Cert, pair.Key)
		if err != nil {
			continue
		}
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil || leaf.VerifyHostname(u.Hostname()) != nil || time.Now().Before(leaf.NotBefore) || time.Now().After(leaf.NotAfter) {
			continue
		}
		result.Certificates = append(result.Certificates, cert)
	}
	if len(result.Certificates) == 0 {
		return nil, errors.New("WebIDE certificate unavailable")
	}
	return result, nil
}
func (g *webIDEGateway) ready() bool { _, err := g.tlsConfig(); return err == nil }
func ideRandom() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}
func ideGrantKey(s string) string { sum := sha256.Sum256([]byte(s)); return hex.EncodeToString(sum[:]) }

func (g *webIDEGateway) launch(ctx context.Context, token, access, instance, project string, themes ...string) (string, error) {
	theme := ""
	if len(themes) > 0 {
		theme = themes[0]
	}
	if theme != "" && theme != "dark" && theme != "light" {
		return "", errors.New("invalid IDE theme")
	}
	release, err := g.acquireAccess(ctx, access)
	if err != nil {
		return "", err
	}
	defer release()
	if len(instance) < 24 || len(instance) > 32 {
		return "", errors.New("invalid IDE instance")
	}
	svc, ok := g.web.controlPlane.(webIDEService)
	if !ok {
		return "", errors.New("IDE unavailable")
	}
	conn, err := svc.OpenWebIDEStream(ctx, access, instance)
	if err != nil {
		return "", err
	}
	if err = conn.Close(); err != nil {
		return "", err
	}
	if err = ctx.Err(); err != nil {
		return "", err
	}
	ticket, err := ideRandom()
	if err != nil {
		return "", err
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return "", errors.New("IDE gateway closed")
	}
	key := access + ":" + instance
	entry := g.endpoints[key]
	g.mu.Unlock()
	if entry == nil {
		tlsConf, err := g.tlsConfig()
		if err != nil {
			return "", err
		}
		public, err := url.Parse(g.conf.Manager.ServerURL)
		if err != nil {
			return "", err
		}
		bind, _, err := net.SplitHostPort(g.conf.Manager.Listen.Addr)
		if err != nil {
			return "", err
		}
		entry = &ideEndpoint{access: access, instance: instance, prefix: "/ide/" + instance + "/", grants: map[string]ideGrant{}}
		if g.conf.Manager.WebIDE.SharedOrigin {
			entry.origin = public.Scheme + "://" + public.Host
			entry.ctx, entry.cancel = context.WithCancel(g.ctx)
		} else {
			var listener net.Listener
			for port := g.conf.Manager.WebIDE.PortStart; port <= g.conf.Manager.WebIDE.PortEnd; port++ {
				listener, err = tls.Listen("tcp", net.JoinHostPort(bind, strconv.Itoa(port)), tlsConf)
				if err == nil {
					entry.origin = "https://" + net.JoinHostPort(public.Hostname(), strconv.Itoa(port))
					break
				}
			}
			if listener == nil {
				return "", errors.New("no WebIDE ports available")
			}
			entry.ctx, entry.cancel = context.WithCancel(g.ctx)
			entry.listener = listener
			entry.server = &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { g.serve(entry, w, r) }), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 64 << 10, BaseContext: func(net.Listener) context.Context { return entry.ctx }}
		}
	}
	// Publish the endpoint and its first grant together. The collector must not
	// observe a newly-created listener without a grant and immediately reap it.
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		if err := entry.close(); err != nil {
			slog.Error("close unregistered WebIDE endpoint", "error", err)
		}
		return "", errors.New("IDE gateway closed")
	}
	if previous := g.endpoints[key]; previous != nil && previous != entry {
		g.mu.Unlock()
		if err := entry.close(); err != nil {
			slog.Error("close superseded WebIDE endpoint", "error", err)
		}
		return "", errors.New("IDE endpoint changed")
	}
	defer g.mu.Unlock()
	if g.endpoints[key] == nil {
		entry.mu.Lock()
		closed := entry.closed
		entry.mu.Unlock()
		if closed {
			return "", errors.New("IDE endpoint expired; reopen it")
		}
		g.endpoints[key] = entry
		if entry.server != nil {
			g.workers.Add(1)
			go g.serveEndpoint(key, entry)
		}
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.closed {
		return "", errors.New("IDE endpoint closed")
	}
	for k, grant := range entry.grants {
		if time.Now().After(grant.expires) {
			delete(entry.grants, k)
		}
	}
	if len(entry.grants)+entry.pendingHandoffs >= 128 {
		return "", errors.New("too many IDE browser sessions")
	}
	entry.grants[ideGrantKey(ticket)] = ideGrant{token: token, expires: time.Now().Add(30 * time.Second), ticket: true, project: project, theme: theme}
	return entry.origin + entry.prefix + "?__liaison_ticket=" + ticket, nil
}

// ServeHTTP dispatches only previously authorized instances. The path never
// selects an arbitrary upstream, nor bypasses the per-instance browser grant.
func (g *webIDEGateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	var entry *ideEndpoint
	for _, candidate := range g.endpoints {
		if strings.HasPrefix(r.URL.Path, candidate.prefix) {
			entry = candidate
			break
		}
	}
	g.mu.Unlock()
	if entry == nil {
		http.Error(w, "IDE session expired. Reopen it from Liaison.", http.StatusGone)
		return
	}
	g.serve(entry, w, r)
}

func (g *webIDEGateway) authorize(ctx context.Context, token, access string) (context.Context, error) {
	r, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://localhost/", nil)
	if err != nil {
		return nil, err
	}
	r.Header.Set("Authorization", "Bearer "+token)
	actor, err := g.web.authenticateHTTP(r)
	if err != nil {
		return nil, err
	}
	if actor.Status != model.UserStatusActive || g.web.iamService == nil {
		return nil, errors.New("IDE permission unavailable")
	}
	if err = g.web.iamService.RequireResourcePermission(actor, "accesses", "use"); err != nil {
		return nil, err
	}
	ctx = context.WithValue(ctx, "user_id", actor.ID)
	ctx = context.WithValue(ctx, "user", actor)
	svc, ok := g.web.controlPlane.(webIDEService)
	if !ok {
		return nil, errors.New("IDE unavailable")
	}
	if _, _, err = svc.WebIDETarget(ctx, access); err != nil {
		return nil, err
	}
	return ctx, nil
}

func (g *webIDEGateway) serve(entry *ideEndpoint, w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	if entry.ctx != nil {
		stop := context.AfterFunc(entry.ctx, cancel)
		defer stop()
	}
	r = r.WithContext(ctx)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	u, _ := url.Parse(entry.origin)
	if r.TLS == nil || !strings.EqualFold(r.Host, u.Host) || !strings.HasPrefix(r.URL.Path, entry.prefix) {
		http.NotFound(w, r)
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" && origin != entry.origin {
		http.Error(w, "Forbidden origin", 403)
		return
	}
	// Same-site cookies cross TCP ports; Origin plus Fetch Metadata prevents
	// another port's page from driving credentialed requests into this IDE.
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" && r.Header.Get("Sec-Fetch-Mode") != "navigate" {
		http.Error(w, "Forbidden source", 403)
		return
	}
	ticket := r.URL.Query().Get("__liaison_ticket")
	cookieName := "liaison_web_ide_" + entry.instance
	key := ""
	if ticket != "" {
		key = ideGrantKey(ticket)
	} else if cookie, err := r.Cookie(cookieName); err == nil {
		key = ideGrantKey(cookie.Value)
	}
	entry.mu.Lock()
	grant, ok := entry.grants[key]
	if entry.closed || !ok || time.Now().After(grant.expires) || grant.ticket != (ticket != "") {
		entry.mu.Unlock()
		http.Error(w, "Open this IDE from Liaison to sign in", 401)
		return
	}
	if ticket != "" {
		delete(entry.grants, key)
		entry.pendingHandoffs++
		defer func() {
			entry.mu.Lock()
			entry.pendingHandoffs--
			entry.mu.Unlock()
		}()
	}
	entry.mu.Unlock()
	ctx, err := g.authorize(r.Context(), grant.token, entry.access)
	if err != nil {
		http.Error(w, "Access revoked or expired", 403)
		return
	}
	if ticket != "" {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", 405)
			return
		}
		secret, err := ideRandom()
		if err != nil {
			http.Error(w, "Unavailable", 503)
			return
		}
		grant.ticket = false
		grant.expires = time.Now().Add(12 * time.Hour)
		entry.mu.Lock()
		if entry.closed {
			entry.mu.Unlock()
			http.Error(w, "IDE endpoint closed", http.StatusGone)
			return
		}
		entry.grants[ideGrantKey(secret)] = grant
		entry.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: cookieName, Value: secret, Path: entry.prefix, Secure: true, HttpOnly: true, SameSite: http.SameSiteStrictMode})
		target := entry.prefix
		if grant.project != "" {
			target += "?folder=" + url.QueryEscape(grant.project)
		}
		http.Redirect(w, r, target, http.StatusSeeOther)
		return
	}
	if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") && r.Header.Get("Origin") != entry.origin {
		http.Error(w, "Forbidden origin", 403)
		return
	}
	// Reject ambiguous paths before the upstream can normalize them differently.
	clean := path.Clean(r.URL.Path)
	if strings.Contains(r.URL.Path, "\\") || (clean != strings.TrimSuffix(r.URL.Path, "/")) || !strings.HasPrefix(clean+"/", entry.prefix) {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, entry.prefix)
	if g.conf != nil && (rel == "_static/src/browser/media/favicon.ico" || rel == "_static/src/browser/media/favicon-dark-support.svg") && (r.Method == http.MethodGet || r.Method == http.MethodHead) {
		w.Header().Set("Content-Type", "image/x-icon")
		http.ServeFile(w, r, filepath.Join(g.conf.Manager.WebDir, "favicon.ico"))
		return
	}
	if rel == "proxy" || rel == "absproxy" || strings.HasPrefix(rel, "proxy/") || strings.HasPrefix(rel, "absproxy/") {
		http.Error(w, "Use an authorized development preview", 403)
		return
	}
	// Revocation also closes an already-upgraded WebSocket, not just new HTTP.
	go func() {
		defer func() {
			if recover() != nil {
				cancel()
				slog.Error("WebIDE authorization monitor panic")
			}
		}()
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Now().After(grant.expires) {
					cancel()
					return
				}
				if _, err := g.authorize(ctx, grant.token, entry.access); err != nil {
					cancel()
					return
				}
			}
		}
	}()
	svc := g.web.controlPlane.(webIDEService)
	target := &url.URL{Scheme: "http", Host: "localhost"}
	p := webgateway.New(target, entry.prefix, func(c context.Context) (net.Conn, error) {
		conn, err := svc.OpenWebIDEStream(c, entry.access, entry.instance)
		if err != nil {
			return nil, err
		}
		stop := context.AfterFunc(ctx, func() { conn.Close() })
		return &ideConn{Conn: conn, stop: stop}, nil
	})
	// A geminio stream close can discard unread receive buffers. Do not ask
	// code-server to close after writing: first consume the complete HTTP body,
	// then release this request's transport. No pool is shared across grants.
	transport := p.Transport.(*http.Transport)
	transport.DisableKeepAlives = false
	defer transport.CloseIdleConnections()
	// code-server compares Origin with X-Forwarded-Host, not only Host. The
	// generic proxy rewrites Origin to its upstream; preserve the already
	// validated public origin here so both sides of that comparison agree.
	rewrite := p.Rewrite
	p.Rewrite = func(request *httputil.ProxyRequest) {
		rewrite(request)
		if rel == "" && g.conf != nil {
			request.Out.Header.Set("Accept-Encoding", "identity")
		}
		// A downstream Connection: close applies to the browser connection,
		// not to the independently managed connector stream.
		request.Out.Close = false
		if origin := request.In.Header.Get("Origin"); origin != "" {
			request.Out.Header.Set("Origin", entry.origin)
		}
	}
	if rel == "" && g.conf != nil {
		modify := p.ModifyResponse
		p.ModifyResponse = func(response *http.Response) error {
			if err := modify(response); err != nil {
				return err
			}
			return brandIDEPage(response, grant.theme)
		}
	}
	p.ServeHTTP(w, r.WithContext(ctx))
}

type ideConn struct {
	net.Conn
	stop func() bool
}

func (c *ideConn) Close() error { c.stop(); return c.Conn.Close() }
func (g *webIDEGateway) Close() error {
	g.mu.Lock()
	g.closed = true
	g.cancel()
	entries := g.endpoints
	g.endpoints = map[string]*ideEndpoint{}
	g.mu.Unlock()
	var errs []error
	for _, entry := range entries {
		errs = append(errs, entry.close())
	}
	g.workers.Wait()
	return errors.Join(errs...)
}
