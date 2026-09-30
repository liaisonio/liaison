package bridge

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

// 仅使用独立临时项目和固定 printf；不操作现有用户项目。
func TestClaudeNativeBridgeLifecycle(t *testing.T) {
	if os.Getenv("LIAISON_TEST_CLAUDE_BRIDGE") != "1" {
		t.Skip("native bridge test not enabled")
	}
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	project, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	store := filepath.Join(t.TempDir(), "bindings.json")
	b, err := New(ctx, store)
	require.NoError(t, err)
	defer func() { require.NoError(t, b.Close()) }()
	access := strings.Repeat("a", 32)
	invoke := func(bridge *Bridge, req proto.EdgeAgentRequest, seed *proto.EdgeAgentResume) proto.EdgeAgentResult {
		req.AccessID = access
		return bridge.Handle(ctx, proto.EdgeAgentRPCRequest{Version: 1, ActorID: "native-test", ProjectRoot: project, Request: req, Resume: seed})
	}
	discovered := b.Handle(ctx, proto.EdgeAgentRPCRequest{Version: 1, ActorID: "native-test", Request: proto.EdgeAgentRequest{Action: "discover"}})
	require.Equal(t, "ok", discovered.Status)
	installation := ""
	for _, i := range discovered.Installations {
		if i.Kind == "claude" {
			installation = i.ID
			break
		}
	}
	require.NotEmpty(t, installation)
	started := invoke(b, proto.EdgeAgentRequest{Action: "start", InstallationID: installation, Project: project}, nil)
	require.Equal(t, "ok", started.Status)
	require.NotEmpty(t, started.ThreadID)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	encoded := strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' {
			return r
		}
		return '-'
	}, project)
	checkpoint := filepath.Join(home, ".claude", "projects", encoded, started.ThreadID+".jsonl")
	t.Cleanup(func() {
		if err := os.Remove(checkpoint); err != nil && !os.IsNotExist(err) {
			t.Error("test transcript cleanup failed")
		}
		if err := os.Remove(filepath.Dir(checkpoint)); err != nil && !os.IsNotExist(err) {
			t.Log("non-empty test directory retained")
		}
	})
	code := strings.ReplaceAll(started.ThreadID, "-", "")[:12]
	sent := invoke(b, proto.EdgeAgentRequest{Action: "send", SessionID: started.SessionID, Text: "Remember code " + code + ". Call Bash exactly once with command `printf LIAISON_BRIDGE_PROBE`. No other command. Then reply READY."}, nil)
	require.Equal(t, "ok", sent.Status)
	approved := false
	await := func(bridge *Bridge, approve bool) proto.EdgeAgentResult {
		ticker := time.NewTicker(30 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.Fatal("native bridge timed out")
			case <-ticker.C:
			}
			r := invoke(bridge, proto.EdgeAgentRequest{Action: "poll", SessionID: started.SessionID}, nil)
			require.False(t, r.Closed, "native instance unexpectedly closed")
			if len(r.Approvals) > 0 {
				require.True(t, approve && !approved, "unexpected additional command; not approved")
				require.Equal(t, "printf LIAISON_BRIDGE_PROBE", r.Approvals[0].Command)
				r = invoke(bridge, proto.EdgeAgentRequest{Action: "approve", SessionID: started.SessionID, ApprovalID: r.Approvals[0].ID, Decision: "accept"}, nil)
				require.Equal(t, "ok", r.Status)
				approved = true
			}
			if !r.Running {
				require.Equal(t, "ok", r.Status)
				return r
			}
		}
	}
	finished := await(b, true)
	require.True(t, approved)
	require.NotEmpty(t, finished.Model)
	require.True(t, invoke(b, proto.EdgeAgentRequest{Action: "stop", SessionID: started.SessionID}, nil).Closed)
	require.NoError(t, b.Close())
	next, err := New(ctx, store)
	require.NoError(t, err)
	defer func() { require.NoError(t, next.Close()) }()
	seed := &proto.EdgeAgentResume{SessionID: started.SessionID, Revision: finished.Revision, StartedAt: started.StartedAt, Messages: finished.Messages, Title: "Native bridge test"}
	resumed := invoke(next, proto.EdgeAgentRequest{Action: "resume", SessionID: started.SessionID}, seed)
	require.Equal(t, "ok", resumed.Status)
	require.Equal(t, started.ThreadID, resumed.ThreadID)
	r := invoke(next, proto.EdgeAgentRequest{Action: "send", SessionID: started.SessionID, Text: "Reply with only the code I asked you to remember. Do not use tools."}, nil)
	require.Equal(t, "ok", r.Status)
	r = await(next, false)
	require.NotEmpty(t, r.Messages)
	if !strings.Contains(r.Messages[len(r.Messages)-1].Text, code) {
		t.Fatal("resumed context mismatch; output withheld")
	}
	t.Log("native discovery/start/approval/stream/stop/bridge-restart/resume verified")
}
