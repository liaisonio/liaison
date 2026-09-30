package claude

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

func applyUpdate(t *testing.T, u *Updates, line string) []agentruntime.Update {
	t.Helper()
	var event Event
	if err := json.Unmarshal([]byte(line), &event); err != nil {
		t.Fatal(err)
	}
	event.Raw = json.RawMessage(line)
	updates, err := u.Apply(event)
	if err != nil {
		t.Fatal(err)
	}
	return updates
}

func TestStreamTextNotRepeated(t *testing.T) {
	u := NewUpdates()
	applyUpdate(t, u, `{"type":"stream_event","event":{"type":"message_start","message":{"id":"m1"}}}`)
	updates := applyUpdate(t, u, `{"type":"stream_event","event":{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"hello"}}}`)
	if len(updates) != 1 || updates[0].Kind != agentruntime.MessageDelta || updates[0].Text != "hello" {
		t.Fatal("missing text delta")
	}
	if got := applyUpdate(t, u, `{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"hello"}]}}`); len(got) != 0 {
		t.Fatal("complete message repeated stream")
	}
	got := applyUpdate(t, u, `{"type":"result","subtype":"success","result":"hello"}`)
	if len(got) != 1 || got[0].Kind != agentruntime.TurnEnded {
		t.Fatal("result repeated message")
	}
}

func TestToolWaitsForResult(t *testing.T) {
	u := NewUpdates()
	got := applyUpdate(t, u, `{"type":"assistant","message":{"id":"m1","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{"command":"pwd"}}]}}`)
	if len(got) != 1 || got[0].Status != "running" || got[0].Command != "pwd" {
		t.Fatal("missing tool start")
	}
	if got := applyUpdate(t, u, `{"type":"stream_event","event":{"type":"content_block_stop","index":0}}`); len(got) != 0 {
		t.Fatal("tool finished before executing")
	}
	got = applyUpdate(t, u, `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool1","content":"/project"}]}}`)
	if len(got) != 1 || got[0].Status != "completed" || got[0].Text != "/project" {
		t.Fatal("missing tool completion")
	}
}

func TestNativeErrorsAndConfigNotExposed(t *testing.T) {
	u := NewUpdates()
	if got := applyUpdate(t, u, `{"type":"system","subtype":"init","apiKey":"secret"}`); len(got) != 0 {
		t.Fatal("native config exposed")
	}
	applyUpdate(t, u, `{"type":"assistant","message":{"id":"m1","content":[{"type":"tool_use","id":"tool1","name":"Bash","input":{}}]}}`)
	got := applyUpdate(t, u, `{"type":"user","message":{"content":[{"type":"tool_result","tool_use_id":"tool1","is_error":true,"content":"raw-secret-error"}]}}`)
	if len(got) != 1 || got[0].Text != "" || got[0].Status != "failed" {
		t.Fatal("raw tool error exposed")
	}
	got = applyUpdate(t, u, `{"type":"result","is_error":true,"result":"raw-provider-error"}`)
	if len(got) != 1 || got[0].Text != "" || got[0].Status != "failed" {
		t.Fatal("provider error exposed")
	}
}

func TestNonStreamingBlocksShareMessageID(t *testing.T) {
	u := NewUpdates()
	for _, s := range []string{"first", "second"} {
		line := `{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":"` + s + `"}]}}`
		got := applyUpdate(t, u, line)
		if len(got) != 1 || got[0].Text != s {
			t.Fatal("same-message content block dropped")
		}
		if got := applyUpdate(t, u, line); len(got) != 0 {
			t.Fatal("identical message replayed")
		}
	}
}

func TestDisplayTextBoundedWithoutBreakingUTF8(t *testing.T) {
	u := NewUpdates()
	text := strings.Repeat("文", maxTurnText)
	raw, err := json.Marshal(text)
	if err != nil {
		t.Fatal(err)
	}
	got := applyUpdate(t, u, `{"type":"assistant","message":{"id":"m1","content":[{"type":"text","text":`+string(raw)+`}]}}`)
	if len(got) != 1 || !got[0].Truncated || len(got[0].Text) > maxTurnText || !utf8.ValidString(got[0].Text) {
		t.Fatal("invalid text bound")
	}
	got = applyUpdate(t, u, `{"type":"result","subtype":"success"}`)
	if len(got) != 1 || got[0].Status != "completed" {
		t.Fatal("display limit prevented completion")
	}
}
