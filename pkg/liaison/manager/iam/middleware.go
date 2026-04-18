package iam

import (
	"context"
	"net"
	"net/http"
	"strings"

	"github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// AuthMiddleware validates the Authorization header and injects user info
// into the context. It handles both JWT session tokens and PATs
// (Personal Access Tokens) — the latter are detected by the
// "liaison_pat_" prefix and routed through the PAT verifier.
func AuthMiddleware(iamService *IAMService) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if httpReq, ok := kratoshttp.RequestFromServerContext(ctx); ok {
				path := httpReq.URL.Path
				if isIAMEndpoint(path) {
					log.Debugf("Skipping IAM endpoint authentication: %s", path)
					return handler(ctx, req)
				}

				authHeader := httpReq.Header.Get("Authorization")
				if authHeader == "" {
					log.Warnf("No authentication token provided")
					return nil, errors.New(http.StatusUnauthorized, "UNAUTHORIZED", "No authentication token provided")
				}

				tokenParts := strings.SplitN(authHeader, " ", 2)
				if len(tokenParts) != 2 || tokenParts[0] != "Bearer" {
					log.Warnf("Invalid token format")
					return nil, errors.New(http.StatusUnauthorized, "UNAUTHORIZED", "Invalid token format")
				}

				tokenString := tokenParts[1]

				var user *model.User
				var err error
				if strings.HasPrefix(tokenString, model.PATPlaintextPrefix) {
					user, err = iamService.GetUserByPAT(tokenString, ExtractClientIP(httpReq))
				} else {
					user, err = iamService.GetUserByToken(tokenString)
				}
				if err != nil {
					log.Warnf("Token validation failed: %v", err)
					return nil, errors.New(http.StatusUnauthorized, "UNAUTHORIZED", "Token validation failed")
				}

				ctx = context.WithValue(ctx, "user_id", user.ID)
				ctx = context.WithValue(ctx, "user_email", user.Email)
				ctx = context.WithValue(ctx, "user", user)

				log.Debugf("User authentication successful: %s", user.Email)
			}

			return handler(ctx, req)
		}
	}
}

// ExtractClientIP returns the most trustworthy client IP from the request
// headers. X-Real-IP is preferred over X-Forwarded-For because the latter
// can be spoofed by the client.
func ExtractClientIP(r *http.Request) string {
	if v := strings.TrimSpace(r.Header.Get("X-Real-IP")); v != "" {
		return v
	}
	if v := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); v != "" {
		if idx := strings.Index(v, ","); idx > 0 {
			return strings.TrimSpace(v[:idx])
		}
		return v
	}
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// isIAMEndpoint returns true for request paths that do NOT require the
// middleware to authenticate. These either need no auth at all (login, static
// install scripts) or handle auth themselves (custom HandleFunc routes like
// PAT and firewall management).
func isIAMEndpoint(path string) bool {
	noAuthPaths := []string{
		"/api/v1/iam/login",
		"/api/v1/iam/logout",
		"/api/v1/iam/client_ip",
		"/install.sh",
		"/install.ps1",
		"/install.bat",
		"/uninstall.sh",
		"/uninstall.ps1",
	}

	if strings.HasPrefix(path, "/packages/") {
		return true
	}
	// PAT management — handler authenticates itself (session or PAT).
	if path == "/api/v1/iam/tokens" || strings.HasPrefix(path, "/api/v1/iam/tokens/") {
		return true
	}
	// Per-proxy firewall — handler authenticates itself.
	if strings.HasPrefix(path, "/api/v1/proxies/") && strings.HasSuffix(path, "/firewall") {
		return true
	}

	for _, noAuthPath := range noAuthPaths {
		if path == noAuthPath {
			return true
		}
	}

	return false
}
