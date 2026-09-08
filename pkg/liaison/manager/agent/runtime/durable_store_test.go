package runtime

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDurableStore_PersistsManagementKindWithoutAttachment(t *testing.T) {
	repository := newAgentTestDAO(t)
	store, err := NewDurableStore(repository)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "management", Kind: tool.SessionManagement, OrganizationID: 1, CreatedBy: 2}))
	reopened, err := NewDurableStore(repository)
	require.NoError(t, err)
	session, err := reopened.GetSession(ctx, "management")
	require.NoError(t, err)
	require.Equal(t, tool.SessionManagement, session.Kind)
	require.Empty(t, session.ActiveAttachmentID)
	_, _, err = store.CreateSessionWithAttachment(ctx, Session{ID: "invalid", Kind: tool.SessionManagement, OrganizationID: 1, CreatedBy: 2}, Attachment{})
	require.Error(t, err)
	require.Error(t, store.CreateSession(ctx, Session{ID: "unknown", Kind: "admin", OrganizationID: 1, CreatedBy: 2}))
}

func TestManagementSessionsRejectAttachmentMutation(t *testing.T) {
	durable, err := NewDurableStore(newAgentTestDAO(t))
	require.NoError(t, err)
	for name, store := range map[string]Store{"memory": NewMemoryStore(), "durable": durable} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			require.NoError(t, store.CreateSession(ctx, Session{ID: "management", Kind: tool.SessionManagement, OrganizationID: 1, CreatedBy: 2}))
			session, err := store.GetSession(ctx, "management")
			require.NoError(t, err)
			_, err = store.CreateAttachment(ctx, Attachment{ID: "ssh", AgentSessionID: session.ID, AccessID: 1, ApplicationID: 2, Protocol: tool.ProtocolWebSSH, Capabilities: []tool.Capability{"terminal.read"}, State: AttachmentConnected})
			require.ErrorContains(t, err, "management")
			_, err = store.SetActiveAttachment(ctx, session.ID, "ssh", session.Version)
			require.ErrorContains(t, err, "management")
			attachments, err := store.ListAttachments(ctx, session.ID)
			require.NoError(t, err)
			require.Empty(t, attachments)
		})
	}
}

func TestDurableStore_PersistsAttachmentLifecycle(t *testing.T) {
	repository := newAgentTestDAO(t)
	store, err := NewDurableStore(repository)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))

	attachment, err := store.CreateAttachment(ctx, Attachment{
		ID: "attachment", AgentSessionID: "session", AccessID: 10, ApplicationID: 20,
		Protocol: tool.ProtocolPostgreSQL, Capabilities: []tool.Capability{"data.schema", "data.query"}, State: AttachmentConnected,
	})
	require.NoError(t, err)
	assert.Equal(t, uint64(1), attachment.Generation)

	session, err := store.GetSession(ctx, "session")
	require.NoError(t, err)
	session, err = store.SetActiveAttachment(ctx, session.ID, attachment.ID, session.Version)
	require.NoError(t, err)
	assert.Equal(t, attachment.ID, session.ActiveAttachmentID)

	attachment.State = AttachmentDisconnected
	attachment, err = store.UpdateAttachment(ctx, attachment, attachment.Generation)
	require.NoError(t, err)
	assert.Equal(t, uint64(2), attachment.Generation)

	reopened, err := NewDurableStore(repository)
	require.NoError(t, err)
	persisted, err := reopened.GetAttachment(ctx, attachment.ID)
	require.NoError(t, err)
	assert.Equal(t, AttachmentDisconnected, persisted.State)
	assert.Equal(t, []tool.Capability{"data.schema", "data.query"}, persisted.Capabilities)
	attachments, err := reopened.ListAttachments(ctx, "session")
	require.NoError(t, err)
	require.Len(t, attachments, 1)
	assert.Equal(t, attachment.ID, attachments[0].ID)

	_, err = reopened.UpdateAttachment(ctx, persisted, 1)
	assert.ErrorIs(t, err, ErrVersionConflict)
	persisted.Protocol = tool.ProtocolMySQL
	_, err = reopened.UpdateAttachment(ctx, persisted, persisted.Generation)
	assert.EqualError(t, err, "attachment resource binding is immutable")
}

