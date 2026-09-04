package iam

import (
	"net/http"
	"testing"
)

func TestResourcePermissionForRequest(t *testing.T) {
	tests := []struct {
		name, method, path, resource, action string
		ok                                   bool
	}{
		{"list connectors", http.MethodGet, "/api/v1/edges", "connectors", "read", true},
		{"scan connector", http.MethodPost, "/api/v1/edges/1/scan_application_tasks", "connectors", "create", true},
		{"create application", http.MethodPost, "/api/v1/applications", "applications", "create", true},
		{"update firewall", http.MethodPut, "/api/v1/proxies/1/firewall", "accesses", "update", true},
		{"open web ssh", http.MethodPost, "/api/v1/webssh/proxies/1/session", "accesses", "use", true},
		{"query web data", http.MethodPost, "/api/v1/webdata/sessions/token/execute", "accesses", "use", true},
		{"read audits", http.MethodGet, "/api/v1/audits/access", "logs", "read", true},
		{"unrelated iam", http.MethodGet, "/api/v1/iam/account", "", "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(tc.method, tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			resource, action, ok := resourcePermissionForRequest(req)
			if ok != tc.ok || resource != tc.resource || action != tc.action {
				t.Fatalf("got (%q, %q, %v), want (%q, %q, %v)", resource, action, ok, tc.resource, tc.action, tc.ok)
			}
		})
	}
}
