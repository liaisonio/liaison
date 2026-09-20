package dao

import (
	"context"
	"encoding/json"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEdgeUninstallTaskMigrationAndIsolation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	up, err := os.ReadFile("../../../../db/migrations/20260919020000_create_edge_uninstall_tasks.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.AutoMigrate(&model.EdgeUninstallTask{}, &model.Edge{}, &model.AccessKey{}))
	d := &dao{db: db}
	ctx := context.Background()
	edge := uint64(7)
	for _, id := range []uint{7, 8} {
		require.NoError(t, db.Create(&model.Edge{Model: gorm.Model{ID: id}, Status: model.EdgeStatusRunning, Online: model.EdgeOnlineStatusOnline}).Error)
		require.NoError(t, db.Create(&model.AccessKey{EdgeID: id, AccessKey: "key" + string(rune(id))}).Error)
	}
	task := &model.EdgeUninstallTask{ID: "first", EdgeID: edge, ActiveEdge: &edge, Status: "accepted", TokenHash: "never-return-this", ExpiresAt: time.Now().Add(time.Minute)}
	created, err := d.CreateEdgeUninstallTask(ctx, task)
	require.NoError(t, err)
	require.True(t, created)
	created, err = d.CreateEdgeUninstallTask(ctx, &model.EdgeUninstallTask{ID: "second", EdgeID: edge, ActiveEdge: &edge, Status: "accepted"})
	require.NoError(t, err)
	require.False(t, created)
	_, err = d.GetEdgeUninstallTask(ctx, 8, "first")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	data, err := json.Marshal(task)
	require.NoError(t, err)
	require.NotContains(t, string(data), task.TokenHash)
	require.NoError(t, d.SetEdgeUninstallResult(ctx, "first", "unknown", "delivery_unconfirmed"))
	active, err := d.GetEdgeUninstallTask(ctx, edge, "")
	require.NoError(t, err)
	require.Equal(t, "unknown", active.Status)
	require.NoError(t, d.SetEdgeUninstallResult(ctx, "first", "running", ""))
	require.NoError(t, d.SetEdgeUninstallResult(ctx, "first", "completed", ""))
	require.NoError(t, d.SetEdgeUninstallResult(ctx, "first", "completed", ""))
	require.Error(t, d.SetEdgeUninstallResult(ctx, "first", "running", ""))
	_, err = d.GetEdgeUninstallTask(ctx, edge, "")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	var count int64
	require.NoError(t, db.Model(&model.AccessKey{}).Where("edge_id = ?", 7).Count(&count).Error)
	require.Zero(t, count)
	require.NoError(t, db.Model(&model.AccessKey{}).Where("edge_id = ?", 8).Count(&count).Error)
	require.EqualValues(t, 1, count)
	var other model.Edge
	require.NoError(t, db.First(&other, 8).Error)
	require.Equal(t, model.EdgeStatusRunning, other.Status)
	down, err := os.ReadFile("../../../../db/migrations/20260919020000_create_edge_uninstall_tasks.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(down)).Error)
	require.False(t, db.Migrator().HasTable(&model.EdgeUninstallTask{}))
}
