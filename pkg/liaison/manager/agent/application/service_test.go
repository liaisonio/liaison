package application

import (
	"context"
	"errors"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestCreateSession_DerivesBindingAndOrganizationFromServerState(t *testing.T) {
	store := runtime.NewMemoryStore()
	authorizer := &fakeAuthorizer{}
	service := newTestService(t, store, authorizer, []*model.IAMResourceRelation{{
		ResourceType: accessResourceType,
		ResourceID:   41,
		Relation:     model.IAMRelationBelongsTo,
		SubjectType:  model.IAMSubjectOrganization,
		SubjectID:    7,
	}})
	actor := &model.User{Model: gorm.Model{ID: 9}}

	detail, err := service.CreateSession(context.Background(), CreateSessionRequest{Actor: actor, HandleID: "live-1"})
	require.NoError(t, err)
	assert.Equal(t, uint(7), detail.Session.OrganizationID)
	assert.Equal(t, uint(9), detail.Session.CreatedBy)
	assert.Equal(t, "New WEB SSH session", detail.Session.Title)
	require.Len(t, detail.Attachments, 1)
	assert.Equal(t, "live-1", detail.Attachments[0].ID)
	assert.Equal(t, uint(41), detail.Attachments[0].AccessID)
	assert.Equal(t, uint(51), detail.Attachments[0].ApplicationID)
	assert.Equal(t, uint64(3), detail.Attachments[0].Generation)
	assert.Equal(t, []permissionCall{{organizationID: 7, resource: "agent_sessions", action: "create"}}, authorizer.organizationCalls)
}

func TestCreateSession_WhenOrganizationRelationMissing_DoesNotPersistPartialSession(t *testing.T) {
	store := runtime.NewMemoryStore()
	service := newTestService(t, store, &fakeAuthorizer{}, nil)
	actor := &model.User{Model: gorm.Model{ID: 9}}

	_, err := service.CreateSession(context.Background(), CreateSessionRequest{Actor: actor, HandleID: "live-1"})
	assert.ErrorIs(t, err, ErrNotFound)
	sessions, listErr := store.ListSessions(context.Background(), actor.ID)
	require.NoError(t, listErr)
	assert.Empty(t, sessions)
}

func TestGetSession_WhenOwnedByAnotherUser_ReturnsNotFound(t *testing.T) {
	store := runtime.NewMemoryStore()
	service := newTestService(t, store, &fakeAuthorizer{}, nil)
	require.NoError(t, store.CreateSession(context.Background(), runtime.Session{
		ID: "session-1", OrganizationID: 7, CreatedBy: 10, Status: runtime.SessionActive,
	}))

	_, err := service.GetSession(context.Background(), &model.User{Model: gorm.Model{ID: 9}}, "session-1")
	assert.ErrorIs(t, err, ErrNotFound)
}

func TestGetSession_ReturnsDurableReconnectView(t *testing.T) {
	store := runtime.NewMemoryStore()
	service := newTestService(t, store, &fakeAuthorizer{}, nil)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, runtime.Session{
		ID: "session-1", OrganizationID: 7, CreatedBy: 9, Status: runtime.SessionActive,
	}))
	_, turn, err := store.StartTurn(ctx, "session-1", runtime.Turn{ID: "turn-1"})
	require.NoError(t, err)
	turn, step, err := store.AppendStep(ctx, turn.ID, turn.Version, runtime.Step{ID: "step-1", Kind: runtime.StepModel, Status: runtime.StepRunning})
	require.NoError(t, err)
	_, err = store.AppendMessage(ctx, runtime.Message{
		ID: "message-1", AgentSessionID: "session-1", TurnID: turn.ID,
		Value: runtime.ModelMessage{Role: runtime.RoleUser, Content: "hello"},
	})
	require.NoError(t, err)

	detail, err := service.GetSession(ctx, &model.User{Model: gorm.Model{ID: 9}}, "session-1")
	require.NoError(t, err)
	require.Len(t, detail.Turns, 1)
	require.Len(t, detail.Steps, 1)
	assert.Equal(t, step.ID, detail.Steps[0].ID)
	require.Len(t, detail.Messages, 1)
	assert.Equal(t, "hello", detail.Messages[0].Value.Content)
}

func TestListSessions_OnlyReturnsActorSessions(t *testing.T) {
	store := runtime.NewMemoryStore()
	authorizer := &fakeAuthorizer{}
	service := newTestService(t, store, authorizer, nil)
	ctx := context.Background()
	require.NoError(t, store.CreateSession(ctx, runtime.Session{ID: "mine", OrganizationID: 7, CreatedBy: 9, Status: runtime.SessionActive}))
	require.NoError(t, store.CreateSession(ctx, runtime.Session{ID: "other", OrganizationID: 7, CreatedBy: 10, Status: runtime.SessionActive}))

	sessions, err := service.ListSessions(ctx, &model.User{Model: gorm.Model{ID: 9}})
	require.NoError(t, err)
	require.Len(t, sessions, 1)
	assert.Equal(t, "mine", sessions[0].ID)
	assert.Equal(t, []permissionCall{{resource: "agent_sessions", action: "read"}, {resource: "management_agent_sessions", action: "read"}}, authorizer.resourceCalls)
}

