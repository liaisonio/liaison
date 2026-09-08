package application

import (
	"context"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

type managementRelations struct{ fakeRelations }

func (managementRelations) ListUserOrganizations(id uint) ([]*model.OrganizationMembership, error) {
	return []*model.OrganizationMembership{{UserID: id, OrganizationID: 9}, {UserID: id, OrganizationID: 2}, {UserID: 99, OrganizationID: 1}}, nil
}

func TestManagementSessionServerIdentityAndOwnership(t *testing.T) {
	store := runtime.NewMemoryStore()
	authorizer := &fakeAuthorizer{}
	s, err := NewService(store, fakeBinder{}, managementRelations{}, authorizer, nil)
	require.NoError(t, err)
	actor := &model.User{Status: model.UserStatusActive}
	actor.ID = 7
	request := CreateSessionRequest{Actor: actor, Kind: tool.SessionManagement}
	detail, err := s.CreateSession(context.Background(), request)
	require.NoError(t, err)
	require.Equal(t, tool.SessionManagement, detail.Session.Kind)
	require.Equal(t, uint(7), detail.Session.CreatedBy)
	require.Equal(t, uint(2), detail.Session.OrganizationID)
	require.Empty(t, detail.Attachments)
	require.Empty(t, detail.Session.ActiveAttachmentID)
	other := &model.User{Status: model.UserStatusActive}
	other.ID = 8
	_, err = s.GetSession(context.Background(), other, detail.Session.ID)
	require.ErrorIs(t, err, ErrNotFound)
	request.HandleID = "live"
	_, err = s.CreateSession(context.Background(), request)
	require.ErrorIs(t, err, ErrInvalid)
	request.HandleID = ""
	request.Kind = "administrator"
	_, err = s.CreateSession(context.Background(), request)
	require.ErrorIs(t, err, ErrInvalid)
	items, err := store.ListSessions(context.Background(), actor.ID)
	require.NoError(t, err)
	require.Len(t, items, 1)
}
