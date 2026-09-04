package dao

import (
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestBackfillWebSSHAuditProtocolsOnlyMigratesIdentifiableBrowserRows(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.WebDataAudit{}); err != nil {
		t.Fatal(err)
	}
	rows := []*model.WebDataAudit{
		{UserID: 1, Protocol: "ssh", Details: `{"client_ip_source":"remote_addr"}`},
		{UserID: 1, Protocol: "ssh", Details: `{}`},
		{UserID: 0, Protocol: "ssh", Details: `{"client_ip_source":"remote_addr","auth_method":"password"}`},
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}

	d := &dao{db: db}
	if err := d.backfillWebSSHAuditProtocols(); err != nil {
		t.Fatal(err)
	}
	if err := d.backfillWebSSHAuditProtocols(); err != nil {
		t.Fatalf("idempotent backfill failed: %v", err)
	}

	var got []*model.WebDataAudit
	if err := db.Order("id ASC").Find(&got).Error; err != nil {
		t.Fatal(err)
	}
	want := []string{"webssh", "ssh", "ssh"}
	for i := range want {
		if got[i].Protocol != want[i] {
			t.Fatalf("row %d protocol = %q, want %q", i, got[i].Protocol, want[i])
		}
	}
}
