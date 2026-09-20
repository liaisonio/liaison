package application

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/modelsettings"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

type selectionSettingsStore struct{}

func (*selectionSettingsStore) ReadAgentModelConfig(context.Context) ([]byte, error) { return nil, nil }
func (*selectionSettingsStore) WriteAgentModelConfig(context.Context, []byte) error  { return nil }

func TestSessionModelOwnershipAndRouting(t *testing.T) {
	ctx := context.Background()
	store := runtime.NewMemoryStore()
	service := newTestService(t, store, &fakeAuthorizer{}, []*model.IAMResourceRelation{{ResourceType: accessResourceType, ResourceID: 41, Relation: model.IAMRelationBelongsTo, SubjectType: model.IAMSubjectOrganization, SubjectID: 7}})
	manager, err := modelsettings.New(&selectionSettingsStore{}, "test", modelsettings.Config{Enabled: true, DefaultProvider: "test", Providers: []modelsettings.ProviderConfig{{ID: "test", Type: "custom", BaseURL: "https://example.test/v1", Model: "default", Models: []string{"default", "alternate"}}}}, func(context.Context, uint, string) error { return nil })
	require.NoError(t, err)
	service.models = manager
	runner := &fakeTurnRunner{}
	service.runner = runner
	actor := &model.User{Model: gorm.Model{ID: 9}}
	detail, err := service.CreateSession(ctx, CreateSessionRequest{Actor: actor, HandleID: "live-1", Kind: tool.SessionAccess})
	require.NoError(t, err)
	choice := runtime.ModelSelection{ProviderID: "test", Model: "alternate"}
	_, err = service.SetSessionModel(ctx, &model.User{Model: gorm.Model{ID: 10}}, detail.Session.ID, detail.Session.Version, choice)
	require.ErrorIs(t, err, ErrNotFound)
	_, err = service.SetSessionModel(ctx, actor, detail.Session.ID, detail.Session.Version, runtime.ModelSelection{ProviderID: "test", Model: "removed"})
	require.ErrorIs(t, err, ErrInvalid)
	saved, err := service.SetSessionModel(ctx, actor, detail.Session.ID, detail.Session.Version, choice)
	require.NoError(t, err)
	_, err = service.RunTurn(ctx, RunTurnRequest{Actor: actor, SessionID: saved.ID, Prompt: "hello"})
	require.NoError(t, err)
	require.Equal(t, choice, runner.request.Selection)
	_, err = service.SetSessionModel(ctx, actor, saved.ID, saved.Version, runtime.ModelSelection{})
	require.NoError(t, err)
	_, err = service.RunTurn(ctx, RunTurnRequest{Actor: actor, SessionID: saved.ID, Prompt: "hello"})
	require.NoError(t, err)
	require.Equal(t, "default", runner.request.Selection.Model)
	current, err := store.GetSession(ctx, saved.ID)
	require.NoError(t, err)
	_, err = store.SetSessionModel(ctx, saved.ID, current.Version, runtime.ModelSelection{ProviderID: "test", Model: "removed"})
	require.NoError(t, err)
	_, err = service.RunTurn(ctx, RunTurnRequest{Actor: actor, SessionID: saved.ID, Prompt: "no fallback"})
	require.ErrorIs(t, err, ErrInvalid)
}
