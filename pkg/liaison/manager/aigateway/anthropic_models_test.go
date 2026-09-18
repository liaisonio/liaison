package aigateway

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAnthropicModelsPagination(t *testing.T) {
	for _, mode := range []string{"success", "auth", "failure", "repeat", "missing cursor", "page limit", "model limit"} {
		t.Run(mode, func(t *testing.T) {
			calls := 0
			u := testUpstream(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.URL.Path != "/v1/models" || r.Header.Get("x-api-key") != "fixture" || r.Header.Get("anthropic-version") != "2023-06-01" {
					t.Error("unexpected path or authentication")
				}
				if calls == 2 && mode == "auth" {
					w.WriteHeader(401)
					return
				}
				if calls == 2 && mode == "failure" {
					w.WriteHeader(503)
					return
				}
				id := "model-a"
				if mode == "page limit" {
					id = fmt.Sprintf("model-%d", calls)
				}
				items := []map[string]string{{"id": id, "type": "model"}}
				more := true
				if mode == "success" && calls == 2 {
					if r.URL.Query().Get("after_id") != "model-a" || r.URL.Query().Get("pageToken") != "" {
						t.Error("invalid pagination query")
					}
					items = append(items, map[string]string{"id": "model-b", "type": "model"})
					more = false
				}
				if mode == "missing cursor" {
					id = ""
				}
				if mode == "model limit" {
					items = nil
					for i := 0; i < 1001; i++ {
						items = append(items, map[string]string{"id": fmt.Sprintf("model-%d", i), "type": "model"})
					}
					more = false
				}
				if err := json.NewEncoder(w).Encode(map[string]any{"data": items, "has_more": more, "last_id": id}); err != nil {
					t.Error(err)
				}
			})
			result := u.Probe(context.Background(), "fixture", "anthropic")
			if mode == "success" {
				require.Equal(t, "compatible", result.State)
				require.Equal(t, []string{"model-a", "model-b"}, result.Models)
				require.Equal(t, 2, calls)
			} else {
				state := "unknown"
				if mode == "auth" {
					state = "auth_required"
				}
				require.Equal(t, state, result.State)
				require.Empty(t, result.Models)
				require.Empty(t, result.Protocol)
				if mode == "page limit" {
					require.Equal(t, 10, calls)
				}
				if mode == "repeat" {
					require.Equal(t, 2, calls)
				}
			}
		})
	}
}
