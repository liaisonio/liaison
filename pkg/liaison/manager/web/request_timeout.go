package web

import (
	"context"
	"net/http"
	"strings"
	"time"
)

func requestTimeoutFilter(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		timeout := time.Second
		switch {
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
