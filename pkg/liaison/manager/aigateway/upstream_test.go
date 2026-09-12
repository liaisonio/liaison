package aigateway

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The fake connector is the ONLY component that knows the local server address.
// model.internal is deliberately not resolvable through the system network.
func testUpstream(t *testing.T, handler http.HandlerFunc) *Upstream {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	u, err := NewUpstream(Target{Host: "model.internal", Port: 8080, BasePath: "/v1"}, func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	})
	require.NoError(t, err)
	t.Cleanup(u.Close)
	return u
}

func TestProbe_OnlyMetadataAndUpstreamCredential(t *testing.T) {
	u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/v1/models", r.URL.Path)
		require.Equal(t, "GET", r.Method)
		require.Equal(t, "model.internal:8080", r.Host)
		require.Equal(t, "Bearer upstream-fixture", r.Header.Get("Authorization"))
		require.Empty(t, r.Header.Get("Cookie"))
		_, err := io.WriteString(w, `{"object":"list","data":[{"id":"model-a"},{"id":"model-a"},{"id":"org/model-b"}]}`)
		require.NoError(t, err)
	})
	result := u.ProbeOpenAI(context.Background(), "upstream-fixture")
	require.Equal(t, "compatible", result.State)
	require.Equal(t, "openai-compatible", result.Protocol)
	require.Equal(t, []string{"model-a", "org/model-b"}, result.Models)
}

func TestProbe_ReportsUncertainResponsesWithoutVendorGuess(t *testing.T) {
	for _, tc := range []struct {
		name        string
		code        int
		body, state string
	}{
		{"unauthorized", 401, "secret upstream error", "auth_required"},
		{"forbidden", 403, "secret upstream error", "auth_required"},
		{"unavailable", 503, "private diagnostics", "unknown"},
		{"html", 200, "<html>login</html>", "unknown"},
		{"missing data", 200, `{"object":"list"}`, "unknown"},
		{"null data", 200, `{"object":"list","data":null}`, "unknown"},
		{"wrong shape", 200, `{"object":"list","data":["model"]}`, "unknown"},
		{"invalid model", 200, `{"object":"list","data":[{"id":"bad\nmodel"}]}`, "unknown"},
		{"trailing JSON", 200, `{"object":"list","data":[]} {}`, "unknown"},
		{"empty list", 200, `{"object":"list","data":[]}`, "compatible"},
		{"oversize", 200, strings.Repeat(" ", 1<<20) + `{}`, "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.code)
				// Oversized probes can close early; server write errors are expected.
				_, _ = io.WriteString(w, tc.body)
			})
			result := u.ProbeOpenAI(context.Background(), "")
			require.Equal(t, tc.state, result.State)
			if tc.state != "compatible" {
				require.Empty(t, result.Protocol)
				require.Empty(t, result.Models)
			}
		})
	}
}

func TestUpstream_RejectsRedirectAndDoesNotRetry(t *testing.T) {
	var calls atomic.Int32
	u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Location", "http://other.internal/private")
		w.WriteHeader(302)
	})
	require.Equal(t, "unknown", u.ProbeOpenAI(context.Background(), "secret").State)
	require.EqualValues(t, 1, calls.Load())
}

func TestUpstream_FailsClosedWithoutConnector(t *testing.T) {
	_, err := NewUpstream(Target{Host: "localhost", Port: 80}, nil)
	require.ErrorIs(t, err, ErrInvalidTarget)
	u, err := NewUpstream(Target{Host: "localhost", Port: 80}, func(context.Context) (net.Conn, error) {
		return nil, errors.New("offline")
	})
	require.NoError(t, err)
	t.Cleanup(u.Close)
	require.Equal(t, "unreachable", u.ProbeOpenAI(context.Background(), "").State)
}

