package dao

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentAccessMigrationAndOwnerIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "entries.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	up, err := os.ReadFile("../../../../db/migrations/20260921010000_create_agent_accesses.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.AutoMigrate(&model.AgentAccess{}, &model.EdgeAgentHistory{}, &model.EdgeAgentHistoryPage{}))
	d := &dao{db: db}
	ctx := context.Background()
	r := &model.AgentAccess{ID: "entry", OwnerID: 1, EdgeID: 7, Name: "Project", Kind: "codex", InstallationID: "installed", Project: "/project"}
	require.NoError(t, d.SaveAgentAccess(ctx, r, true))
	_, err = d.GetAgentAccess(ctx, 2, r.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	rows, total, err := d.ListAgentAccesses(ctx, 2, 1, 20)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, total)
	other := *r
	other.OwnerID = 2
	other.Name = "stolen"
	require.ErrorIs(t, d.SaveAgentAccess(ctx, &other, false), gorm.ErrRecordNotFound)
	require.ErrorIs(t, d.DeleteAgentAccess(ctx, 2, r.ID), gorm.ErrRecordNotFound)
	r.Name = "Renamed"
	require.NoError(t, d.SaveAgentAccess(ctx, r, false))
	for _, history := range []model.EdgeAgentHistory{
		{OwnerID: 1, AccessID: r.ID, EdgeID: 7, SessionID: "live"},
		{OwnerID: 1, AccessID: r.ID, EdgeID: 7, SessionID: "closed", Closed: true},
		{OwnerID: 1, AccessID: r.ID, EdgeID: 7, SessionID: "deleted", Deleted: true},
		{OwnerID: 2, AccessID: r.ID, EdgeID: 7, SessionID: "foreign"},
		{OwnerID: 1, AccessID: r.ID, EdgeID: 8, SessionID: "other-edge"},
	} {
		require.NoError(t, db.Create(&history).Error)
	}
	rows, total, err = d.ListAgentAccesses(ctx, 1, 1, 20)
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Equal(t, "Renamed", rows[0].Name)
	require.EqualValues(t, 2, rows[0].SessionCount)
	rows, total, err = d.ListAgentAccesses(ctx, 1, 1, 20, model.AgentAccessFilter{Name: "NAM", Kind: "codex"})
	require.NoError(t, err)
	require.EqualValues(t, 1, total)
	require.Len(t, rows, 1)
	for _, filter := range []model.AgentAccessFilter{{Name: "%"}, {Name: "_"}, {Name: "missing"}, {Kind: "other"}} {
		rows, total, err = d.ListAgentAccesses(ctx, 1, 1, 20, filter)
		require.NoError(t, err)
		require.Zero(t, total)
		require.Empty(t, rows)
	}
	require.NoError(t, d.DeleteAgentAccess(ctx, 1, r.ID))
	_, err = d.GetAgentAccess(ctx, 1, r.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	down, err := os.ReadFile("../../../../db/migrations/20260921010000_create_agent_accesses.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
}
