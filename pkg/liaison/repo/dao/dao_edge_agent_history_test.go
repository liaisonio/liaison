package dao

import (
	"bytes"
	"context"
	"fmt"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestHistoryBatchLimitsCursorAndScope(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "batch.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.EdgeAgentHistoryPage{}))
	d := &dao{db: db}
	ctx := context.Background()
	scope := &model.EdgeAgentHistory{OwnerID: 1, AccessID: "access", EdgeID: 7, SessionID: "session"}
	for i := uint64(0); i < 45; i++ {
		require.NoError(t, db.Create(&model.EdgeAgentHistoryPage{OwnerID: 1, AccessID: "access", EdgeID: 7, SessionID: "session", Window: i, Payload: []byte("small")}).Error)
	}
	pages, err := d.ListEdgeAgentHistoryPages(ctx, scope, 45, 20, 2<<20)
	require.NoError(t, err)
	require.Len(t, pages, 20)
	require.EqualValues(t, 44, pages[0].Window)
	require.EqualValues(t, 25, pages[19].Window)
	pages, err = d.ListEdgeAgentHistoryPages(ctx, scope, 25, 20, 2<<20)
	require.NoError(t, err)
	require.Len(t, pages, 20)
	require.EqualValues(t, 24, pages[0].Window)
	pages, err = d.ListEdgeAgentHistoryPages(ctx, scope, 5, 20, 2<<20)
	require.NoError(t, err)
	require.Len(t, pages, 5)
	require.Zero(t, pages[4].Window)
	for _, foreign := range []model.EdgeAgentHistory{{OwnerID: 2, AccessID: "access", EdgeID: 7, SessionID: "session"}, {OwnerID: 1, AccessID: "other", EdgeID: 7, SessionID: "session"}, {OwnerID: 1, AccessID: "access", EdgeID: 8, SessionID: "session"}, {OwnerID: 1, AccessID: "access", EdgeID: 7, SessionID: "other"}} {
		pages, err = d.ListEdgeAgentHistoryPages(ctx, &foreign, 45, 20, 2<<20)
		require.NoError(t, err)
		require.Empty(t, pages)
	}
	require.NoError(t, db.Model(&model.EdgeAgentHistoryPage{}).Where("window >= ?", 40).Update("payload", bytes.Repeat([]byte("x"), 500<<10)).Error)
	pages, err = d.ListEdgeAgentHistoryPages(ctx, scope, 45, 20, (2<<20)-1024)
	require.NoError(t, err)
	require.Len(t, pages, 4)
	pages, err = d.ListEdgeAgentHistoryPages(ctx, scope, pages[3].Window, 20, (2<<20)-1024)
	require.NoError(t, err)
	require.EqualValues(t, 40, pages[0].Window)
	_, err = d.ListEdgeAgentHistoryPages(ctx, scope, 45, 21, 2<<20)
	require.Error(t, err)
}

func TestEdgeAgentHistoryDurabilityScopeAndTombstone(t *testing.T) {
	file := filepath.Join(t.TempDir(), "history.db")
	db, err := gorm.Open(sqlite.Open(file), &gorm.Config{})
	require.NoError(t, err)
	up, err := os.ReadFile("../../../../db/migrations/20260922110000_create_edge_agent_histories.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.AutoMigrate(&model.AgentApplication{}, &model.AgentAccess{}, &model.EdgeAgentHistory{}, &model.EdgeAgentHistoryPage{}))
	d := &dao{db: db}
	ctx := context.Background()
	require.NoError(t, d.SaveAgentAccess(ctx, &model.AgentAccess{ID: "access", OwnerID: 1, EdgeID: 7}, true))
	scope := model.EdgeAgentHistory{OwnerID: 1, AccessID: "access", EdgeID: 7, SessionID: "session", Revision: 2, Payload: []byte("synthetic ciphertext"), UpdatedAt: time.Now().Add(-48 * time.Hour)}
	require.NoError(t, d.SaveEdgeAgentHistory(ctx, &scope))
	page := model.EdgeAgentHistoryPage{OwnerID: 1, AccessID: "access", EdgeID: 7, SessionID: "session", Window: 0, Revision: 2, Payload: []byte("encrypted round")}
	require.NoError(t, d.SaveEdgeAgentHistoryPage(ctx, &page))
	page.Payload = []byte("must not overwrite")
	require.NoError(t, d.SaveEdgeAgentHistoryPage(ctx, &page))
	previous, err := d.PreviousEdgeAgentHistoryPage(ctx, &scope, 1)
	require.NoError(t, err)
	require.Equal(t, "encrypted round", string(previous.Payload))
	_, err = d.PreviousEdgeAgentHistoryPage(ctx, &scope, 0)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	stale := scope
	stale.Revision = 1
	stale.Payload = []byte("stale")
	require.NoError(t, d.SaveEdgeAgentHistory(ctx, &stale))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	db, err = gorm.Open(sqlite.Open(file), &gorm.Config{})
	require.NoError(t, err)
	d.db = db
	sqlDB, err = db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	saved, err := d.GetEdgeAgentHistory(ctx, &scope)
	require.NoError(t, err)
	require.Equal(t, scope.Payload, saved.Payload)
	for _, foreign := range []model.EdgeAgentHistory{{OwnerID: 2, AccessID: "access", EdgeID: 7, SessionID: "session"}, {OwnerID: 1, AccessID: "other", EdgeID: 7, SessionID: "session"}, {OwnerID: 1, AccessID: "access", EdgeID: 8, SessionID: "session"}} {
		_, err = d.GetEdgeAgentHistory(ctx, &foreign)
		require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		rows, total, err := d.ListEdgeAgentHistories(ctx, &foreign, 1)
		require.NoError(t, err)
		require.Empty(t, rows)
		require.Zero(t, total)
	}
	for i := 0; i < 51; i++ {
		r := scope
		r.SessionID = fmt.Sprint(i)
		require.NoError(t, d.SaveEdgeAgentHistory(ctx, &r))
	}
	rows, total, err := d.ListEdgeAgentHistories(ctx, &scope, 1)
	require.NoError(t, err)
	require.EqualValues(t, 52, total)
	require.Len(t, rows, 50)
	rows, _, err = d.ListEdgeAgentHistories(ctx, &scope, 2)
	require.NoError(t, err)
	require.Len(t, rows, 2)
	require.NoError(t, d.MutateEdgeAgentHistory(ctx, &scope, true))
	_, err = d.PreviousEdgeAgentHistoryPage(ctx, &scope, 1)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, d.SaveEdgeAgentHistoryPage(ctx, &page), gorm.ErrRecordNotFound)
	scope.Revision = 99
	require.NoError(t, d.SaveEdgeAgentHistory(ctx, &scope))
	saved, err = d.GetEdgeAgentHistory(ctx, &scope)
	require.NoError(t, err)
	require.True(t, saved.Deleted)
	require.Empty(t, saved.Payload)
	scope.TitleOverride = []byte("late rename")
	require.ErrorIs(t, d.MutateEdgeAgentHistory(ctx, &scope, false), gorm.ErrRecordNotFound)
	saved, err = d.GetEdgeAgentHistory(ctx, &scope)
	require.NoError(t, err)
	require.Empty(t, saved.TitleOverride)
	require.NoError(t, d.DeleteAgentAccess(ctx, 1, "access"))
	_, err = d.GetEdgeAgentHistory(ctx, &scope)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, d.SaveEdgeAgentHistory(ctx, &scope), gorm.ErrRecordNotFound)
	down, err := os.ReadFile("../../../../db/migrations/20260922110000_create_edge_agent_histories.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
}