func TestUpstream_RejectsTargetAndOperationOverrides(t *testing.T) {
	for _, target := range []Target{
		{Host: "", Port: 80}, {Host: "a/b", Port: 80}, {Host: "a@b", Port: 80},
		{Host: "a:80", Port: 80}, {Host: "a", Port: 0}, {Host: "a", Port: 65536},
		{Host: "a", Port: 80, BasePath: "//b/v1"}, {Host: "a", Port: 80, BasePath: "/v1/../admin"},
		{Host: "a", Port: 80, BasePath: "/v1%2fadmin"}, {Host: "a", Port: 80, BasePath: "/v1?key=x"},
	} {
		_, err := NewUpstream(target, func(context.Context) (net.Conn, error) { t.Fatal("must not dial"); return nil, nil })
		require.ErrorIs(t, err, ErrInvalidTarget)
	}
	u, err := NewUpstream(Target{Host: "model.internal", Port: 8080}, func(context.Context) (net.Conn, error) {
		t.Fatal("must not dial")
		return nil, nil
	})
	require.NoError(t, err)
	t.Cleanup(u.Close)
	for _, op := range []string{"../admin", "http://other/models", "models?key=x", "responses", "/models"} {
		_, err := u.Request(context.Background(), "GET", op, "", nil)
		require.Error(t, err)
	}
	_, err = u.Request(context.Background(), "GET", "models", "key\r\nX-Foo: bar", nil)
	require.Error(t, err)
}

func TestUpstream_StreamsBeforeEOFAndPropagatesCancellation(t *testing.T) {
	cancelled := make(chan struct{})
	u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		_, readErr := io.Copy(io.Discard, r.Body)
		if readErr != nil {
			t.Errorf("read request: %v", readErr)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, err := io.WriteString(w, "data: first\n\n")
		require.NoError(t, err)
		w.(http.Flusher).Flush()
		<-r.Context().Done()
		close(cancelled)
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	resp, err := u.Request(ctx, "POST", "chat/completions", "", strings.NewReader(`{"model":"a","stream":true}`))
	require.NoError(t, err)
	defer resp.Body.Close()
	buf := make([]byte, len("data: first\n\n"))
	_, err = io.ReadFull(resp.Body, buf)
	require.NoError(t, err)
	require.Equal(t, "data: first\n\n", string(buf))
	cancel()
	select {
	case <-cancelled:
	case <-time.After(time.Second):
		t.Fatal("upstream was not cancelled")
	}
}

func TestKeyAndScopes_DenyEmptyAndDoNotAliasInput(t *testing.T) {
	a, hash, err := NewKey()
	require.NoError(t, err)
	b, _, err := NewKey()
	require.NoError(t, err)
	require.NotEqual(t, a, b)
	require.Len(t, hash, 64)
	require.Equal(t, hash, KeyDigest(a))
	mappings := map[string]string{"chat": "private-a", "code": "private-b"}
	require.Empty(t, AllowedModels(nil, mappings))
	require.Empty(t, AllowedModels([]string{"*", "private-a"}, mappings))
	allowed := AllowedModels([]string{"chat", "unknown"}, mappings)
	require.Equal(t, map[string]string{"chat": "private-a"}, allowed)
	require.Equal(t, []string{"chat"}, ModelAliases(allowed))
	allowed["chat"] = "changed"
	require.Equal(t, "private-a", mappings["chat"])
}

func TestProbeAnthropicRequiresProtocolEvidence(t *testing.T) {
	for _, tc := range []struct{ body, state string }{
		{`{"object":"list","data":[{"id":"model-a"}]}`, "unknown"},
		{`{"has_more":false,"data":[{"id":"model-a"}]}`, "unknown"},
		{`{"has_more":false,"data":[{"id":"model-a","type":"model"}]}`, "compatible"},
	} {
		u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "2023-06-01", r.Header.Get("anthropic-version"))
			require.Equal(t, "fixture-key", r.Header.Get("x-api-key"))
			require.Empty(t, r.Header.Get("Authorization"))
			_, _ = io.WriteString(w, tc.body)
		})
		require.Equal(t, tc.state, u.Probe(context.Background(), "fixture-key", "anthropic").State)
	}
}

func TestUpstreamTLSDoesNotTrustUnverifiedConnectorTarget(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("untrusted TLS request must never reach handler")
	}))
	defer server.Close()
	u, err := NewUpstream(Target{Host: "model.internal", Port: 443, TLS: true}, func(ctx context.Context) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
	})
	require.NoError(t, err)
	defer u.Close()
	_, err = u.Request(context.Background(), "GET", "models", "", nil)
	require.Error(t, err)
}
