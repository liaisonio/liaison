package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/webgateway"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type httpEntryBackend interface {
	HTTPEntryTarget(context.Context, uint) (*controlplane.HTTPEntryTarget, error)
	OpenHTTPEntryStream(context.Context, uint) (net.Conn, error)
	HTTPEntrySourceAllowed(uint, string) bool
}

type httpEntryGrant struct {
	token   string
	id      uint
	mode    string
	expires time.Time
	ticket  bool
}
type httpEntries struct {
	mu     sync.Mutex
	grants map[string]httpEntryGrant
	conf   *config.Configuration
}

func (s *httpEntries) issue(grant httpEntryGrant) (string, bool) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, g := range s.grants {
		if time.Now().After(g.expires) {
			delete(s.grants, key)
		}
	}
	if len(s.grants) >= 4096 {
		return "", false
	}
	key := hex.EncodeToString(b)
	s.grants[key] = grant
	return key, true
}
func (s *httpEntries) get(key string, id uint, mode string, ticket bool) (httpEntryGrant, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	g, ok := s.grants[key]
	if !ok || g.id != id || g.mode != mode || g.ticket != ticket || time.Now().After(g.expires) {
		return httpEntryGrant{}, false
	}
	if ticket {
		delete(s.grants, key)
	}
	return g, true
}

func (web *web) handleHTTPEntryAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if actor.Status != model.UserStatusActive {
		http.Error(w, "Forbidden", 403)
		return
	}
	if r.URL.Path == "/api/v1/web-entries/capabilities" && r.Method == "GET" {
		writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]bool{"domain": web.httpEntries.conf.Manager.WebDomainReady()}})
		return
	}
	if web.iamService.RequireResourcePermission(actor, "accesses", "use") != nil {
		http.Error(w, "Forbidden", 403)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/web-entries/"), "/")
	if len(parts) != 2 || parts[1] != "launch" || r.Method != "POST" {
		http.NotFound(w, r)
		return
	}
	id, e := strconv.ParseUint(parts[0], 10, 32)
	if e != nil || id == 0 {
		http.Error(w, "Invalid access", 400)
		return
	}
	backend, ok := web.controlPlane.(httpEntryBackend)
	if !ok {
		http.Error(w, "Unavailable", 503)
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", actor.ID)
	target, err := backend.HTTPEntryTarget(ctx, uint(id))
	if err != nil {
		http.Error(w, "Access unavailable", 403)
		return
	}
	if !backend.HTTPEntrySourceAllowed(uint(id), r.RemoteAddr) {
		http.Error(w, "Forbidden", 403)
		return
	}
	token, ok := bearerToken(r)
	if !ok {
		writeUnauthorized(w)
		return
	}
	ticket, ok := web.httpEntries.issue(httpEntryGrant{token: token, id: uint(id), mode: target.Mode, expires: time.Now().Add(30 * time.Second), ticket: true})
	if !ok {
		http.Error(w, "Try again later", 503)
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]string{"url": target.URL + "?__liaison_ticket=" + ticket}})
}

