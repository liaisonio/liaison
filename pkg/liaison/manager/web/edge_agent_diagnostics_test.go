package web

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestAgentRequestDiagnosticPrivacyAndActionAllowlist(t *testing.T) {
	for _, action := range []string{"poll", "watch", "file_read", "send\nsecret", ""} {
		_, ok := agentRequestDiagnostic(proto.EdgeAgentRequest{Action: action}, proto.EdgeAgentResult{}, nil, time.Second)
		require.False(t, ok)
	}
	for _, action := range []string{"start", "resume", "send", "interrupt", "stop"} {
		r, ok := agentRequestDiagnostic(proto.EdgeAgentRequest{Action: action, Text: "sensitive-prompt", SessionID: "sensitive-id", Project: "/sensitive-project"}, proto.EdgeAgentResult{Status: "sensitive-provider-error"}, errors.New("sensitive-credential"), -time.Second)
		require.True(t, ok)
		require.Empty(t, r.SessionID)
		require.Equal(t, "failed", r.Outcome)
		require.Zero(t, r.ElapsedMS)
		data, err := json.Marshal(r)
		require.NoError(t, err)
		require.NotContains(t, string(data), "sensitive")
	}
	r, ok := agentRequestDiagnostic(proto.EdgeAgentRequest{Action: "send", SessionID: strings.Repeat("a", 32)}, proto.EdgeAgentResult{Window: 4, Status: "ok"}, nil, 123*time.Millisecond)
	require.True(t, ok)
	require.Equal(t, "ok", r.Outcome)
	require.EqualValues(t, 123, r.ElapsedMS)
	require.EqualValues(t, 4, r.Window)
	require.Equal(t, strings.Repeat("a", 32), r.SessionID)
	canceled, _ := agentRequestDiagnostic(proto.EdgeAgentRequest{Action: "send"}, proto.EdgeAgentResult{}, context.Canceled, time.Second)
	require.Equal(t, "canceled", canceled.Outcome)
	require.NotEqual(t, r.TraceID, canceled.TraceID)
}
