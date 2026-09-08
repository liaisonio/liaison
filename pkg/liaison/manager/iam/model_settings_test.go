package iam

import (
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"testing"
)

func TestModelSettingsRequiresRootAdmin(t *testing.T) {
	service, _, admin := newOrganizationTestService(t)
	org, err := service.CreateOrganizationFor(admin, "Engineering", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range []model.IAMRoleCode{model.IAMRoleAdmin, model.IAMRoleUser} {
		user, _, err := service.CreateUserFor(admin, org.ID, string(role), "child-"+string(role)+"@example.com", "password123", role)
		if err != nil {
			t.Fatal(err)
		}
		for _, action := range []string{"read", "update", "test"} {
			if err := service.RequireModelSettingsPermission(user, action); !errors.Is(err, ErrForbidden) {
				t.Fatalf("child %s allowed %s: %v", role, action, err)
			}
			if err := service.RequireModelSettingsPermission(admin, action); err != nil {
				t.Fatal(err)
			}
		}
	}
}
