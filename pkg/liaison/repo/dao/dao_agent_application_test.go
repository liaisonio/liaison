package dao

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAgentApplicationMigrationAndBinding(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "applications.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, sqlDB.Close()) })
	run := func(name string) {
		t.Helper()
		script, err := os.ReadFile("../../../../db/migrations/" + name)
		require.NoError(t, err)
		require.NoError(t, db.Exec(string(script)).Error)
	}
	run("20260921010000_create_agent_accesses.up.sql")
	stamp := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	require.NoError(t, db.Exec("INSERT INTO agent_accesses (id,owner_id,name,kind,edge_id,installation_id,project,updated_at) VALUES (?,?,?,?,?,?,?,?)", "old", 1, "Existing", "codex", 7, "install", "/project", stamp).Error)
	run("20260926010000_create_agent_applications.up.sql")
	for _, entity := range []any{&model.AgentApplication{}, &model.AgentAccess{}, &model.EdgeAgentHistory{}, &model.EdgeAgentHistoryPage{}} {
		require.NoError(t, db.AutoMigrate(entity), "%T", entity)
	}
	d := &dao{db: db}
	ctx := context.Background()
	original, err := d.GetAgentAccess(ctx, 1, "old")
	require.NoError(t, err)
	require.NotEmpty(t, original.ApplicationID)
	// Simulate an old client/binary creating a row without an application reference.
	legacy := *original
	legacy.ID = "legacy"
	legacy.ApplicationID = ""
	require.NoError(t, db.Create(&legacy).Error)
	history := model.EdgeAgentHistory{OwnerID: 1, AccessID: "legacy", EdgeID: 7, SessionID: "native-session", Payload: []byte("encrypted-history"), Revision: 1}
	require.NoError(t, db.Create(&history).Error)
	require.NoError(t, d.backfillAgentApplications())
	require.NoError(t, d.backfillAgentApplications())
	linked, err := d.GetAgentAccess(ctx, 1, "legacy")
	require.NoError(t, err)
	require.Equal(t, original.ApplicationID, linked.ApplicationID)
	var preserved model.EdgeAgentHistory
	require.NoError(t, db.Where("session_id = ?", "native-session").First(&preserved).Error)
	require.Equal(t, history.Payload, preserved.Payload)
	again, err := d.GetAgentAccess(ctx, 1, "old")
	require.NoError(t, err)
	require.Equal(t, original.ApplicationID, again.ApplicationID)
	require.True(t, again.UpdatedAt.Equal(stamp))
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, id := range []string{"second", "third"} {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			errs <- d.SaveAgentAccess(ctx, &model.AgentAccess{ID: id, OwnerID: 1, EdgeID: 7, Kind: "codex", InstallationID: "install", Project: "/other", Name: id}, true)
		}(id)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	var count int64
	require.NoError(t, db.Model(&model.AgentApplication{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	for _, id := range []string{"second", "third"} {
		row, err := d.GetAgentAccess(ctx, 1, id)
		require.NoError(t, err)
		require.Equal(t, original.ApplicationID, row.ApplicationID)
	}
	for _, change := range []func(*model.AgentAccess){func(r *model.AgentAccess) { r.OwnerID = 2 }, func(r *model.AgentAccess) { r.EdgeID = 8 }, func(r *model.AgentAccess) { r.InstallationID = "other" }} {
		row := *original
		row.ID = "invalid"
		change(&row)
		require.ErrorIs(t, d.SaveAgentAccess(ctx, &row, true), gorm.ErrRecordNotFound)
	}
	// A failed access insert must roll back the registration, too.
	failed := *original
	failed.ApplicationID = ""
	failed.InstallationID = "new"
	require.Error(t, d.SaveAgentAccess(ctx, &failed, true))
	require.NoError(t, db.Model(&model.AgentApplication{}).Count(&count).Error)
	require.EqualValues(t, 1, count)
	_, err = d.GetAgentApplication(ctx, 2, original.ApplicationID)
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	require.ErrorIs(t, d.DeleteAgentApplication(ctx, 1, original.ApplicationID), ErrAgentApplicationInUse)
	require.ErrorIs(t, d.DeleteAgentApplication(ctx, 2, original.ApplicationID), gorm.ErrRecordNotFound)
	registered := &model.AgentApplication{ID: "unused", OwnerID: 1, EdgeID: 7, Kind: "codex", InstallationID: "unused-install", Name: "Standalone"}
	require.NoError(t, d.SaveAgentApplication(ctx, registered, true))
	registered.Name = "Renamed"
	require.NoError(t, d.SaveAgentApplication(ctx, registered, false))
	apps, total, err := d.ListAgentApplications(ctx, 1, 7, 1, 100)
	require.NoError(t, err)
	require.EqualValues(t, 2, total)
	require.Len(t, apps, 2)
	for _, app := range apps {
		if app.ID == original.ApplicationID {
			require.EqualValues(t, 4, app.AccessCount)
		}
	}
	require.NoError(t, d.DeleteAgentApplication(ctx, 1, registered.ID))
	// Tampered linkage fails closed on the runtime's access lookup.
	require.NoError(t, db.Model(&model.AgentAccess{}).Where("id = ?", "second").UpdateColumn("installation_id", "tampered").Error)
	_, err = d.GetAgentAccess(ctx, 1, "second")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	// Removing an access leaves the reusable application and other accesses intact.
	require.NoError(t, d.DeleteAgentAccess(ctx, 1, "old"))
	_, err = d.GetAgentApplication(ctx, 1, original.ApplicationID)
	require.NoError(t, err)
	_, err = d.GetAgentAccess(ctx, 1, "third")
	require.NoError(t, err)
	run("20260926010000_create_agent_applications.down.sql")
	require.False(t, db.Migrator().HasTable(&model.AgentApplication{}))
	require.False(t, db.Migrator().HasColumn(&model.AgentAccess{}, "application_id"))
	run("20260926010000_create_agent_applications.up.sql")
	require.NoError(t, d.backfillAgentApplications())
	_, err = d.GetAgentAccess(ctx, 1, "third")
	require.NoError(t, err)
}
