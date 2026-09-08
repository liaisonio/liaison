package runtime

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemoryStore_AttachmentLifecycle(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))

	created, err := store.CreateAttachment(ctx, Attachment{
		ID: "attachment", AgentSessionID: "session", AccessID: 10, ApplicationID: 20,
		Protocol: tool.ProtocolWebSSH, Capabilities: []tool.Capability{"terminal.read", "terminal.execute"}, State: AttachmentConnected,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), created.Generation)

	// Returned values cannot mutate the store's capability snapshot.
	created.Capabilities[0] = "changed"
	persisted, err := store.GetAttachment(ctx, "attachment")
	require.NoError(t, err)
	assert.Equal(t, tool.Capability("terminal.read"), persisted.Capabilities[0])

	session, err := store.GetSession(ctx, "session")
	require.NoError(t, err)
	session, err = store.SetActiveAttachment(ctx, session.ID, persisted.ID, session.Version)
	require.NoError(t, err)
	assert.Equal(t, persisted.ID, session.ActiveAttachmentID)

	persisted.State = AttachmentDisconnected
	updated, err := store.UpdateAttachment(ctx, persisted, persisted.Generation)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), updated.Generation)
	assert.Equal(t, AttachmentDisconnected, updated.State)

	_, err = store.UpdateAttachment(ctx, persisted, persisted.Generation)
	assert.ErrorIs(t, err, ErrVersionConflict)
	updated.AccessID++
	_, err = store.UpdateAttachment(ctx, updated, updated.Generation)
	assert.EqualError(t, err, "attachment resource binding is immutable")

	attachments, err := store.ListAttachments(ctx, "session")
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	assert.Equal(t, uint64(2), attachments[0].Generation)
}

func TestMemoryStore_RejectsCrossSessionAndRemovedActiveAttachment(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "one", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	require.NoError(t, store.CreateSession(ctx, Session{ID: "two", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	attachment, err := store.CreateAttachment(ctx, Attachment{ID: "attachment", AgentSessionID: "one", AccessID: 10, ApplicationID: 20,
		Protocol: tool.ProtocolMySQL, Capabilities: []tool.Capability{"data.query"}, State: AttachmentConnected})
	require.NoError(t, err)

	two, err := store.GetSession(ctx, "two")
	require.NoError(t, err)
	_, err = store.SetActiveAttachment(ctx, two.ID, attachment.ID, two.Version)
	assert.ErrorIs(t, err, ErrAttachmentNotFound)

	attachment.State = AttachmentRemoved
	attachment, err = store.UpdateAttachment(ctx, attachment, attachment.Generation)
	require.NoError(t, err)
	one, err := store.GetSession(ctx, "one")
	require.NoError(t, err)
	_, err = store.SetActiveAttachment(ctx, one.ID, attachment.ID, one.Version)
	assert.ErrorIs(t, err, ErrAttachmentNotFound)
}

func TestMemoryStore_OnlyAllowsOneActiveTurnPerSession(t *testing.T) {
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(context.Background(), Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	results := make(chan error, 2)
	for _, turnID := range []string{"turn-1", "turn-2"} {
		turnID := turnID
		go func() {
			defer waitGroup.Done()
			_, _, err := store.StartTurn(context.Background(), "session", Turn{ID: turnID})
			results <- err
		}()
	}
	waitGroup.Wait()
	close(results)

	var success, conflict int
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrTurnAlreadyActive) {
			conflict++
		}
	}
	assert.Equal(t, 1, success)
	assert.Equal(t, 1, conflict)
}

func TestMemoryStore_TerminalTurnReleasesSession(t *testing.T) {
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(context.Background(), Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	_, turn, err := store.StartTurn(context.Background(), "session", Turn{ID: "turn-1"})
	require.NoError(t, err)
	turn, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnRunning, "", "")
	require.NoError(t, err)
	turn, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnCompleted, "", "")
	require.NoError(t, err)
	assert.NotNil(t, turn.CompletedAt)

	session, err := store.GetSession(context.Background(), "session")
	require.NoError(t, err)
	assert.Empty(t, session.ActiveTurnID)
	_, _, err = store.StartTurn(context.Background(), "session", Turn{ID: "turn-2"})
	assert.NoError(t, err)
}

func TestMemoryStore_TurnWaitingApprovalCanResume(t *testing.T) {
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(context.Background(), Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	_, turn, err := store.StartTurn(context.Background(), "session", Turn{ID: "turn"})
	require.NoError(t, err)
	turn, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnRunning, "", "")
	require.NoError(t, err)
	turn, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnWaitingApproval, "", "")
	require.NoError(t, err)
	turn, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnRunning, "", "")
	require.NoError(t, err)
	assert.Equal(t, TurnRunning, turn.Status)
}

func TestMemoryStore_RejectsTerminalTurnMutationAndVersionConflicts(t *testing.T) {
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(context.Background(), Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	_, turn, err := store.StartTurn(context.Background(), "session", Turn{ID: "turn"})
	require.NoError(t, err)
	_, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version+1, TurnRunning, "", "")
	assert.ErrorIs(t, err, ErrVersionConflict)

	turn, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnCancelled, "cancelled", "user cancelled")
	require.NoError(t, err)
	_, err = store.TransitionTurn(context.Background(), turn.ID, turn.Version, TurnRunning, "", "")
	assert.ErrorIs(t, err, ErrInvalidTransition)
	_, _, err = store.AppendStep(context.Background(), turn.ID, turn.Version, Step{ID: "late", Kind: StepTool})
	assert.ErrorIs(t, err, ErrInvalidTransition)
}

func TestMemoryStore_AppendStepAssignsMonotonicSequence(t *testing.T) {
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(context.Background(), Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	_, turn, err := store.StartTurn(context.Background(), "session", Turn{ID: "turn"})
	require.NoError(t, err)
	turn, first, err := store.AppendStep(context.Background(), turn.ID, turn.Version, Step{ID: "step-1", Kind: StepModel})
	require.NoError(t, err)
	_, second, err := store.AppendStep(context.Background(), turn.ID, turn.Version, Step{ID: "step-2", Kind: StepTool})
	require.NoError(t, err)
	assert.Equal(t, uint32(1), first.Sequence)
	assert.Equal(t, uint32(2), second.Sequence)
}
