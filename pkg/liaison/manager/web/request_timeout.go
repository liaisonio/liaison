package web

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func requestTimeoutFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// IDE HTTP streams and upgraded terminal connections live until the
		// client disconnects or the gateway revokes their instance grant.
		if strings.HasPrefix(r.URL.Path, "/ide/") {
			next.ServeHTTP(w, r)
			return
		}
		timeout := time.Second
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/webide/"):
			timeout = 30 * time.Second
		case strings.HasPrefix(r.URL.Path, "/api/v1/edge-agents"):
			timeout = 25 * time.Second
		case r.URL.Path == "/api/v1/applications/probe":
			timeout = 8 * time.Second
		case strings.HasPrefix(r.URL.Path, "/api/v1/ai/"):
			timeout = 5 * time.Minute
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/sessions/") && strings.HasSuffix(r.URL.Path, "/events"):
			timeout = 30 * time.Minute
		case strings.HasPrefix(r.URL.Path, "/api/v1/agent/sessions/"):
			timeout = 5 * time.Minute
		case r.URL.Path == "/api/v1/settings/model/test":
			timeout = 30 * time.Second
		case r.URL.Path == "/api/v1/assistance/suggestions":
			timeout = 16 * time.Second
		case strings.HasPrefix(r.URL.Path, "/api/v1/webdata/sessions/"):
			timeout = webDataExecuteTimeout + time.Second
		case strings.HasPrefix(r.URL.Path, "/api/v1/webdata/proxies/"):
			timeout = webDataConnectTimeout + time.Second
		}
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
