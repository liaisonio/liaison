package web

import (
	"context"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestAgentDeadlineAndClientCancellation(t *testing.T) {
	for _, tc := range []struct {
		path    string
		minimum time.Duration
	}{
		{"/api/v1/agent/sessions/s/turns", 4 * time.Minute},
		{"/api/v1/settings/model/test", 20 * time.Second},
		{"/api/v1/assistance/suggestions", 5 * time.Second},
		{"/api/v1/webdata/sessions/s/execute", 2 * time.Minute},
		{"/api/v1/webdata/proxies/3/session", 12 * time.Second},
		{"/api/v1/proxies", 0},
	} {
		server := kratoshttp.NewServer(kratoshttp.Timeout(0), kratoshttp.Filter(requestTimeoutFilter))
		server.HandleFunc(tc.path, func(_ http.ResponseWriter, r *http.Request) {
			deadline, ok := r.Context().Deadline()
			if !ok {
				t.Fatal("unbounded request")
			}
			remaining := time.Until(deadline)
			if remaining < tc.minimum {
				t.Fatalf("deadline too short: %s %v", tc.path, remaining)
			}
			if tc.minimum == 0 && remaining > time.Second {
				t.Fatal("ordinary request timeout changed")
			}
		})
		server.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", tc.path, nil))
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	requestTimeoutFilter(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if r.Context().Err() != context.Canceled {
			t.Fatal("client cancellation lost")
		}
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("POST", "/api/v1/agent/sessions/s/turns", nil).WithContext(ctx))
}
