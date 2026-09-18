// Package webgateway implements shared HTTP entries over connector streams.
// Authentication and access selection belong to the caller, before ServeHTTP.
package webgateway

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"
)

const CookiePrefix = "liaison_web_"

// This is credential redaction, not authentication: even expired console JWTs
// must not reach an application. Upstream Basic/Bearer credentials still work.
func consoleAuthorization(value string) bool {
	parts := strings.Fields(value)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return false
	}
	if strings.HasPrefix(parts[1], "liaison_pat_") {
		return true
	}
	jwtParts := strings.Split(parts[1], ".")
	if len(jwtParts) != 3 {
		return false
	}
	payload, err := base64.RawURLEncoding.DecodeString(jwtParts[1])
	if err != nil {
		return false
	}
	var claims struct {
		Issuer string `json:"iss"`
	}
	return json.Unmarshal(payload, &claims) == nil && claims.Issuer == "liaison"
}

// New creates a proxy for one authorized request. Dial must open only the
// selected application's connector stream, never an address supplied by clients.
// Prefix is empty for domain entries and ends in '/' for path entries.
func New(target *url.URL, prefix string, dial func(context.Context) (net.Conn, error)) *httputil.ReverseProxy {
	cookieNamespace := "liaison_app_" + base64.RawURLEncoding.EncodeToString([]byte(prefix)) + "_"
	transport := &http.Transport{
		Proxy:                  nil,
		DialContext:            func(ctx context.Context, _, _ string) (net.Conn, error) { return dial(ctx) },
		DisableKeepAlives:      true,
		ResponseHeaderTimeout:  30 * time.Second,
		TLSHandshakeTimeout:    10 * time.Second,
		MaxResponseHeaderBytes: 1 << 20,
	}
	return &httputil.ReverseProxy{
		Transport:     transport,
		FlushInterval: -1,
		Rewrite: func(p *httputil.ProxyRequest) {
			p.Out.URL.Scheme, p.Out.URL.Host = target.Scheme, target.Host
			p.Out.Host = target.Host
			if prefix != "" {
				p.Out.URL.Path = "/" + strings.TrimPrefix(p.In.URL.Path, prefix)
				if p.In.URL.RawPath != "" {
					p.Out.URL.RawPath = "/" + strings.TrimPrefix(p.In.URL.RawPath, prefix)
				}
			}
			for _, value := range p.In.Header.Values("Authorization") {
				if consoleAuthorization(value) {
					p.Out.Header.Del("Authorization")
					break
				}
			}
			p.Out.Header.Del("Proxy-Authorization")
			p.Out.Header.Del("Cookie")
			for _, cookie := range p.In.Cookies() {
				if prefix != "" {
					if !strings.HasPrefix(cookie.Name, cookieNamespace) {
						continue
					}
					name, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(cookie.Name, cookieNamespace))
					if err != nil {
						continue
					}
					cookie.Name = string(name)
				}
				if !strings.HasPrefix(cookie.Name, CookiePrefix) {
					p.Out.AddCookie(cookie)
				}
			}
			p.Out.Header.Del("X-Forwarded-Prefix")
			p.SetXForwarded()
			if prefix != "" {
				p.Out.Header.Set("X-Forwarded-Prefix", strings.TrimSuffix(prefix, "/"))
			}
			// Preserve source-site origin checks without allowing caller-supplied
			// external origins to masquerade as the upstream.
			for _, header := range []string{"Origin", "Referer"} {
				u, err := url.Parse(p.In.Header.Get(header))
				if err == nil && u.Host == p.In.Host && (u.Scheme == "http" || u.Scheme == "https") {
					u.Scheme, u.Host = target.Scheme, target.Host
					if prefix != "" && strings.HasPrefix(u.Path, prefix) {
						u.Path = "/" + strings.TrimPrefix(u.Path, prefix)
						u.RawPath = ""
					}
					p.Out.Header.Set(header, u.String())
				}
			}
		},
		ModifyResponse: func(response *http.Response) error {
			// A proxied site must not clear console cookies/storage or register a
			// service worker outside its own path.
			response.Header.Del("Clear-Site-Data")
			response.Header.Del("Service-Worker-Allowed")
			if location := response.Header.Get("Location"); location != "" {
				u, err := url.Parse(location)
				if err != nil {
					return errors.New("invalid upstream redirect")
				}
				if u.Host == "" && u.Scheme == "" && prefix != "" {
					u = response.Request.URL.ResolveReference(u)
				}
				if strings.EqualFold(u.Host, target.Host) {
					u.Scheme, u.Host = "", ""
					escaped := strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(u.EscapedPath(), "/")
					u.Path, err = url.PathUnescape(escaped)
					if err != nil {
						return err
					}
					u.RawPath = escaped
					response.Header.Set("Location", u.String())
				}
			}
			cookies := response.Cookies()
			response.Header.Del("Set-Cookie")
			for _, cookie := range cookies {
				if strings.HasPrefix(cookie.Name, CookiePrefix) {
					continue
				}
				cookie.Domain = ""
				if prefix != "" {
					if strings.HasPrefix(cookie.Name, "__Host-") {
						continue
					}
					cookie.Path = strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(cookie.Path, "/")
					cookie.Name = cookieNamespace + base64.RawURLEncoding.EncodeToString([]byte(cookie.Name))
				}
				response.Header.Add("Set-Cookie", cookie.String())
			}
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			http.Error(w, "Upstream unavailable", http.StatusBadGateway)
		},
	}
}
