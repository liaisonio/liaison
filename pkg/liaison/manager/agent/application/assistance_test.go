package application

import (
	"context"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type assistanceGeneratorFunc func(context.Context, assistance.Binding, assistance.Input) (string, error)

func (f assistanceGeneratorFunc) Suggest(ctx context.Context, b assistance.Binding, i assistance.Input) (string, error) {
	return f(ctx, b, i)
}

func TestAssistance_IsolatedFromChatAndBoundToLiveGeneration(t *testing.T) {
	service := newTestService(t, runtime.NewMemoryStore(), &fakeAuthorizer{}, []*model.IAMResourceRelation{{
		ResourceType: accessResourceType, ResourceID: 41, Relation: model.IAMRelationBelongsTo,
		SubjectType: model.IAMSubjectOrganization, SubjectID: 1,
	}})
	// 补全不得访问聊天存储，包括创建会话。
	service.store = nil
	calls := 0
	session, err := service.NewAssistanceSession(context.Background(), &model.User{Model: gorm.Model{ID: 7}}, "live-1", assistanceGeneratorFunc(func(_ context.Context, b assistance.Binding, _ assistance.Input) (string, error) {
		calls++
		require.Equal(t, assistance.Binding{OwnerID: 7, HandleID: "live-1", Protocol: "ssh"}, b)
		return "pwd", nil
	}))
	require.NoError(t, err)
	t.Cleanup(session.Close)
	result, err := session.Suggest(context.Background(), assistance.Input{Revision: 1})
	require.NoError(t, err)
	require.Equal(t, "pwd", result.Text)
	binder := service.binder.(fakeBinder)
	binder.snapshot.Generation++
	service.binder = binder
	_, err = session.Suggest(context.Background(), assistance.Input{Revision: 2})
	require.ErrorIs(t, err, assistance.ErrClosed)
	require.Equal(t, 1, calls)
}