func TestRunTurn_DerivesPrincipalAndRuntimePolicy(t *testing.T) {
	store := runtime.NewMemoryStore()
	authorizer := &fakeAuthorizer{}
	runner := &fakeTurnRunner{result: runtime.RunResult{Text: "done"}}
	service, err := NewService(store, fakeBinder{}, fakeRelations{}, authorizer, fixedIDs{}, WithTurnRunner(runner))
	require.NoError(t, err)
	require.NoError(t, store.CreateSession(context.Background(), runtime.Session{
		ID: "session-1", OrganizationID: 7, CreatedBy: 9, Status: runtime.SessionActive,
	}))

	result, err := service.RunTurn(context.Background(), RunTurnRequest{
		Actor: &model.User{Model: gorm.Model{ID: 9}}, SessionID: "session-1", Prompt: " inspect schema ",
	})
	require.NoError(t, err)
	assert.Equal(t, "done", result.Text)
	assert.Equal(t, "inspect schema", runner.request.Prompt)
	assert.Equal(t, tool.Principal{UserID: 9, OrganizationID: 7}, runner.request.Principal)
	assert.Equal(t, "agent-medium-risk-v1", runner.request.PolicyRevision)
	assert.Equal(t, tool.DefaultDisclosureBudget(), runner.request.Budget)
	assert.Equal(t, []permissionCall{{organizationID: 7, resource: "agent_sessions", action: "use"}}, authorizer.organizationCalls)
}

func TestRunTurn_WhenRuntimeDisabled_ReturnsUnavailable(t *testing.T) {
	store := runtime.NewMemoryStore()
	service := newTestService(t, store, &fakeAuthorizer{}, nil)
	require.NoError(t, store.CreateSession(context.Background(), runtime.Session{
		ID: "session-1", OrganizationID: 7, CreatedBy: 9, Status: runtime.SessionActive,
	}))
	_, err := service.RunTurn(context.Background(), RunTurnRequest{
		Actor: &model.User{Model: gorm.Model{ID: 9}}, SessionID: "session-1", Prompt: "hello",
	})
	assert.ErrorIs(t, err, ErrUnavailable)
}

func newTestService(t *testing.T, store Store, authorizer Authorizer, relations []*model.IAMResourceRelation) *Service {
	t.Helper()
	service, err := NewService(store, fakeBinder{snapshot: tool.AttachmentSnapshot{
		ID: "ignored", AccessHandleID: "live-1", AccessID: 41, ApplicationID: 51,
		Protocol: tool.ProtocolWebSSH, Capabilities: []tool.Capability{"terminal.read", "terminal.execute"}, Generation: 3,
	}}, fakeRelations{relations: relations}, authorizer, fixedIDs{})
	require.NoError(t, err)
	return service
}

type fakeBinder struct {
	snapshot tool.AttachmentSnapshot
	err      error
}

func (binder fakeBinder) Bind(context.Context, tool.Principal, string) (tool.AttachmentSnapshot, error) {
	return binder.snapshot, binder.err
}

type fakeRelations struct {
	relations []*model.IAMResourceRelation
	err       error
}

func (relations fakeRelations) ListIAMResourceRelations(string, uint64) ([]*model.IAMResourceRelation, error) {
	return relations.relations, relations.err
}

type permissionCall struct {
	organizationID uint
	resource       string
	action         string
}

type fakeAuthorizer struct {
	resourceCalls     []permissionCall
	organizationCalls []permissionCall
	err               error
}

func (authorizer *fakeAuthorizer) RequireResourcePermission(_ *model.User, resource, action string) error {
	authorizer.resourceCalls = append(authorizer.resourceCalls, permissionCall{resource: resource, action: action})
	return authorizer.err
}

func (authorizer *fakeAuthorizer) RequireOrganizationResourcePermission(_ *model.User, organizationID uint, resource, action string) error {
	authorizer.organizationCalls = append(authorizer.organizationCalls, permissionCall{organizationID: organizationID, resource: resource, action: action})
	return authorizer.err
}

type fixedIDs struct{}

func (fixedIDs) NewID(prefix string) (string, error) {
	if prefix == "" {
		return "", errors.New("prefix is required")
	}
	return prefix + "-1", nil
}

type fakeTurnRunner struct {
	request runtime.RunRequest
	result  runtime.RunResult
	err     error
}

func (runner *fakeTurnRunner) Run(_ context.Context, request runtime.RunRequest) (runtime.RunResult, error) {
	runner.request = request
	return runner.result, runner.err
}

func (runner *fakeTurnRunner) ResolveApproval(_ context.Context, request runtime.RunRequest, _ string, _ bool, _ string) (runtime.RunResult, error) {
	runner.request = request
	return runner.result, runner.err
}

func (runner *fakeTurnRunner) ListApprovals(context.Context, string) ([]runtime.ApprovalView, error) {
	return []runtime.ApprovalView{{ID: "approval-1", Status: runtime.ApprovalPending}}, nil
}
