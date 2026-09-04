package iam

import (
	"errors"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func TestCasbinControlsPlatformAndOrganizationRoles(t *testing.T) {
	service, r, admin := newOrganizationTestService(t)
	root, err := r.GetOrganizationByName(model.DefaultOrganizationName)
	if err != nil {
		t.Fatal(err)
	}

	member, _, err := service.CreateUserFor(admin, root.ID, "Member", "member@example.com", "password123", model.IAMRoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.ListUsersFor(member); !errors.Is(err, ErrForbidden) {
		t.Fatalf("member ListUsersFor error = %v, want ErrForbidden", err)
	}
	if _, err := service.ListOrganizationsFor(member); err != nil {
		t.Fatalf("authenticated organization visibility denied: %v", err)
	}

	if _, err := service.UpsertOrganizationMemberFor(admin, root.ID, member.ID, model.OrganizationRoleMember); err != nil {
		t.Fatal(err)
	}
	if allowed, err := service.CanManageOrganization(member, root.ID); err != nil || allowed {
		t.Fatalf("member CanManageOrganization = %v, %v; want false, nil", allowed, err)
	}

	if _, err := service.UpsertOrganizationMemberFor(admin, root.ID, member.ID, model.OrganizationRoleAdmin); err != nil {
		t.Fatal(err)
	}
	if allowed, err := service.CanManageOrganization(member, root.ID); err != nil || !allowed {
		t.Fatalf("org admin CanManageOrganization = %v, %v; want true, nil", allowed, err)
	}
	if _, err := service.CreateOrganizationFor(member, "Member child", "", &root.ID); err != nil {
		t.Fatalf("org admin create child: %v", err)
	}

	if _, err := service.UpsertOrganizationMemberFor(admin, root.ID, member.ID, model.IAMRoleUser); err != nil {
		t.Fatal(err)
	}
}

func TestCasbinControlsDefaultOrganizationResources(t *testing.T) {
	service, _, admin := newOrganizationTestService(t)
	root, err := service.repo.GetOrganizationByName(model.DefaultOrganizationName)
	if err != nil || root == nil {
		t.Fatalf("default organization = %#v, %v", root, err)
	}
	member, _, err := service.CreateUserFor(admin, root.ID, "Member", "resource-member@example.com", "password123", model.IAMRoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpsertOrganizationMemberFor(admin, root.ID, member.ID, model.OrganizationRoleMember); err != nil {
		t.Fatal(err)
	}

	for _, resource := range []string{"connectors", "devices", "applications", "accesses", "logs", "overview"} {
		if err := service.RequireResourcePermission(member, resource, "read"); err != nil {
			t.Fatalf("member read %s: %v", resource, err)
		}
	}
	if err := service.RequireResourcePermission(member, "accesses", "create"); err != nil {
		t.Fatalf("member create access: %v", err)
	}
	if err := service.RequireResourcePermission(member, "accesses", "use"); err != nil {
		t.Fatalf("member use access: %v", err)
	}
	for _, resource := range []string{"connectors", "devices", "applications", "accesses"} {
		for _, action := range []string{"create", "read", "update", "delete", "use"} {
			if err := service.RequireResourcePermission(member, resource, action); err != nil {
				t.Fatalf("user %s %s: %v", action, resource, err)
			}
		}
	}
}

func TestCasbinAssignmentsRebuildFromBusinessData(t *testing.T) {
	service, r, admin := newOrganizationTestService(t)
	root, err := r.GetOrganizationByName(model.DefaultOrganizationName)
	if err != nil {
		t.Fatal(err)
	}
	member, _, err := service.CreateUserFor(admin, root.ID, "Member", "restart-member@example.com", "password123", model.IAMRoleUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpsertOrganizationMemberFor(admin, root.ID, member.ID, model.OrganizationRoleAdmin); err != nil {
		t.Fatal(err)
	}

	restarted, err := NewIAMService(r)
	if err != nil {
		t.Fatal(err)
	}
	if allowed, err := restarted.CanManageOrganization(member, root.ID); err != nil || !allowed {
		t.Fatalf("rebuilt CanManageOrganization = %v, %v; want true, nil", allowed, err)
	}
}

func TestCasbinSystemRoleChangeTakesEffectImmediately(t *testing.T) {
	service, _, admin := newOrganizationTestService(t)
	root, err := service.repo.GetOrganizationByName(model.DefaultOrganizationName)
	if err != nil {
		t.Fatal(err)
	}
	secondAdmin, _, err := service.CreateUserFor(admin, root.ID, "Second admin", "second-admin@example.com", "password123", model.IAMRoleAdmin)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.ListUsersFor(secondAdmin); err != nil {
		t.Fatalf("system administrator list users: %v", err)
	}

	demote := model.IAMRoleUser
	if _, err := service.UpdateUserFor(admin, secondAdmin.ID, secondAdmin.Name, secondAdmin.Status, &demote); err != nil {
		t.Fatal(err)
	}
	if _, _, err := service.ListUsersFor(secondAdmin); !errors.Is(err, ErrForbidden) {
		t.Fatalf("demoted administrator ListUsersFor error = %v, want ErrForbidden", err)
	}
}
