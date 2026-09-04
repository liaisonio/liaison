package controlplane

import (
	"context"
	"testing"
)

func TestManagementAudit_RecordAndListCurrentUser(t *testing.T) {
	cp, repo := newTestControlPlane(t)
	defer repo.Close()
	ctx := context.WithValue(context.Background(), "user_id", uint(1))
	if err := cp.RecordManagementAudit(ctx, &ManagementAudit{UserID: 1, UserEmail: "owner@example.test", Module: "application", Action: "create", Resource: "/api/v1/applications", Method: "POST", ClientIP: "203.0.113.5", Success: true, StatusCode: 200, ElapsedMS: 8}); err != nil {
		t.Fatalf("RecordManagementAudit() error = %v", err)
	}
	if err := cp.RecordManagementAudit(context.Background(), &ManagementAudit{UserID: 2, UserEmail: "other@example.test", Module: "application", Action: "delete", Resource: "/api/v1/applications/2", Method: "DELETE", Success: true, StatusCode: 200}); err != nil {
		t.Fatalf("RecordManagementAudit(other) error = %v", err)
	}
	result, err := cp.ListManagementAudits(ctx, &ManagementAuditListQuery{Module: "application", Page: 1, PageSize: 20})
	if err != nil {
		t.Fatalf("ListManagementAudits() error = %v", err)
	}
	if result.Total != 1 || len(result.Items) != 1 {
		t.Fatalf("ListManagementAudits() = %+v, want one current-user record", result)
	}
	item := result.Items[0]
	if item.Action != "create" || item.ClientIP != "203.0.113.5" || !item.Success {
		t.Fatalf("item = %+v, want recorded management operation", item)
	}
}
