package web

import "testing"

func TestClassifyManagementOperation(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantModule string
		wantAction string
		wantOK     bool
	}{
		{name: "create connector", method: "POST", path: "/api/v1/edges", wantModule: "connector", wantAction: "create", wantOK: true},
		{name: "update application", method: "PUT", path: "/api/v1/applications/12", wantModule: "application", wantAction: "update", wantOK: true},
		{name: "delete access", method: "DELETE", path: "/api/v1/proxies/9", wantModule: "access", wantAction: "delete", wantOK: true},
		{name: "change password", method: "POST", path: "/api/v1/iam/password", wantModule: "account", wantAction: "change_password", wantOK: true},
		{name: "logout", method: "POST", path: "/api/v1/iam/logout", wantModule: "account", wantAction: "logout", wantOK: true},
		{name: "ignore access execution", method: "POST", path: "/api/v1/webdata/sessions/token/execute", wantOK: false},
		{name: "ignore read", method: "GET", path: "/api/v1/applications", wantOK: false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			module, action, _, ok := classifyManagementOperation(test.method, test.path)
			if ok != test.wantOK || module != test.wantModule || action != test.wantAction {
				t.Fatalf("classifyManagementOperation() = (%q, %q, %v), want (%q, %q, %v)", module, action, ok, test.wantModule, test.wantAction, test.wantOK)
			}
		})
	}
}