func TestDurableStore_CreateSessionWithAttachmentPersistsOneAggregate(t *testing.T) {
	store, err := NewDurableStore(newAgentTestDAO(t))
	require.NoError(t, err)
	ctx := context.Background()
	session, attachment, err := store.CreateSessionWithAttachment(ctx,
		Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive},
		Attachment{ID: "live", AgentSessionID: "session", AccessID: 10, ApplicationID: 20,
			Protocol: tool.ProtocolWebSSH, Capabilities: []tool.Capability{"terminal.read"}, Generation: 4, State: AttachmentConnected})
	require.NoError(t, err)
	assert.Equal(t, attachment.ID, session.ActiveAttachmentID)
	persisted, err := store.GetSession(ctx, session.ID)
	require.NoError(t, err)
	assert.Equal(t, attachment.ID, persisted.ActiveAttachmentID)
	listed, err := store.ListSessions(ctx, 2)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	assert.Equal(t, session.ID, listed[0].ID)
	other, err := store.ListSessions(ctx, 3)
	require.NoError(t, err)
	assert.Empty(t, other)
}

func TestDurableStore_RejectsCrossSessionAttachmentActivation(t *testing.T) {
	store, err := NewDurableStore(newAgentTestDAO(t))
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "one", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	require.NoError(t, store.CreateSession(ctx, Session{ID: "two", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))
	attachment, err := store.CreateAttachment(ctx, Attachment{ID: "attachment", AgentSessionID: "one", AccessID: 10, ApplicationID: 20,
		Protocol: tool.ProtocolRedis, Capabilities: []tool.Capability{"data.query"}, State: AttachmentConnected})
	require.NoError(t, err)
	two, err := store.GetSession(ctx, "two")
	require.NoError(t, err)
	_, err = store.SetActiveAttachment(ctx, two.ID, attachment.ID, two.Version)
	assert.ErrorIs(t, err, ErrAttachmentNotFound)
}

func TestDurableStore_PersistsStateAndReleasesCompletedTurn(t *testing.T) {
	repository := newAgentTestDAO(t)
	store, err := NewDurableStore(repository)
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))

	_, turn, err := store.StartTurn(ctx, "session", Turn{ID: "turn"})
	require.NoError(t, err)
	turn, err = store.TransitionTurn(ctx, turn.ID, turn.Version, TurnRunning, "", "")
	require.NoError(t, err)
	turn, step, err := store.AppendStep(ctx, turn.ID, turn.Version, Step{ID: "step", Kind: StepTool, Status: StepRunning})
	require.NoError(t, err)
	_, err = store.CompleteStep(ctx, step.ID, []byte(`{"ok":true}`))
	require.NoError(t, err)
	turn, err = store.TransitionTurn(ctx, turn.ID, turn.Version, TurnCompleted, "", "")
	require.NoError(t, err)

	reopened, err := NewDurableStore(repository)
	require.NoError(t, err)
	persistedTurn, err := reopened.GetTurn(ctx, turn.ID)
	require.NoError(t, err)
	assert.Equal(t, TurnCompleted, persistedTurn.Status)
	persistedStep, err := reopened.GetStep(ctx, step.ID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"ok":true}`, string(persistedStep.Output))
	steps, err := reopened.ListSessionSteps(ctx, "session")
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, step.ID, steps[0].ID)
	persistedSession, err := reopened.GetSession(ctx, "session")
	require.NoError(t, err)
	assert.Empty(t, persistedSession.ActiveTurnID)
}

func TestDurableStore_OnlyCreatesOneConcurrentActiveTurn(t *testing.T) {
	store, err := NewDurableStore(newAgentTestDAO(t))
	require.NoError(t, err)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "session", OrganizationID: 1, CreatedBy: 2, Status: SessionActive}))

	var waitGroup sync.WaitGroup
	waitGroup.Add(2)
	results := make(chan error, 2)
	for _, id := range []string{"turn-a", "turn-b"} {
		id := id
		go func() {
			defer waitGroup.Done()
			_, _, startErr := store.StartTurn(ctx, "session", Turn{ID: id})
			results <- startErr
		}()
	}
	waitGroup.Wait()
	close(results)

	var successes, rejected int
	for startErr := range results {
		if startErr == nil {
			successes++
			continue
		}
		if errors.Is(startErr, ErrTurnAlreadyActive) || errors.Is(startErr, ErrVersionConflict) {
			rejected++
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, rejected)
}

func newAgentTestDAO(t *testing.T) dao.Dao {
	t.Helper()
	repository, err := dao.NewDao(&config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "agent.db")}})
	require.NoError(t, err)
	return repository
}
