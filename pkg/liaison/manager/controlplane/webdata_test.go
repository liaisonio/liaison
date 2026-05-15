package controlplane

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func TestWebDataTargetRequiresDataApplication(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeSSH
	application.Port = 22
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "ssh-proxy",
		ApplicationID: application.ID,
		Port:          0,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	if _, err := cp.GetWebDataTarget(context.Background(), proxy.ID); err == nil || !strings.Contains(err.Error(), "WebData") {
		t.Fatalf("GetWebDataTarget error = %v, want WebData-only error", err)
	}
}

func TestWebDataTargetNormalizesDatabaseAlias(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeDatabase
	application.Port = 3306
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "db-proxy",
		ApplicationID: application.ID,
		Port:          0,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	target, err := cp.GetWebDataTarget(context.Background(), proxy.ID)
	if err != nil {
		t.Fatalf("GetWebDataTarget: %v", err)
	}
	if target.Protocol != "mysql" {
		t.Fatalf("protocol = %q, want mysql", target.Protocol)
	}
	if target.EffectiveStatus != proxyEffectiveStatusActive {
		t.Fatalf("effective status = %q, want active", target.EffectiveStatus)
	}
}

func TestWebDataCredentialsAreScopedByIdentity(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeMySQL
	application.Port = 3306
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "mysql-proxy",
		ApplicationID: application.ID,
		Port:          0,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	ctxUser1 := context.WithValue(context.Background(), "user_id", uint(1))
	ctxUser2 := context.WithValue(context.Background(), "user_id", uint(2))
	if err := cp.SaveWebDataCredential(ctxUser1, proxy.ID, "mysql", "root", "app", "", "enc-1", "nonce-1"); err != nil {
		t.Fatalf("save user1 credential: %v", err)
	}
	if err := cp.SaveWebDataCredential(ctxUser2, proxy.ID, "mysql", "root", "app", "", "enc-2", "nonce-2"); err != nil {
		t.Fatalf("save user2 credential: %v", err)
	}

	target, err := cp.GetWebDataTarget(ctxUser1, proxy.ID)
	if err != nil {
		t.Fatalf("GetWebDataTarget: %v", err)
	}
	if len(target.Credentials) != 1 || target.Credentials[0].Username != "root" || target.Credentials[0].Database != "app" {
		t.Fatalf("user1 credentials = %+v, want root/app", target.Credentials)
	}

	secret, err := cp.GetWebDataCredentialSecret(ctxUser1, proxy.ID, "database", "root", "app", "")
	if err != nil {
		t.Fatalf("GetWebDataCredentialSecret: %v", err)
	}
	if secret.EncryptedPassword != "enc-1" {
		t.Fatalf("secret password = %q, want enc-1", secret.EncryptedPassword)
	}
	if err := cp.DeleteWebDataCredential(ctxUser1, proxy.ID, "mysql", "root", "app", ""); err != nil {
		t.Fatalf("DeleteWebDataCredential: %v", err)
	}
	if _, err := cp.GetWebDataCredentialSecret(ctxUser1, proxy.ID, "mysql", "root", "app", ""); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("user1 secret after delete error = %v, want record not found", err)
	}
	if _, err := cp.GetWebDataCredentialSecret(ctxUser2, proxy.ID, "mysql", "root", "app", ""); err != nil {
		t.Fatalf("user2 credential should remain: %v", err)
	}
}

func TestWebDataCredentialProfileCanBeEditedAndLoadedByID(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeMySQL
	application.Port = 3306
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "mysql-proxy",
		ApplicationID: application.ID,
		Port:          0,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	ctx := context.WithValue(context.Background(), "user_id", uint(1))
	saved, err := cp.SaveWebDataCredentialProfile(ctx, proxy.ID, &WebDataCredentialProfile{
		Name:              "primary mysql",
		Protocol:          "mysql",
		Username:          "root",
		Database:          "app",
		TLSMode:           "require",
		EncryptedPassword: "enc-1",
		Nonce:             "nonce-1",
		PasswordChanged:   true,
	})
	if err != nil {
		t.Fatalf("SaveWebDataCredentialProfile: %v", err)
	}
	if saved.ID == 0 || saved.Name != "primary mysql" || saved.TLSMode != "require" {
		t.Fatalf("saved profile = %+v, want id/name/tls", saved)
	}

	if _, err := cp.SaveWebDataCredentialProfile(ctx, proxy.ID, &WebDataCredentialProfile{
		ID:              saved.ID,
		Name:            "renamed mysql",
		Protocol:        "mysql",
		Username:        "root",
		Database:        "app",
		TLSMode:         "disable",
		PasswordChanged: false,
	}); err != nil {
		t.Fatalf("update profile without password: %v", err)
	}
	secret, err := cp.GetWebDataCredentialSecretByID(ctx, proxy.ID, saved.ID)
	if err != nil {
		t.Fatalf("GetWebDataCredentialSecretByID: %v", err)
	}
	if secret.Name != "renamed mysql" || secret.TLSMode != "disable" || secret.EncryptedPassword != "enc-1" {
		t.Fatalf("secret after update = %+v, want renamed profile with original password", secret)
	}
	if err := cp.DeleteWebDataCredentialByID(ctx, proxy.ID, saved.ID); err != nil {
		t.Fatalf("DeleteWebDataCredentialByID: %v", err)
	}
	if _, err := cp.GetWebDataCredentialSecretByID(ctx, proxy.ID, saved.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("secret after delete error = %v, want record not found", err)
	}
}

