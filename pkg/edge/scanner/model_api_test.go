package scanner

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
)

func probeTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, string, int) {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	host, portString, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatal(err)
	}
	return server, host, port
}

func TestModelDiscovery_ReadOnlyAndProtocols(t *testing.T) {
	tests := []struct {
		name, tags, list string
		protocols        int
		service          string
		models           int
	}{
		{"ollama", `{"models":[{"name":"local:test","model":"local:test"}]}`, `{}`, 1, "ollama", 1},
		{"openai custom port", `{}`, `{"object":"list","data":[{"id":"local:test","object":"model"}]}`, 1, "", 1},
		{"dual protocol dedup", `{"models":[{"name":"local:test","digest":"abc"}]}`, `{"object":"list","data":[{"id":"local:test","object":"model"}]}`, 2, "ollama", 1},
		{"generic models object", `{"models":[{"name":"ordinary"}]}`, `{"data":[{"id":"ordinary"}]}`, 0, "", 0},
		{"invalid ID", `{}`, `{"object":"list","data":[{"id":"unsafe\nname","object":"model"}]}`, 0, "", 0},
		{"empty openai", `{}`, `{"object":"list","data":[]}`, 1, "", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, host, port := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != "" {
					t.Errorf("unsafe request: %s", r.Method)
				}
				w.Header().Set("Content-Type", "application/json")
				switch r.URL.Path {
				case "/api/tags":
					fmt.Fprint(w, tt.tags)
				case "/v1/models":
					fmt.Fprint(w, tt.list)
				default:
					t.Errorf("unexpected metadata request %s", r.URL.Path)
				}
			})
			got := discoverModelAPI(context.Background(), host, port)
			if tt.protocols == 0 {
				if got != nil {
					t.Fatalf("false positive: %+v", got)
				}
				return
			}
			if got == nil || got.State != "verified" || len(got.Protocols) != tt.protocols || got.Service != tt.service || len(got.Models) != tt.models {
				t.Fatalf("unexpected result: %+v", got)
			}
		})
	}
}

func TestModelDiscovery_RejectsRedirectAndGenericAuth(t *testing.T) {
	var reached atomic.Int32
	external, _, _ := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) { reached.Add(1) })
	for _, status := range []int{302, 401, 403} {
		t.Run(strconv.Itoa(status), func(t *testing.T) {
			_, host, port := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", external.URL)
				w.WriteHeader(status)
			})
			if got := discoverModelAPI(context.Background(), host, port); got != nil {
				t.Fatalf("generic response misidentified: %+v", got)
			}
		})
	}
	if reached.Load() != 0 {
		t.Fatal("followed redirect outside selected target")
	}
}

func TestModelDiscovery_BoundsAndEmptyOllama(t *testing.T) {
	for _, name := range []string{"large", "html", "empty ollama", "empty generic", "bounded models"} {
		t.Run(name, func(t *testing.T) {
			_, host, port := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				switch name {
				case "large":
					fmt.Fprint(w, strings.Repeat(" ", maxModelResponse)+`{"object":"list","data":[]}`)
				case "html":
					w.Header().Set("Content-Type", "text/html")
					fmt.Fprint(w, `{"object":"list","data":[]}`)
				case "empty ollama", "empty generic":
					if r.URL.Path == "/api/tags" {
						fmt.Fprint(w, `{"models":[]}`)
					} else if r.URL.Path == "/api/version" && name == "empty ollama" {
						fmt.Fprint(w, `{"version":"0.9.0"}`)
					} else {
						fmt.Fprint(w, `{}`)
					}
				case "bounded models":
					if r.URL.Path != "/v1/models" {
						fmt.Fprint(w, `{}`)
						return
					}
					fmt.Fprint(w, `{"object":"list","data":[`)
					for i := 0; i < 110; i++ {
						if i > 0 {
							fmt.Fprint(w, ",")
						}
						fmt.Fprintf(w, `{"id":"model-%d","object":"model"}`, i)
					}
					fmt.Fprint(w, ` ]}`)
				}
			})
			got := discoverModelAPI(context.Background(), host, port)
			switch name {
			case "empty ollama":
				if got == nil || got.Service != "ollama" || len(got.Models) != 0 {
					t.Fatalf("empty Ollama missing: %+v", got)
				}
			case "bounded models":
				if got == nil || len(got.Models) != 100 || !got.ModelsTruncated {
					t.Fatalf("unbounded models: %+v", got)
				}
			default:
				if got != nil {
					t.Fatalf("false positive: %+v", got)
				}
			}
		})
	}
}

func TestModelDiscovery_Cancellation(t *testing.T) {
	_, host, port := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	start := time.Now()
	if got := discoverModelAPI(ctx, host, port); got != nil {
		t.Fatal(got)
	}
	if time.Since(start) > time.Second {
		t.Fatal("cancellation ignored")
	}
}

func TestModelDiscovery_AuthCandidate(t *testing.T) {
	for _, tc := range []struct {
		port, tags, models int
		want               bool
	}{
		{11434, 401, 404, true}, {8000, 404, 403, true},
		{1234, 401, 401, true}, {30000, 404, 401, true},
		{8080, 401, 401, false}, {11434, 404, 404, false},
		{8000, 200, 200, false}, {8000, 0, 0, false},
	} {
		if got := modelAuthCandidate(tc.port, tc.tags, tc.models); got != tc.want {
			t.Errorf("candidate(%d,%d,%d)=%v want %v", tc.port, tc.tags, tc.models, got, tc.want)
		}
	}
}

func TestDiscovery_DefaultPortsAndCustomScan(t *testing.T) {
	s := &scanner{}
	ports, err := s.getPorts(0)
	if err != nil {
		t.Fatal(err)
	}
	seen := map[int]bool{}
	for _, p := range ports {
		if seen[p] {
			t.Fatalf("duplicate port %d", p)
		}
		seen[p] = true
	}
	for _, p := range []int{11434, 8000, 1234, 30000} {
		if !seen[p] {
			t.Fatalf("missing port %d", p)
		}
	}
	_, host, port := probeTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/v1/models" {
			fmt.Fprint(w, `{"object":"list","data":[{"id":"local","object":"model"}]}`)
		} else {
			fmt.Fprint(w, `{}`)
		}
	})
	result := &proto.ScanApplicationTaskResult{}
	if err := s.scanWithGo(context.Background(), &proto.ScanApplicationTaskRequest{Nets: []string{host}, Port: port, Protocol: "tcp"}, result); err != nil {
		t.Fatal(err)
	}
	if len(result.ScannedApplications) != 1 || result.ScannedApplications[0].ModelAPI == nil {
		t.Fatalf("metadata lost: %+v", result)
	}
}
