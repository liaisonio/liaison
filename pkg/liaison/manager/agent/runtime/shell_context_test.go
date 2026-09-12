package runtime

import (
	"context"
	"fmt"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/require"
)

func TestShellCompletionContext_SharesCompletedConclusionsWithoutPersistingDrafts(t *testing.T) {
	ctx := context.Background()
	store := NewMemoryStore()
	require.NoError(t, store.CreateSession(ctx, Session{ID: "shell", CreatedBy: 7, OrganizationID: 1, Kind: tool.SessionShell}))
	for i, state := range []TurnStatus{TurnCompleted, TurnFailed} {
		_, turn, err := store.StartTurn(ctx, "shell", Turn{ID: fmt.Sprint("turn", i)})
		require.NoError(t, err)
		for j, msg := range []ModelMessage{{Role: RoleTool, Content: "RAW_OUTPUT"}, {Role: RoleAssistant, Content: fmt.Sprint("conclusion", i)}} {
			_, err = store.AppendMessage(ctx, Message{ID: fmt.Sprintf("m%d%d", i, j), AgentSessionID: "shell", TurnID: turn.ID, Value: msg})
			require.NoError(t, err)
		}
		turn, err = store.TransitionTurn(ctx, turn.ID, turn.Version, TurnRunning, "", "")
		require.NoError(t, err)
		_, err = store.TransitionTurn(ctx, turn.ID, turn.Version, state, "", "")
		require.NoError(t, err)
	}
	loop := &Loop{store: store}
	messages, err := loop.ShellCompletionContext(ctx, tool.Principal{UserID: 7, OrganizationID: 1}, "shell")
	require.NoError(t, err)
	require.Contains(t, messages[0].Content, "terminal-local Shell Agent")
	require.Contains(t, messages[1].Content, "conclusion0")
	require.NotContains(t, messages[1].Content, "conclusion1")
	require.NotContains(t, messages[1].Content, "RAW_OUTPUT")
	turns, err := store.ListTurns(ctx, "shell")
	require.NoError(t, err)
	require.Len(t, turns, 2)
	_, err = loop.ShellCompletionContext(ctx, tool.Principal{UserID: 8, OrganizationID: 1}, "shell")
	require.ErrorIs(t, err, ErrSessionNotFound)
}
