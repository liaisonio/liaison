package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

type approvalAgent struct {
	*fakeAgent
	files    int
	rejects  int
	fail     bool
	snapshot func()
}

func approvalFixture(t *testing.T) (*session, *approvalAgent) {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	a := &approvalAgent{fakeAgent: &fakeAgent{events: make(chan rpc.Message), done: make(chan struct{})}}
	s := &session{agent: a, project: root, access: strings.Repeat("a", 32), thread: "thread-owned", turn: "turn", running: true, status: "ok", changed: make(chan struct{})}
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	return s, a
}

func (a *approvalAgent) ApproveFileChange(context.Context, rpc.Message) error {
	a.files++
	if a.snapshot != nil {
		a.snapshot()
	}
	if a.fail {
		return errors.New("uncertain write")
	}
	return nil
}
func (a *approvalAgent) RejectRequest(context.Context, rpc.Message) error { a.rejects++; return nil }

func TestFileApprovalBoundaries(t *testing.T) {
	for _, test := range []struct {
		name     string
		decision string
		fail     bool
	}{{"accept", "accept", false}, {"decline", "decline", false}, {"uncertain write", "accept", true}} {
		t.Run(test.name, func(t *testing.T) {
			s, a := approvalFixture(t)
			a.fail = test.fail
			a.snapshot = func() { s.snapshot() } // Must not hold session lock while writing to the adapter.
			s.recordActivityLocked("file", "fileChange", "", 0, false)
			s.activities[0].Changes = []proto.AgentFileChange{{Path: "new.txt", Kind: "add", Diff: "+hello"}}
			event := rpc.Message{ID: json.RawMessage(`"file-request"`), Method: "item/fileChange/requestApproval", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn","itemId":"file"}`)}
			require.True(t, s.queueApproval(event))
			require.True(t, s.queueApproval(event))
			require.Len(t, s.snapshot().Approvals, 1)
			q := proto.EdgeAgentRequest{Action: "approve", ApprovalID: s.snapshot().Approvals[0].ID, Decision: test.decision}
			got := s.managePermissions(context.Background(), q)
			if test.fail {
				require.True(t, got.Closed)
			} else {
				require.Equal(t, "ok", got.Status)
			}
			require.Empty(t, s.snapshot().Approvals)
			require.NotEqual(t, "ok", s.managePermissions(context.Background(), q).Status)
			if test.decision == "accept" {
				require.Equal(t, 1, a.files)
			} else {
				require.Equal(t, 1, a.rejects)
				require.Zero(t, a.files)
			}
		})
	}
}

func TestUnsafeApprovalRequestsStayClosed(t *testing.T) {
	s, _ := approvalFixture(t)
	s.recordActivityLocked("file", "fileChange", "", 0, false)
	s.activities[0].Changes = []proto.AgentFileChange{{Path: "new.txt", Kind: "add", Diff: "+hello"}}
	for _, params := range []string{
		`{"threadId":"foreign","turnId":"turn","itemId":"file"}`,
		`{"threadId":"thread-owned","turnId":"old","itemId":"file"}`,
		`{"threadId":"thread-owned","turnId":"turn","itemId":"missing"}`,
		`{"threadId":"thread-owned","turnId":"turn","itemId":"file","grantRoot":"/"}`,
	} {
		require.False(t, s.queueApproval(rpc.Message{ID: json.RawMessage(`1`), Method: "item/fileChange/requestApproval", Params: json.RawMessage(params)}))
	}
	valid := rpc.Message{ID: json.RawMessage(`1`), Method: "item/fileChange/requestApproval", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn","itemId":"file"}`)}
	s.activities[0].Truncated = true
	require.False(t, s.queueApproval(valid))
	s.activities[0].Truncated = false
	s.activities[0].Changes[0].Diff = ""
	require.False(t, s.queueApproval(valid))
	s.activities[0].Changes[0].Diff = "+hello"
	s.activities[0].Changes[0].Path = "../outside"
	require.False(t, s.queueApproval(valid))
	for _, extra := range []string{`"networkApprovalContext":{}`, `"additionalPermissions":{}`} {
		require.False(t, s.queueApproval(rpc.Message{ID: json.RawMessage(`1`), Method: "item/commandExecution/requestApproval", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn","command":"ls",` + extra + `}`)}))
	}
	require.Empty(t, s.snapshot().Approvals)
}

func TestApprovalPathContainment(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	outside := t.TempDir()
	require.NoError(t, os.Symlink(outside, filepath.Join(root, "link")))
	require.NoError(t, os.Symlink(filepath.Join(outside, "missing"), filepath.Join(root, "dangling")))
	for _, path := range []string{"", ".", "..", "../outside", "link/file", "dangling", "missing-parent/file", outside} {
		require.False(t, approvalPathInProject(root, path), path)
	}
	require.True(t, approvalPathInProject(root, "new.txt"))
	require.True(t, approvalPathInProject(root, filepath.Join(root, "new.txt")))
}

func TestApprovalDisplayBudgetNeverTruncatesGrantDetails(t *testing.T) {
	s, _ := approvalFixture(t)
	s.recordActivityLocked("file", "fileChange", "", 0, false)
	s.activities[0].Changes = []proto.AgentFileChange{{Path: "new.txt", Kind: "add", Diff: strings.Repeat("<", 16<<10)}}
	event := rpc.Message{ID: json.RawMessage(`1`), Method: "item/fileChange/requestApproval", Params: json.RawMessage(`{"threadId":"thread-owned","turnId":"turn","itemId":"file"}`)}
	require.True(t, s.queueApproval(event))
	event.ID = json.RawMessage(`2`)
	require.False(t, s.queueApproval(event), "escaped JSON must fit the pending-request budget")
	require.Len(t, s.snapshot().Approvals, 1)
	require.Equal(t, s.activities[0].Changes[0].Diff, s.snapshot().Approvals[0].Changes[0].Diff)
}
