package application

import (
	"context"
	"errors"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestRunTurn_DisconnectedBindingDoesNotCallModel(t *testing.T) {
	for _, changed := range []bool{false, true} {
		store := runtime.NewMemoryStore()
		runner := &fakeTurnRunner{}
		binder := fakeBinder{err: errors.New("closed handle")}
		if changed {
			binder = fakeBinder{snapshot: tool.AttachmentSnapshot{Generation: 2}}
		}
		service, err := NewService(store, binder, fakeRelations{}, &fakeAuthorizer{}, fixedIDs{}, WithTurnRunner(runner))
		require.NoError(t, err)
		_, _, err = store.CreateSessionWithAttachment(context.Background(), runtime.Session{ID: "session", CreatedBy: 9, OrganizationID: 7, Status: runtime.SessionActive}, runtime.Attachment{ID: "handle", AgentSessionID: "session", AccessID: 1, ApplicationID: 1, Protocol: tool.ProtocolOracle, Capabilities: []tool.Capability{"data.schema"}, Generation: 1, State: runtime.AttachmentConnected})
		require.NoError(t, err)
		_, err = service.RunTurn(context.Background(), RunTurnRequest{Actor: &model.User{Model: gorm.Model{ID: 9}}, SessionID: "session", Prompt: "hello"})
		require.ErrorIs(t, err, ErrConnectionUnavailable)
		require.Empty(t, runner.request.SessionID)
		_, err = service.ResolveApproval(context.Background(), ResolveApprovalRequest{Actor: &model.User{Model: gorm.Model{ID: 9}}, SessionID: "session", ApprovalID: "approval", Approve: true})
		require.ErrorIs(t, err, ErrConnectionUnavailable)
		require.Empty(t, runner.request.SessionID)
	}
}
