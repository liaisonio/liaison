package web

import (
	"context"
	"encoding/json"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSearchCommand_RejectsUnsafeTargets(t *testing.T) {
	for _, path := range []string{"https://other.test/index", "//other.test/index", "/../_search", "/a%2f..%2f_search", "/a/_search?scroll=1m", "/_reindex", "/_snapshot/repo", "/_security/user", "/a/_search/extra", "/a\\b/_search", "/*/_search", "/a/../_search"} {
		t.Run(path, func(t *testing.T) {
			raw, _ := json.Marshal(searchCommand{Method: "POST", Path: path})
			_, err := parseSearchCommand(string(raw))
			require.Error(t, err)
		})
	}
	for _, raw := range []string{`{"method":"GET","path":"/logs/_search","headers":{}}`, `{"method":"GET","path":"/logs/_search"} {}`, `{"method":"POST","path":"/logs/_search","body":null}`, `{"method":"POST","path":"/logs/_search","body":[]}`} {
		_, err := parseSearchCommand(raw)
		require.Error(t, err)
	}
	for _, method := range []string{"GET", "POST"} {
		_, err := parseSearchCommand(`{"method":"` + method + `","path":"/logs/_search","body":{"size":20}}`)
		require.NoError(t, err)
	}
	require.True(t, searchIsQuery(`{"method":"POST","path":"/logs/_search"}`))
	require.False(t, searchIsQuery(`{"method":"PUT","path":"/logs/_doc/1","body":{"a":1}}`))
}

func TestSearchWorkspace_HTTPResponseAndMetadata(t *testing.T) {
	for _, protocol := range []string{"elasticsearch", "opensearch"} {
		t.Run(protocol, func(t *testing.T) {
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				u, p, ok := r.BasicAuth()
				require.True(t, ok)
				require.Equal(t, "reader", u)
				require.Equal(t, "test-only", p)
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/_alias":
					_, _ = w.Write([]byte(`{"logs": {"aliases": {}}}`))
				case "/logs/_mapping":
					_, _ = w.Write([]byte(`{"logs":{"mappings":{"properties":{"message":{"type":"text"}}}}}`))
				case "/logs/_search":
					var body map[string]any
					require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
					require.Equal(t, float64(100), body["size"])
					_, _ = w.Write([]byte(`{"hits":{"total":{"value":1},"hits":[{"_index":"logs","_id":"1","_score":1,"_source":{"message":"中文","value":null}}]},"aggregations":{"count":{"value":1}}}`))
				default:
					w.WriteHeader(400)
					_, _ = w.Write([]byte(`{"error":{"type":"parse_exception","reason":"invalid query"}}`))
				}
			}))
			defer server.Close()
			s := &webDataSession{protocol: protocol, searchClient: server.Client(), searchURL: server.URL, username: "reader", searchPassword: "test-only"}
			nodes, err := s.metadata(context.Background())
			require.NoError(t, err)
			require.Len(t, nodes, 1)
			require.Equal(t, "index", nodes[0].Type)
			detail, err := s.objectDetails(context.Background(), webDataObjectRequest{ObjectType: "index", Name: "logs"})
			require.NoError(t, err)
			require.Contains(t, detail.DDL, "message")
			result, err := s.execute(context.Background(), `{"method":"POST","path":"/logs/_search","body":{"query":{"match_all":{}}}}`)
			require.NoError(t, err)
			require.Len(t, result.Rows, 1)
			require.Contains(t, result.Message, "aggregations")
			require.Contains(t, result.Message, "中文")
			before := calls
			_, err = s.execute(context.Background(), `{"method":"POST","path":"/logs/_search","body":{"size":501}}`)
			require.Error(t, err)
			require.Equal(t, before, calls)
			_, err = s.execute(context.Background(), `{"method":"GET","path":"/missing/_mapping"}`)
			require.ErrorContains(t, err, "HTTP 400")
			s.close()
			require.Empty(t, s.searchPassword)
			_, err = s.execute(context.Background(), `{"method":"GET","path":"/logs/_mapping"}`)
			require.Error(t, err)
		})
	}
}

func TestSearchResponse_LimitsAndCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(strings.Repeat("x", (4<<20)+1))) }))
	defer server.Close()
	s := &webDataSession{searchClient: server.Client(), searchURL: server.URL}
	_, err := s.searchRequest(context.Background(), "GET", "/", nil)
	require.ErrorContains(t, err, "4 MiB")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = s.searchRequest(ctx, "GET", "/", nil)
	require.ErrorIs(t, err, context.Canceled)
}