func TestRecordWebDataAudit(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	if err := cp.RecordWebDataAudit(context.Background(), &WebDataAudit{
		UserID:           1,
		ProxyID:          2,
		ApplicationID:    3,
		Protocol:         "redis",
		StatementPreview: "FLUSHDB",
		StatementSHA256:  strings.Repeat("a", 64),
		Success:          false,
		Error:            "boom",
		ElapsedMS:        12,
		ClientIP:         "203.0.113.1",
	}); err != nil {
		t.Fatalf("RecordWebDataAudit: %v", err)
	}
}

func TestListWebDataAuditsScopesByProxyAndUser(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeMySQL
	application.Port = 3306
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "mysql-proxy",
		ApplicationID: application.ID,
		Port:          0,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	audits := []*WebDataAudit{
		{UserID: 1, ProxyID: proxy.ID, ApplicationID: application.ID, Protocol: "mysql", Database: "app", StatementPreview: "SELECT 1", StatementSHA256: strings.Repeat("a", 64), Success: true, AffectedRows: 0, ClientIP: "203.0.113.10", Details: map[string]any{"client_ip_source": "x-forwarded-for"}},
		{UserID: 2, ProxyID: proxy.ID, ApplicationID: application.ID, Protocol: "mysql", StatementPreview: "SELECT 2", StatementSHA256: strings.Repeat("b", 64), Success: true},
		{UserID: 1, ProxyID: proxy.ID + 1, ApplicationID: application.ID, Protocol: "mysql", StatementPreview: "SELECT 3", StatementSHA256: strings.Repeat("c", 64), Success: true},
	}
	for _, audit := range audits {
		if err := cp.RecordWebDataAudit(context.Background(), audit); err != nil {
			t.Fatalf("RecordWebDataAudit: %v", err)
		}
	}

	entries, err := cp.ListWebDataAudits(context.WithValue(context.Background(), "user_id", uint(1)), proxy.ID, 100)
	if err != nil {
		t.Fatalf("ListWebDataAudits: %v", err)
	}
	if len(entries) != 1 || entries[0].StatementPreview != "SELECT 1" {
		t.Fatalf("entries = %+v, want only user1/proxy audit", entries)
	}
	if entries[0].Details["database"] != "app" || entries[0].Details["affected_rows"] == nil || entries[0].Details["client_ip"] != "203.0.113.10" || entries[0].Details["client_ip_source"] != "x-forwarded-for" {
		t.Fatalf("details = %+v, want database/affected_rows/client_ip", entries[0].Details)
	}
}

func TestListAccessAuditsAllowsSSHProtocol(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeSSH
	application.Port = 22
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "ssh-proxy",
		ApplicationID: application.ID,
		Port:          0,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	if err := cp.RecordWebDataAudit(context.Background(), &WebDataAudit{
		UserID:           1,
		ProxyID:          proxy.ID,
		ApplicationID:    application.ID,
		Protocol:         "ssh",
		Action:           "execute",
		Database:         "root",
		StatementPreview: "uptime",
		StatementSHA256:  strings.Repeat("d", 64),
		Success:          true,
	}); err != nil {
		t.Fatalf("RecordWebDataAudit: %v", err)
	}

	result, err := cp.ListWebDataAuditEntries(context.WithValue(context.Background(), "user_id", uint(1)), &WebDataAuditListQuery{
		ProxyID:  proxy.ID,
		Protocol: "ssh",
		Page:     1,
		PageSize: 20,
	})
	if err != nil {
		t.Fatalf("ListWebDataAuditEntries: %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("result = %+v, want one ssh audit", result)
	}
	item := result.Items[0]
	if item.Protocol != "ssh" || item.StatementPreview != "uptime" || item.Details["ssh_user"] != "root" {
		t.Fatalf("item = %+v, want ssh uptime audit", item)
	}
}