// httpEntryFilter runs before SPA/API routing so an application host cannot
// accidentally expose management routes, even when its access has been deleted.
func (web *web) httpEntryFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s := web.httpEntries
		host := strings.ToLower(r.Host)
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		domain := strings.ToLower(strings.TrimSpace(s.conf.Manager.WebDomain))
		consoleHost := ""
		if consoleURL, err := url.Parse(s.conf.Manager.ServerURL); err == nil {
			consoleHost = strings.ToLower(consoleURL.Hostname())
		}
		mode, prefix, idText := "", "", ""
		if domain != "" && host != consoleHost && (host == domain || strings.HasSuffix(host, "."+domain)) {
			if !s.conf.Manager.WebDomainReady() {
				http.Error(w, "Domain unavailable", 503)
				return
			}
			label := strings.TrimSuffix(host, "."+domain)
			if !strings.HasPrefix(label, "a-") {
				http.NotFound(w, r)
				return
			}
			mode, idText = "domain", strings.TrimPrefix(label, "a-")
		} else if strings.HasPrefix(r.URL.Path, "/access/") {
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/access/"), "/", 3)
			if len(parts) < 2 || parts[1] != "web" {
				next.ServeHTTP(w, r)
				return
			}
			mode, idText = "path", parts[0]
			prefix = "/access/" + idText + "/web/"
		} else if strings.HasPrefix(r.URL.Path, "/_liaison/") {
			parts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/_liaison/a/"), "/", 2)
			if !strings.HasPrefix(r.URL.Path, "/_liaison/a/") || len(parts) != 2 {
				http.NotFound(w, r)
				return
			}
			mode, idText = "path", parts[0]
			prefix = "/_liaison/a/" + idText + "/"
		} else {
			next.ServeHTTP(w, r)
			return
		}
		id, err := strconv.ParseUint(idText, 10, 32)
		if err != nil || id == 0 || strconv.FormatUint(id, 10) != idText {
			http.NotFound(w, r)
			return
		}
		// Keep relative links inside the website when the entry slash is omitted.
		if mode == "path" && r.URL.Path == strings.TrimSuffix(prefix, "/") {
			if r.URL.RawPath != "" && r.URL.RawPath != r.URL.Path {
				http.Error(w, "Invalid path", 400)
				return
			}
			location := prefix
			if r.URL.RawQuery != "" {
				location += "?" + r.URL.RawQuery
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			http.Redirect(w, r, location, http.StatusTemporaryRedirect)
			return
		}
		if r.URL.RawPath != "" && prefix != "" && !strings.HasPrefix(r.URL.RawPath, prefix) {
			http.Error(w, "Invalid path", 400)
			return
		}
		backend, ok := web.controlPlane.(httpEntryBackend)
		if !ok {
			http.Error(w, "Unavailable", 503)
			return
		}
		cookieName := webgateway.CookiePrefix + idText
		key := r.URL.Query().Get("__liaison_ticket")
		isTicket := key != ""
		if isTicket && (r.Method != "GET" || r.URL.Path != prefix && mode == "path" || mode == "domain" && r.URL.Path != "/") {
			http.Error(w, "Invalid launch", 400)
			return
		}
		if !isTicket {
			if c, e := r.Cookie(cookieName); e == nil {
				key = c.Value
			}
		}
		grant, ok := s.get(key, uint(id), mode, isTicket)
		if !ok {
			http.Error(w, "Open this access from Liaison to sign in.", 401)
			return
		}
		authRequest := r.Clone(r.Context())
		authRequest.Header.Set("Authorization", "Bearer "+grant.token)
		actor, err := web.authenticateHTTP(authRequest)
		if err != nil || actor.Status != model.UserStatusActive {
			writeUnauthorized(w)
			return
		}
		if web.iamService.RequireResourcePermission(actor, "accesses", "use") != nil || !backend.HTTPEntrySourceAllowed(uint(id), r.RemoteAddr) {
			http.Error(w, "Forbidden", 403)
			return
		}
		ctx := context.WithValue(r.Context(), "user_id", actor.ID)
		target, err := backend.HTTPEntryTarget(ctx, uint(id))
		if err != nil || target.Mode != mode {
			http.Error(w, "Access unavailable", 403)
			return
		}
		if isTicket {
			grant.ticket = false
			grant.expires = time.Now().Add(time.Hour)
			key, ok = s.issue(grant)
			if !ok {
				http.Error(w, "Try again later", 503)
				return
			}
			cookiePath := prefix
			if cookiePath == "" {
				cookiePath = "/"
			}
			http.SetCookie(w, &http.Cookie{Name: cookieName, Value: key, Path: cookiePath, HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteLaxMode, MaxAge: 3600})
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("Referrer-Policy", "no-referrer")
			http.Redirect(w, r, cookiePath, 303)
			return
		}
		upstream := &url.URL{Scheme: "http", Host: target.Address}
		webgateway.New(upstream, prefix, func(c context.Context) (net.Conn, error) { return backend.OpenHTTPEntryStream(c, uint(id)) }).ServeHTTP(w, r.WithContext(ctx))
	})
}
