package application

import (
	"context"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type assistanceGeneratorFunc func(context.Context, assistance.Binding, assistance.Input) (string, error)

func (f assistanceGeneratorFunc) Suggest(ctx context.Context, b assistance.Binding, i assistance.Input) (string, error) {
	return f(ctx, b, i)
}

func TestAssistance_UsesShellSessionAndBindsLiveGeneration(t *testing.T) {
	service := newTestService(t, runtime.NewMemoryStore(), &fakeAuthorizer{}, []*model.IAMResourceRelation{{
		ResourceType: accessResourceType, ResourceID: 41, Relation: model.IAMRelationBelongsTo,
		SubjectType: model.IAMSubjectOrganization, SubjectID: 1,
	}})
	actor := &model.User{Model: gorm.Model{ID: 7}}
	detail, err := service.CreateSession(context.Background(), CreateSessionRequest{Actor: actor, HandleID: "live-1", Kind: tool.SessionShell})
	require.NoError(t, err)
	service.runner = &completionContextRunner{}
	calls := 0
	session, err := service.NewAssistanceSession(context.Background(), actor, "live-1", assistanceGeneratorFunc(func(_ context.Context, b assistance.Binding, input assistance.Input) (string, error) {
		calls++
		require.Equal(t, detail.Session.ID, input.AgentSessionID)
		require.Equal(t, "shared conclusion", input.AgentContext[0].Content)
		require.Equal(t, assistance.Binding{OwnerID: 7, HandleID: "live-1", Protocol: "ssh"}, b)
		return "pwd", nil
	}))
	require.NoError(t, err)
	t.Cleanup(session.Close)
	result, err := session.Suggest(context.Background(), assistance.Input{AgentSessionID: detail.Session.ID, Revision: 1})
	require.NoError(t, err)
	require.Equal(t, "pwd", result.Text)
	binder := service.binder.(fakeBinder)
	binder.snapshot.Generation++
	service.binder = binder
	_, err = session.Suggest(context.Background(), assistance.Input{AgentSessionID: detail.Session.ID, Revision: 2})
	require.ErrorIs(t, err, assistance.ErrClosed)
	require.Equal(t, 1, calls)
}

type completionContextRunner struct{ fakeTurnRunner }

func (*completionContextRunner) ShellCompletionContext(context.Context, tool.Principal, string) ([]runtime.ModelMessage, error) {
	return []runtime.ModelMessage{{Role: runtime.RoleUser, Content: "shared conclusion"}}, nil
}

func TestShellCompletion_RejectsWrongOwnerKindHandleAndArchivedSession(t *testing.T) {
	for _, scenario := range []string{"missing", "owner", "kind", "handle", "archived", "revoked-after-generation"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			store := runtime.NewMemoryStore()
			auth := &fakeAuthorizer{}
			service := newTestService(t, store, auth, []*model.IAMResourceRelation{{ResourceType: accessResourceType, ResourceID: 41, Relation: model.IAMRelationBelongsTo, SubjectType: model.IAMSubjectOrganization, SubjectID: 1}})
			actor := &model.User{Model: gorm.Model{ID: 7}}
			req := CreateSessionRequest{Actor: actor, HandleID: "live-1", Kind: tool.SessionShell}
			if scenario == "owner" {
				req.Actor = &model.User{Model: gorm.Model{ID: 8}}
			}
			if scenario == "kind" {
				req.Kind = tool.SessionAccess
			}
			detail, err := service.CreateSession(ctx, req)
			require.NoError(t, err)
			if scenario == "archived" {
				_, err = store.ArchiveSession(ctx, detail.Session.ID, detail.Session.Version)
				require.NoError(t, err)
			}
			id := detail.Session.ID
			if scenario == "missing" {
				id = ""
			}
			binding := assistance.Binding{OwnerID: 7, HandleID: "live-1", Protocol: "ssh"}
			if scenario == "handle" {
				binding.HandleID = "other-connection"
			}
			service.runner = &completionContextRunner{}
			calls := 0
			generator := &shellGenerator{service: service, actor: *actor, inner: assistanceGeneratorFunc(func(context.Context, assistance.Binding, assistance.Input) (string, error) {
				calls++
				_, e := store.ArchiveSession(ctx, detail.Session.ID, detail.Session.Version)
				require.NoError(t, e)
				return " suffix", nil
			})}
			_, err = generator.Suggest(ctx, binding, assistance.Input{AgentSessionID: id})
			require.Error(t, err)
			if scenario == "revoked-after-generation" {
				require.Equal(t, 1, calls)
			} else {
				require.Zero(t, calls)
			}
		})
	}
}
