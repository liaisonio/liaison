package web

import (
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/proto"
)

func TestAgentRequestDecodeExactIDs(t *testing.T) {
	for _, id := range []string{"9", "9007199254740993", "18446744073709551615"} {
		for _, quoted := range []bool{false, true} {
			raw := id
			if quoted {
				raw = `"` + id + `"`
			}
			body := `{"edge_id":` + raw + `}`
			var request proto.EdgeAgentRequest
			var application controlplane.AgentApplicationInput
			var access controlplane.AgentAccessInput
			for _, dst := range []any{&request, &application, &access} {
				if err := decodeAgentRequest(strings.NewReader(body), dst, true); err != nil {
					t.Fatalf("%s: %v", body, err)
				}
			}
			if request.EdgeID != application.EdgeID || request.EdgeID != access.EdgeID {
				t.Fatal("ID mismatch")
			}
			if id == "9007199254740993" && request.EdgeID != 9007199254740993 {
				t.Fatal("precision loss")
			}
			if id == "18446744073709551615" && request.EdgeID != ^uint64(0) {
				t.Fatal("uint64 truncated")
			}
		}
	}
}

func TestAgentRequestDecodeOptionalAndInvalid(t *testing.T) {
	for _, body := range []string{`{}`, `{"edge_id":0}`, `{"edge_id":"0"}`, `{"edge_id":null}`, `{"edge_id":"01"}`, `{"edge_id":1.5}`, `{"edge_id":18446744073709551616}`, `{"edge_id":"18446744073709551616"}`} {
		var input proto.EdgeAgentRequest
		if err := decodeAgentRequest(strings.NewReader(body), &input, true); err == nil {
			t.Fatalf("accepted invalid required connector: %s", body)
		}
	}
	for _, body := range []string{`{}`, `{"name":"updated"}`} {
		var input controlplane.AgentApplicationInput
		if err := decodeAgentRequest(strings.NewReader(body), &input, false); err != nil {
			t.Fatal(err)
		}
	}
	for _, body := range []string{`null`, `[]`, `{} {}`, `{"edge_id":true}`, `{"edge_id":-1}`, `{"edge_id":"+1"}`, `{"edge_id":" 1"}`, `{"edge_id":""}`, `{"edge_id":"1.0"}`, `{"edge_id":"1e2"}`, `{"edge_id":"9","actor_id":"17"}`, `{"edge_id":"9","owner_id":17}`} {
		var input controlplane.AgentApplicationInput
		if err := decodeAgentRequest(strings.NewReader(body), &input, false); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}
