package controlplane

import (
	"context"
	"errors"
	"testing"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func seedResourceScopeUsers(t *testing.T, r repo.Repo) (context.Context, context.Context, context.Context) {
	t.Helper()
	root := &model.Organization{Name: "Root", IsRoot: true}
	if err := r.CreateOrganization(root); err != nil {
		t.Fatalf("create root organization: %v", err)
	}
	adminRole := &model.IAMRole{Code: model.IAMRoleAdmin, Name: "Administrator", BuiltIn: true}
	userRole := &model.IAMRole{Code: model.IAMRoleUser, Name: "User", BuiltIn: true}
	if err := r.UpsertIAMRole(adminRole); err != nil {
		t.Fatal(err)
	}
	if err := r.UpsertIAMRole(userRole); err != nil {
		t.Fatal(err)
	}
	adminRole, _ = r.GetIAMRoleByCode(model.IAMRoleAdmin)
	userRole, _ = r.GetIAMRoleByCode(model.IAMRoleUser)
	users := []*model.User{
		{Name: "Admin", Email: "admin@example.test", Password: "unused", Status: model.UserStatusActive},
		{Name: "User one", Email: "one@example.test", Password: "unused", Status: model.UserStatusActive},
		{Name: "User two", Email: "two@example.test", Password: "unused", Status: model.UserStatusActive},
	}
	for _, user := range users {
		if err := r.CreateUser(user); err != nil {
			t.Fatal(err)
		}
		if err := r.UpsertOrganizationMembership(&model.OrganizationMembership{OrganizationID: root.ID, UserID: user.ID}); err != nil {
			t.Fatal(err)
		}
	}
	bindings := []*model.IAMRoleBinding{
		{UserID: users[0].ID, RoleID: adminRole.ID, OrganizationID: root.ID, InheritChildren: true},
		{UserID: users[1].ID, RoleID: userRole.ID, OrganizationID: root.ID},
		{UserID: users[2].ID, RoleID: userRole.ID, OrganizationID: root.ID},
	}
	for _, binding := range bindings {
		if err := r.UpsertIAMRoleBinding(binding); err != nil {
			t.Fatal(err)
		}
	}
	return context.WithValue(context.Background(), "user_id", users[0].ID),
		context.WithValue(context.Background(), "user_id", users[1].ID),
		context.WithValue(context.Background(), "user_id", users[2].ID)
}

func TestResourceOwnershipScopesConnectorCRUD(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	adminCtx, userOneCtx, userTwoCtx := seedResourceScopeUsers(t, r)

	one, err := cp.CreateEdge(userOneCtx, &v1.CreateEdgeRequest{Name: "one"})
	if err != nil || one == nil {
		t.Fatalf("create user one connector: %v", err)
	}
	two, err := cp.CreateEdge(userTwoCtx, &v1.CreateEdgeRequest{Name: "two"})
	if err != nil || two == nil {
		t.Fatalf("create user two connector: %v", err)
	}

	userOneList, err := cp.ListEdges(userOneCtx, &v1.ListEdgesRequest{Page: -1, PageSize: -1})
	if err != nil {
		t.Fatal(err)
	}
	if userOneList.Data.Total != 1 || userOneList.Data.Edges[0].Name != "one" {
		t.Fatalf("user one connectors = %+v, want only one", userOneList.Data.Edges)
	}
	userTwoList, err := cp.ListEdges(userTwoCtx, &v1.ListEdgesRequest{Page: -1, PageSize: -1})
	if err != nil {
		t.Fatal(err)
	}
	if userTwoList.Data.Total != 1 || userTwoList.Data.Edges[0].Name != "two" {
		t.Fatalf("user two connectors = %+v, want only two", userTwoList.Data.Edges)
	}
	adminList, err := cp.ListEdges(adminCtx, &v1.ListEdgesRequest{Page: -1, PageSize: -1})
	if err != nil {
		t.Fatal(err)
	}
	if adminList.Data.Total != 2 {
		t.Fatalf("admin connector total = %d, want 2", adminList.Data.Total)
	}

	oneID := uint64(adminList.Data.Edges[1].Id)
	if adminList.Data.Edges[0].Name == "one" {
		oneID = adminList.Data.Edges[0].Id
	}
	_, err = cp.GetEdge(userTwoCtx, &v1.GetEdgeRequest{Id: oneID})
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) || httpErr.Status() != 404 {
		t.Fatalf("cross-user GetEdge error = %v, want 404", err)
	}
	if _, err := cp.GetEdge(adminCtx, &v1.GetEdgeRequest{Id: oneID}); err != nil {
		t.Fatalf("admin GetEdge: %v", err)
	}
}

func TestEmptyResourceScopeReturnsNoRows(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, _, userTwoCtx := seedResourceScopeUsers(t, r)
	if err := r.CreateEdge(&model.Edge{Name: "unowned", Status: model.EdgeStatusRunning}); err != nil {
		t.Fatal(err)
	}
	result, err := cp.ListEdges(userTwoCtx, &v1.ListEdgesRequest{Page: -1, PageSize: -1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.Total != 0 || len(result.Data.Edges) != 0 {
		t.Fatalf("empty scope leaked connectors: %+v", result.Data.Edges)
	}
}
