package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/stretchr/testify/require"
)

type openingHTTPPlane struct {
	controlplane.ControlPlane
	calls int
	err   error
}

func (p *openingHTTPPlane) TCPOpening(ctx context.Context, id uint, requestID, operation string) (*controlplane.TCPOpeningData, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return &controlplane.TCPOpeningData{RequestID: requestID, State: "closed"}, nil
}
func TestTCPOpeningHTTPBoundary(t *testing.T) {
	w, admin, _ := newPermissionHTTPTest(t)
	cp := &openingHTTPPlane{}
	w.controlPlane = cp
	call := func(method, path, body, origin string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), "user", admin))
		if auth {
			r.Header.Set("Authorization", "Bearer test-token")
		}
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		rec := httptest.NewRecorder()
		w.handleTCPOpeningHTTP(rec, r)
		return rec
	}
	base := "/api/v1/proxies/1/tcp_openings"
	require.Equal(t, 401, call("POST", base, `{"request_id":"request-000000001"}`, "", false).Code)
	require.Equal(t, 403, call("POST", base, `{"request_id":"request-000000001"}`, "https://cross-site.example", true).Code)
	require.Equal(t, 400, call("POST", base, `{"request_id":"request-000000001","duration":9999}`, "", true).Code)
	require.Equal(t, 400, call("POST", base, `{"request_id":"request-000000001"} {}`, "", true).Code)
	require.Equal(t, 400, call("GET", base+"/request-000000001?owner=2", "", "", true).Code)
	require.Equal(t, 405, call("GET", base, "", "", true).Code)
	require.Zero(t, cp.calls)
	require.Equal(t, 200, call("POST", base, `{"request_id":"request-000000001"}`, "", true).Code)
	require.Equal(t, 200, call("GET", base+"/request-000000001", "", "", true).Code)
	require.Equal(t, 200, call("DELETE", base+"/request-000000001", "", "", true).Code)
	cp.err = errors.New("sensitive internal diagnostic")
	response := call("POST", base, `{"request_id":"request-000000001"}`, "", true)
	require.Equal(t, 503, response.Code)
	require.NotContains(t, response.Body.String(), "sensitive")
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
}
