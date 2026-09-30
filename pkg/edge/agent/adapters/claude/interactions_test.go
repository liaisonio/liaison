package claude

import (
	"encoding/json"
	"testing"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestSharedCommandDecisionIsOneShot(t *testing.T) {
	for _, allow := range []bool{true, false} {
		f := newFixture(t)
		f.emit(t, `{"type":"control_request","request_id":"r","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"printf test","description":"Test"}}}`)
		s := &Session{ctx: t.Context(), driver: &Driver{Client: f.client}, thread: "thread", turn: "turn", updates: make(chan agentruntime.Update, 8)}
		require.True(t, s.handleInteraction(nextEvent(t, f.client)))
		u := <-s.updates
		require.Equal(t, "printf test", u.Interaction.Command)
		require.Equal(t, "turn", u.TurnID)
		require.Error(t, s.Decide(t.Context(), "wrong", "r", allow))
		require.NoError(t, s.Decide(t.Context(), "turn", "r", allow))
		var envelope struct {
			Response struct {
				Behavior string `json:"behavior"`
			} `json:"response"`
		}
		require.NoError(t, json.Unmarshal(f.request(t)["response"], &envelope))
		want := "deny"
		if allow {
			want = "allow"
		}
		require.Equal(t, want, envelope.Response.Behavior)
		require.Error(t, s.Decide(t.Context(), "turn", "r", allow))
	}
}

func TestUnrenderableInteractionDenied(t *testing.T) {
	for name, payload := range map[string]string{
		"file":          `"tool_name":"Write","input":{"file_path":"x","content":"secret"}`,
		"background":    `"tool_name":"Bash","input":{"command":"printf test","run_in_background":true}`,
		"unknown field": `"tool_name":"Bash","input":{"command":"printf test","extension":true}`,
		"multiselect":   `"tool_name":"AskUserQuestion","input":` + questionInput,
	} {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			f.emit(t, `{"type":"control_request","request_id":"r","request":{"subtype":"can_use_tool",`+payload+`}}`)
			s := &Session{ctx: t.Context(), driver: &Driver{Client: f.client}, thread: "thread", turn: "turn", updates: make(chan agentruntime.Update, 8)}
			require.True(t, s.handleInteraction(nextEvent(t, f.client)))
			require.Empty(t, s.updates)
			require.Contains(t, string(f.request(t)["response"]), `"behavior":"deny"`)
		})
	}
}

func TestCancelledSharedInteractionCannotBeApproved(t *testing.T) {
	f := newFixture(t)
	f.emit(t, `{"type":"control_request","request_id":"r","request":{"subtype":"can_use_tool","tool_name":"Bash","input":{"command":"printf test"}}}`)
	s := &Session{ctx: t.Context(), driver: &Driver{Client: f.client}, thread: "thread", turn: "turn", updates: make(chan agentruntime.Update, 8)}
	require.True(t, s.handleInteraction(nextEvent(t, f.client)))
	<-s.updates
	f.emit(t, `{"type":"control_cancel_request","request_id":"r"}`)
	require.True(t, s.handleInteraction(nextEvent(t, f.client)))
	require.Equal(t, agentruntime.InteractionResolved, (<-s.updates).Kind)
	require.Error(t, s.Decide(t.Context(), "turn", "r", true))
}
