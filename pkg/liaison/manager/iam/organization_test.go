package iam

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func newOrganizationTestService(t *testing.T) (*IAMService, repo.Repo, *model.User) {
	t.Helper()
	r, err := repo.NewRepo(&config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "iam.db")}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	admin := &model.User{Name: "Admin", Email: "admin@example.com", Password: "unused", Status: model.UserStatusActive}
	if err := r.CreateUser(admin); err != nil {
		t.Fatal(err)
	}
	service, err := NewIAMService(r)
	if err != nil {
		t.Fatal(err)
	}
	if err := service.EnsureOrganizationBootstrap(); err != nil {
		t.Fatal(err)
	}
	admin, err = r.GetUserByID(admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	return service, r, admin
}

func TestOrganizationBootstrapCreatesRootAndMembership(t *testing.T) {
	service, r, admin := newOrganizationTestService(t)
	if role, err := service.UserRole(admin.ID); err != nil || role != model.IAMRoleAdmin {
		t.Fatalf("first user role = %q, %v; want admin", role, err)
	}
	root, err := r.GetOrganizationByName(model.DefaultOrganizationName)
	if err != nil {
		t.Fatal(err)
	}
	if root == nil || root.ParentID != nil {
		t.Fatalf("unexpected root: %#v", root)
	}
	membership, err := r.GetOrganizationMembership(root.ID, admin.ID)
	if err != nil {
		t.Fatal(err)
	}
	if membership == nil {
		t.Fatalf("unexpected membership: %#v", membership)
	}
	binding, err := r.GetIAMRoleBinding(root.ID, admin.ID)
	if err != nil || binding == nil || binding.Role == nil || binding.Role.Code != model.IAMRoleAdmin {
		t.Fatalf("unexpected role binding: %#v, %v", binding, err)
	}
	if err := service.EnsureOrganizationBootstrap(); err != nil {
		t.Fatalf("bootstrap is not idempotent: %v", err)
	}
}

func TestOrganizationHierarchyRejectsCycleAndLastAdminRemoval(t *testing.T) {
	service, _, admin := newOrganizationTestService(t)
	parent, err := service.CreateOrganizationFor(admin, "Engineering", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.CreateOrganizationFor(admin, "Platform", "", &parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateOrganizationFor(admin, parent.ID, parent.Name, "", &child.ID, true); !errors.Is(err, ErrInvalid) {
		t.Fatalf("cycle update error = %v, want ErrInvalid", err)
	}
	if err := service.RemoveOrganizationMemberFor(admin, child.ID, admin.ID); !errors.Is(err, ErrInvalid) {
		t.Fatalf("last admin removal error = %v, want ErrInvalid", err)
	}
	if _, err := service.UpsertOrganizationMemberFor(admin, child.ID, admin.ID, model.OrganizationRoleMember); !errors.Is(err, ErrInvalid) {
		t.Fatalf("last admin downgrade error = %v, want ErrInvalid", err)
	}
}
