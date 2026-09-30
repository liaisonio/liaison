package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestAgentHistorySearchAcrossPagesAndScopes(t *testing.T) {
	d, err := dao.NewDao(&config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "search.db")}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, d.Close()) })
	cipher, err := newHistoryCipher("test-search-key")
	require.NoError(t, err)
	cp := &controlPlane{repo: &historyRepo{Repo: d}, historyCipher: cipher}
	ctx := context.Background()
	scope := model.EdgeAgentHistory{OwnerID: 1, AccessID: strings.Repeat("a", 32), EdgeID: 7}
	require.NoError(t, d.SaveAgentAccess(ctx, &model.AgentAccess{ID: scope.AccessID, OwnerID: 1, EdgeID: 7}, true))
	for i := 0; i < 120; i++ {
		row := scope
		row.SessionID = fmt.Sprintf("%032x", i+1)
		row.Revision = 1
		title := fmt.Sprintf("session %d", i)
		if i%2 == 0 {
			title = "Needle " + title
		}
		raw, err := json.Marshal(proto.EdgeAgentResult{SessionID: row.SessionID, Title: title, Project: "/项目/demo"})
		require.NoError(t, err)
		row.Payload, err = cp.sealHistory(raw, historyAAD(&row))
		require.NoError(t, err)
		require.NoError(t, d.SaveEdgeAgentHistory(ctx, &row))
	}
	rows, total, err := cp.searchAgentHistories(ctx, &scope, 1, " needle ")
	require.NoError(t, err)
	require.EqualValues(t, 60, total)
	require.Len(t, rows, 50)
	rows, total, err = cp.searchAgentHistories(ctx, &scope, 2, "NEEDLE")
	require.NoError(t, err)
	require.EqualValues(t, 60, total)
	require.Len(t, rows, 10)
	_, total, err = cp.searchAgentHistories(ctx, &scope, 1, "项目")
	require.NoError(t, err)
	require.EqualValues(t, 120, total)
	for _, foreign := range []model.EdgeAgentHistory{{OwnerID: 2, AccessID: scope.AccessID, EdgeID: 7}, {OwnerID: 1, AccessID: strings.Repeat("b", 32), EdgeID: 7}, {OwnerID: 1, AccessID: scope.AccessID, EdgeID: 8}} {
		rows, total, err := cp.searchAgentHistories(ctx, &foreign, 1, "needle")
		require.NoError(t, err)
		require.Empty(t, rows)
		require.Zero(t, total)
	}
	row := scope
	row.SessionID = fmt.Sprintf("%032x", 1)
	row.TitleOverride, err = cp.sealHistory([]byte("Renamed unique"), historyAAD(&row)+"/title")
	require.NoError(t, err)
	require.NoError(t, d.MutateEdgeAgentHistory(ctx, &row, false))
	_, total, err = cp.searchAgentHistories(ctx, &scope, 1, "renamed unique")
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.NoError(t, d.MutateEdgeAgentHistory(ctx, &row, true))
	_, total, err = cp.searchAgentHistories(ctx, &scope, 1, "renamed unique")
	require.NoError(t, err)
	require.Zero(t, total)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, _, err = cp.searchAgentHistories(cancelled, &scope, 1, "needle")
	require.ErrorIs(t, err, context.Canceled)
}
