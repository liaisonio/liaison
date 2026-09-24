package proto

import (
	"strings"
	"testing"
)

func TestEdgeAgentDirectoryAndWatchValidation(t *testing.T) {
	id := strings.Repeat("a", 32)
	for _, tc := range []struct {
		name  string
		req   EdgeAgentRequest
		valid bool
	}{
		{"home directory", EdgeAgentRequest{Action: "directories"}, true},
		{"sessions scoped to access", EdgeAgentRequest{Action: "sessions", AccessID: id}, true},
		{"sessions requires access", EdgeAgentRequest{Action: "sessions"}, false},
		{"sessions rejects cursor", EdgeAgentRequest{Action: "sessions", AccessID: id, Cursor: "1"}, false},
		{"specific directory", EdgeAgentRequest{Action: "directories", Directory: "/home/user/project", AccessID: id}, true},
		{"client project is not a browse root", EdgeAgentRequest{Action: "directories", Project: "/etc"}, false},
		{"oversized directory", EdgeAgentRequest{Action: "directories", Directory: strings.Repeat("a", 4097)}, false},
		{"watch", EdgeAgentRequest{Action: "watch", SessionID: id, Cursor: "1"}, true},
		{"watch requires cursor", EdgeAgentRequest{Action: "watch", SessionID: id}, false},
		{"negative cursor", EdgeAgentRequest{Action: "watch", SessionID: id, Cursor: "-1"}, false},
		{"cursor overflow", EdgeAgentRequest{Action: "watch", SessionID: id, Cursor: "18446744073709551616"}, false},
		{"poll compatibility", EdgeAgentRequest{Action: "poll", SessionID: id}, true},
		{"resume scoped to access", EdgeAgentRequest{Action: "resume", SessionID: id, AccessID: id}, true},
		{"resume requires access", EdgeAgentRequest{Action: "resume", SessionID: id}, false},
		{"directory only on browse", EdgeAgentRequest{Action: "poll", SessionID: id, Directory: "/home/user"}, false},
		{"session directory", EdgeAgentRequest{Action: "start", InstallationID: id, Project: "/saved", WorkingDirectory: "/home/user/project"}, true},
		{"cannot change running directory", EdgeAgentRequest{Action: "send", SessionID: id, Text: "hello", WorkingDirectory: "/home/user"}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.req.Valid(); got != tc.valid {
				t.Fatalf("Valid()=%v, want %v", got, tc.valid)
			}
		})
	}
}
