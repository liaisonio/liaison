package claude

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/stretchr/testify/require"
)

func TestSessionNormalizedTurn(t *testing.T) {
	s := &Session{thread: "owned", turn: "turn", normalizer: NewUpdates(), updates: make(chan agentruntime.Update, 8)}
	require.True(t, s.consume(Event{Type: "assistant", Raw: json.RawMessage(`{"type":"assistant","message":{"id":"m","content":[{"type":"text","text":"hello"}]}}`)}))
	u := <-s.updates
	require.Equal(t, "owned", u.ThreadID)
	require.Equal(t, "turn", u.TurnID)
	require.Equal(t, "hello", u.Text)
	require.True(t, s.consume(Event{Type: "result", Raw: json.RawMessage(`{"type":"result","subtype":"success","result":"hello"}`)}))
	u = <-s.updates
	require.Equal(t, agentruntime.TurnEnded, u.Kind)
	require.Equal(t, "turn", u.TurnID)
	require.Empty(t, s.turn)
	require.True(t, s.consume(Event{Type: "result", Raw: json.RawMessage(`{"type":"result"}`)}))
	require.Empty(t, s.updates)
}

func TestUnstartedSessionClose(t *testing.T) {
	s, err := (Adapter{}).Launch(t.Context(), discovery.Installation{Agent: "claude"}, "/unused")
	require.NoError(t, err)
	require.NoError(t, s.Close())
	require.NoError(t, s.Close())
	_, err = s.NewThread(t.Context())
	require.Error(t, err)
	_, ok := s.(agentruntime.PermissionSession)
	require.False(t, ok, "remote tools must remain gated")
}

func TestSessionLifetimeCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	s, err := (Adapter{}).Launch(ctx, discovery.Installation{Agent: "claude"}, "/unused")
	require.NoError(t, err)
	cancel()
	select {
	case <-s.Done():
	case <-time.After(time.Second):
		t.Fatal("session slot leaked")
	}
}
