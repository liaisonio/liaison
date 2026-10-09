package dao

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestWebIDEAccessFilterBeforePagination(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "filter.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	require.NoError(t, db.AutoMigrate(&model.WebIDEAccess{}))
	d := &dao{db: db}
	for i, name := range []string{"Project Alpha", "PROJECT beta", "project 100%_done", "其他项目", "project hidden"} {
		owner := uint(1)
		if i == 4 {
			owner = 2
		}
		require.NoError(t, db.Create(&model.WebIDEAccess{ID: fmt.Sprintf("%032d", i), OwnerID: owner, Name: name}).Error)
	}
	for _, tc := range []struct {
		name       string
		page, size int
		total      int64
		length     int
	}{{"project", 1, 2, 3, 2}, {"project", 2, 2, 3, 1}, {"%_", 1, 20, 1, 1}, {"项目", 1, 20, 1, 1}, {"' OR 1=1 --", 1, 20, 0, 0}} {
		t.Run(tc.name, func(t *testing.T) {
			rows, total, err := d.WebIDEAccesses(t.Context(), 1, tc.page, tc.size, tc.name)
			require.NoError(t, err)
			require.Equal(t, tc.total, total)
			require.Len(t, rows, tc.length)
			for _, row := range rows {
				require.Equal(t, uint(1), row.OwnerID)
			}
		})
	}
}

func TestWebIDEAccessOwnershipAndApplicationBinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ide.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	migration, err := os.ReadFile("../../../../db/migrations/20261002010000_create_webide.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(migration)).Error)
	require.NoError(t, db.AutoMigrate(&model.WebIDEApplication{}, &model.WebIDEAccess{}))
	d := &dao{db: db}
	app := &model.WebIDEApplication{ID: strings.Repeat("a", 32), OwnerID: 1, EdgeID: 7, InstallationID: "installation", Name: "IDE", Mode: "managed"}
	require.NoError(t, d.SaveWebIDEApplication(t.Context(), app, true))
	access := &model.WebIDEAccess{ID: strings.Repeat("b", 32), OwnerID: 2, ApplicationID: app.ID, Name: "Project", Enabled: true}
	require.ErrorIs(t, d.SaveWebIDEAccess(t.Context(), access, true), gorm.ErrRecordNotFound)
	access.OwnerID = 1
	require.NoError(t, d.SaveWebIDEAccess(t.Context(), access, true))
	require.ErrorIs(t, d.DeleteWebIDEApplication(t.Context(), 1, app.ID), ErrWebIDEInUse)
	_, err = d.GetWebIDEApplication(t.Context(), 2, app.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	_, err = d.GetWebIDEAccess(t.Context(), 2, access.ID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, d.DeleteWebIDEAccess(t.Context(), 2, access.ID), gorm.ErrRecordNotFound)
	rows, total, err := d.WebIDEAccesses(t.Context(), 2, 1, 20)
	require.NoError(t, err)
	require.Empty(t, rows)
	require.Zero(t, total)
	access.Enabled = false
	require.NoError(t, d.SaveWebIDEAccess(t.Context(), access, false))
	saved, err := d.GetWebIDEAccess(t.Context(), 1, access.ID)
	require.NoError(t, err)
	require.False(t, saved.Enabled)
	require.NoError(t, d.DeleteWebIDEAccess(t.Context(), 1, access.ID))
	require.NoError(t, d.DeleteWebIDEApplication(t.Context(), 1, app.ID))
}
