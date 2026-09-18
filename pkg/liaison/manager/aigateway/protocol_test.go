package aigateway

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNativeTransport(t *testing.T) {
	for _, tc := range []struct {
		protocol, operation, header, value, query string
		stream                                    bool
	}{
		{"openai", "responses", "Authorization", "Bearer fixture", "", true},
		{"ark", "chat/completions", "Authorization", "Bearer fixture", "", false},
		{"ark", "responses", "Authorization", "Bearer fixture", "", true},
		{"qwen", "services/aigc/text-generation/generation", "Authorization", "Bearer fixture", "", true},
		{"gemini", "models/gemini-fixture:generateContent", "x-goog-api-key", "fixture", "", false},
		{"gemini", "models/gemini-fixture:streamGenerateContent", "x-goog-api-key", "fixture", "alt=sse", true},
	} {
		t.Run(tc.protocol+"/"+tc.operation, func(t *testing.T) {
			u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				require.Equal(t, "/v1/"+tc.operation, r.URL.Path)
				require.Equal(t, tc.query, r.URL.RawQuery)
				require.Equal(t, tc.value, r.Header.Get(tc.header))
				require.Empty(t, r.Header.Get("Cookie"))
				require.Empty(t, r.Header.Get("x-api-key"))
				if tc.protocol == "gemini" {
					require.Empty(t, r.Header.Get("Authorization"))
				}
				if tc.protocol == "qwen" {
					require.Equal(t, "enable", r.Header.Get("X-DashScope-SSE"))
				} else {
					require.Empty(t, r.Header.Get("X-DashScope-SSE"))
				}
				_, _ = io.WriteString(w, `{}`)
			})
			resp, err := u.RequestProtocolStream(context.Background(), "POST", tc.operation, "fixture", tc.protocol, strings.NewReader(`{}`), tc.stream)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
		})
	}
}

func TestNativeOperationsDenyUnownedResourcesAndPaths(t *testing.T) {
	for _, tc := range []struct{ protocol, method, operation string }{
		{"openai", "GET", "responses/other-user"},
		{"openai", "POST", "files"},
		{"ark", "GET", "models"},
		{"qwen", "GET", "models"},
		{"ollama", "POST", "pull"},
		{"ollama", "DELETE", "delete"},
		{"gemini", "POST", "models/../admin:generateContent"},
		{"gemini", "POST", "models/x%2fy:generateContent"},
		{"gemini", "POST", "models/x:generateContent?key=secret"},
		{"gemini", "POST", "models/x:streamGenerateContent?alt=sse"},
		{"gemini", "POST", "models/x:delete"},
		{"gemini", "GET", "http://other/models"},
		{"unknown", "GET", "models"},
	} {
		require.False(t, allowedOperation(tc.protocol, tc.method, tc.operation), "%+v", tc)
	}
}

func TestGeminiProbeDoesNotMisrepresentPartialCatalog(t *testing.T) {
	for _, tc := range []struct{ body, state string }{
		{`{"models":[{"name":"models/gemini-fixture"}]}`, "compatible"},
		{`{"models":[]}`, "compatible"},
		{`{"models":[],"nextPageToken":"next"}`, "unknown"},
		{`{"models":[{"name":"models/../private"}]}`, "unknown"},
		{`{"models":[{"name":"gemini-fixture"}]}`, "unknown"},
		{`{"models":null}`, "unknown"},
	} {
		u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
			require.Equal(t, "fixture", r.Header.Get("x-goog-api-key"))
			_, _ = io.WriteString(w, tc.body)
		})
		require.Equal(t, tc.state, u.Probe(context.Background(), "fixture", "gemini").State)
	}
}

func TestProtocolCatalogIsExplicit(t *testing.T) {
	for id, path := range map[string]string{"openai": "/v1", "openai-compatible": "/v1", "anthropic": "/v1", "ark": "/api/v3", "qwen": "/api/v1", "gemini": "/v1beta", "ollama": "/api"} {
		p, ok := LookupProtocol(id)
		require.True(t, ok)
		require.Equal(t, path, p.BasePath)
	}
	_, ok := LookupProtocol("qwen3-on-vllm")
	require.False(t, ok)
	for _, id := range []string{"ark", "qwen", "unknown"} {
		u := testUpstream(t, func(http.ResponseWriter, *http.Request) { t.Error("unsupported catalog must not dial") })
		require.Equal(t, "unsupported", u.Probe(context.Background(), "", id).State)
	}
}

func TestGeminiProbePagination(t *testing.T) {
	calls := 0
	u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		require.Equal(t, "fixture", r.Header.Get("x-goog-api-key"))
		if calls == 1 {
			_, _ = io.WriteString(w, `{"models":[{"name":"models/first","supportedGenerationMethods":["generateContent"]}],"nextPageToken":"a+/= &"}`)
			return
		}
		require.Equal(t, "a+/= &", r.URL.Query().Get("pageToken"))
		_, _ = io.WriteString(w, `{"models":[{"name":"models/second"},{"name":"models/embed","supportedGenerationMethods":["embedContent"]}]}`)
	})
	result := u.Probe(context.Background(), "fixture", "gemini")
	require.Equal(t, "compatible", result.State)
	require.Equal(t, []string{"first", "second"}, result.Models)
	require.Equal(t, 2, calls)
}
