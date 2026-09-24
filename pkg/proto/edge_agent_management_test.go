package proto

import (
	"strings"
	"testing"
)

func TestAgentManagementValidation(t *testing.T) {
	for _, tc := range []struct {
		action, model, title string
		valid                bool
	}{
		{"model", "native-model", "", true}, {"model", "", "", false}, {"model", strings.Repeat("m", 129), "", false},
		{"send", "model", "", false}, {"models", "", "", true}, {"rename", "", "hello", true},
		{"rename", "", "\n", false}, {"rename", "", strings.Repeat("中", 121), false}, {"rename", "", "\x00", false}, {"delete", "", "", true}, {"delete", "", "title", false},
	} {
		r := EdgeAgentRequest{Action: tc.action, Model: tc.model, Title: tc.title, SessionID: strings.Repeat("a", 32)}
		if r.Valid() != tc.valid {
			t.Errorf("%s model=%q title=%q validity mismatch", tc.action, tc.model, tc.title)
		}
	}
}
