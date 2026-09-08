package application

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"testing"
)

type referenceResolver struct {
	denied bool
	userID uint
}

func (r *referenceResolver) ResolveAgentResource(_ context.Context, userID uint, _ string, _ uint64) (string, error) {
	r.userID = userID
	if r.denied {
		return "", errors.New("private detail must not leak")
	}
	return "server name", nil
}

func TestRunTurnReferences(t *testing.T) {
	for _, tc := range []struct {
		name   string
		kind   tool.SessionKind
		refs   []runtime.ResourceReference
		denied bool
		want   error
	}{
		{name: "canonical names", kind: tool.SessionManagement, refs: []runtime.ResourceReference{{Type: "device", ID: "7", Name: "forged name"}}},
		{name: "deduplicate", kind: tool.SessionManagement, refs: []runtime.ResourceReference{{Type: "device", ID: "7"}, {Type: "device", ID: "7"}}},
		{name: "inaccessible or deleted", kind: tool.SessionManagement, refs: []runtime.ResourceReference{{Type: "application", ID: "7"}}, denied: true, want: ErrNotFound},
		{name: "unknown type", kind: tool.SessionManagement, refs: []runtime.ResourceReference{{Type: "user", ID: "7"}}, want: ErrInvalid},
		{name: "noncanonical id", kind: tool.SessionManagement, refs: []runtime.ResourceReference{{Type: "device", ID: "07"}}, want: ErrInvalid},
		{name: "zero id", kind: tool.SessionManagement, refs: []runtime.ResourceReference{{Type: "device", ID: "0"}}, want: ErrInvalid},
		{name: "too many", kind: tool.SessionManagement, refs: make([]runtime.ResourceReference, 9), want: ErrInvalid},
		{name: "access session", kind: tool.SessionAccess, refs: []runtime.ResourceReference{{Type: "device", ID: "7"}}, want: ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			store := runtime.NewMemoryStore()
			runner := &fakeTurnRunner{}
			resolver := &referenceResolver{denied: tc.denied}
			require.NoError(t, store.CreateSession(ctx, runtime.Session{ID: "refs", CreatedBy: 9, OrganizationID: 7, Kind: tc.kind, Status: runtime.SessionActive}))
			service, err := NewService(store, fakeBinder{}, fakeRelations{}, &fakeAuthorizer{}, fixedIDs{}, WithTurnRunner(runner), WithResourceReferences(resolver))
			require.NoError(t, err)
			_, err = service.RunTurn(ctx, RunTurnRequest{Actor: &model.User{Model: gorm.Model{ID: 9}}, SessionID: "refs", Prompt: "inspect", References: tc.refs})
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
				require.Empty(t, runner.request.SessionID)
				require.NotContains(t, err.Error(), "private detail")
				return
			}
			require.NoError(t, err)
			require.Equal(t, uint(9), resolver.userID)
			require.Equal(t, []runtime.ResourceReference{{Type: "device", ID: "7", Name: "server name"}}, runner.request.References)
		})
	}
}
