package runtime

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSessionModelPersistenceAndConcurrency(t *testing.T) {
	repository := newAgentTestDAO(t)
	durable, err := NewDurableStore(repository)
	require.NoError(t, err)
	for name, store := range map[string]Store{"memory": NewMemoryStore(), "durable": durable} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			require.NoError(t, store.CreateSession(ctx, Session{ID: "model-choice", Kind: tool.SessionManagement, OrganizationID: 1, CreatedBy: 2}))
			updater := store.(interface {
				SetSessionModel(context.Context, string, uint64, ModelSelection) (Session, error)
			})
			selection := ModelSelection{ProviderID: "one", Model: "chosen"}
			saved, err := updater.SetSessionModel(ctx, "model-choice", 1, selection)
			require.NoError(t, err)
			loaded, err := store.GetSession(ctx, saved.ID)
			require.NoError(t, err)
			require.Equal(t, selection, loaded.ModelSelection)
			if name == "durable" {
				reopened, e := NewDurableStore(repository)
				require.NoError(t, e)
				record, e := reopened.GetSession(ctx, saved.ID)
				require.NoError(t, e)
				require.Equal(t, selection, record.ModelSelection)
			}
			_, err = updater.SetSessionModel(ctx, saved.ID, 1, ModelSelection{})
			require.ErrorIs(t, err, ErrVersionConflict)
			cleared, err := updater.SetSessionModel(ctx, saved.ID, saved.Version, ModelSelection{})
			require.NoError(t, err)
			require.Empty(t, cleared.ModelSelection)
			active, _, err := store.StartTurn(ctx, saved.ID, Turn{ID: "model-turn"})
			require.NoError(t, err)
			_, err = updater.SetSessionModel(ctx, saved.ID, active.Version, selection)
			require.ErrorIs(t, err, ErrTurnAlreadyActive)
		})
	}
}
