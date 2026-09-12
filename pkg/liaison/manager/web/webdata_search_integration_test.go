package web

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/stretchr/testify/require"
)

func TestSearchConnection_ScopedTLSAndRedirects(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://other.invalid/", http.StatusFound)
	}))
	defer server.Close()
	parsed, err := url.Parse(server.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(parsed.Port())
	require.NoError(t, err)
	s := &webDataSession{target: &controlplane.WebDataTarget{TargetHost: parsed.Hostname(), TargetPort: port}, tlsMode: "require"}
	dial := (&net.Dialer{}).DialContext
	require.Error(t, s.openSearch(context.Background(), "", dial), "untrusted TLS certificate must fail")
	s.tlsMode = "skip-verify"
	require.ErrorContains(t, s.openSearch(context.Background(), "", dial), "redirects are disabled")
	require.Nil(t, s.searchClient)
	s.connectionParams = "host=other"
	require.Error(t, s.openSearch(context.Background(), "", dial))
}

// Opt-in: run against dedicated local test instances, never production.
func TestSearchIntegration_Workspace(t *testing.T) {
	for _, protocol := range []string{"elasticsearch", "opensearch"} {
		t.Run(protocol, func(t *testing.T) {
			prefix := "TEST_" + strings.ToUpper(protocol)
			address := os.Getenv(prefix + "_ADDRESS")
			if address == "" {
				t.Skip("set " + prefix + "_ADDRESS for a dedicated instance")
			}
			host, portText, err := net.SplitHostPort(address)
			require.NoError(t, err)
			port, err := strconv.Atoi(portText)
			require.NoError(t, err)
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			s := &webDataSession{protocol: protocol, target: &controlplane.WebDataTarget{TargetHost: host, TargetPort: port}, username: os.Getenv(prefix + "_USERNAME"), tlsMode: os.Getenv(prefix + "_TLS_MODE")}
			require.NoError(t, s.openSearch(ctx, os.Getenv(prefix+"_PASSWORD"), (&net.Dialer{}).DialContext))
			defer s.close()
			index := "liaison-test-" + strconv.FormatInt(time.Now().UnixNano(), 10)
			run := func(method, path, body string) *webDataExecuteResponse {
				t.Helper()
				cmd := searchCommand{Method: method, Path: path, Body: json.RawMessage(body)}
				raw, e := json.Marshal(cmd)
				require.NoError(t, e)
				result, e := s.execute(ctx, string(raw))
				require.NoError(t, e)
				return result
			}
			run("PUT", "/"+index, `{"mappings":{"properties":{"message":{"type":"text"},"value":{"type":"integer"}}}}`)
			defer func() {
				cleanup, c := context.WithTimeout(context.Background(), 10*time.Second)
				defer c()
				_, e := s.searchRequest(cleanup, "DELETE", "/"+index, nil)
				require.NoError(t, e)
			}()
			run("PUT", "/"+index+"/_doc/1", `{"message":"中文 test","value":3}`)
			got := run("GET", "/"+index+"/_doc/1", "")
			require.Contains(t, got.Message, "中文")
			run("POST", "/"+index+"/_update/1", `{"doc":{"value":4}}`)
			got = run("GET", "/"+index+"/_doc/1", "")
			require.Contains(t, got.Message, `"value": 4`)
			nodes, e := s.metadata(ctx)
			require.NoError(t, e)
			require.NotEmpty(t, nodes)
			detail, e := s.objectDetails(ctx, webDataObjectRequest{ObjectType: "index", Name: index})
			require.NoError(t, e)
			require.Len(t, detail.Columns, 2)
			// Refresh is test setup only, not an exposed workspace endpoint.
			_, e = s.searchRequest(ctx, "POST", "/"+index+"/_refresh", nil)
			require.NoError(t, e)
			got = run("POST", "/"+index+"/_search", `{"size":10,"query":{"match_all":{}},"aggs":{"value_sum":{"sum":{"field":"value"}}}}`)
			require.Len(t, got.Rows, 1)
			require.Contains(t, got.Message, "value_sum")
			run("DELETE", "/"+index+"/_doc/1", "")
		})
	}
}
