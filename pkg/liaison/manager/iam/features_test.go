package iam

import (
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestFeaturePolicy_GrantRevokeAndRestart(t *testing.T) {
	s, repo, admin := newOrganizationTestService(t)
	root, err := repo.GetRootOrganization()
	require.NoError(t, err)
	child, err := s.CreateOrganizationFor(admin, "Feature child", "", &root.ID)
	require.NoError(t, err)
	user, _, err := s.CreateUserFor(admin, child.ID, "User", "features@example.test", "password123", model.IAMRoleUser)
	require.NoError(t, err)
	for _, feature := range featureCatalog {
		require.ErrorIs(t, s.RequireFeature(user, feature.Code), ErrForbidden)
		require.NoError(t, s.RequireFeature(admin, feature.Code))
	}
	require.NoError(t, s.RequireResourcePermission(user, "applications", "create"))
	require.ErrorIs(t, s.SetUserFeaturePolicy(user, []string{FeatureHomeAI}), ErrForbidden)
	require.ErrorIs(t, s.SetUserFeaturePolicy(admin, []string{FeaturePermissions}), ErrInvalid)
	require.ErrorIs(t, s.SetUserFeaturePolicy(admin, []string{FeatureSettingsWrite}), ErrInvalid)
	require.NoError(t, s.SetUserFeaturePolicy(admin, []string{FeatureHomeAI, FeatureSettingsRead, FeatureAudit}))
	require.NoError(t, s.RequireFeature(user, FeatureHomeAI))
	require.NoError(t, s.RequireOrganizationResourcePermission(user, child.ID, "management_agent_sessions", "use"))
	require.ErrorIs(t, s.RequireOrganizationResourcePermission(user, child.ID, "agent_sessions", "use"), ErrForbidden)
	require.NoError(t, s.RequireModelSettingsPermission(user, "read"))
	require.ErrorIs(t, s.RequireModelSettingsPermission(user, "update"), ErrForbidden)
	require.NoError(t, s.RequireResourcePermission(user, "logs", "read"))
	restarted, err := NewIAMService(repo)
	require.NoError(t, err)
	require.NoError(t, restarted.RequireFeature(user, FeatureHomeAI))
	require.NoError(t, restarted.SetUserFeaturePolicy(admin, []string{}))
	restarted, err = NewIAMService(repo)
	require.NoError(t, err)
	require.ErrorIs(t, restarted.RequireFeature(user, FeatureHomeAI), ErrForbidden)
	require.NoError(t, restarted.RequireFeature(admin, FeaturePermissions))
	require.NoError(t, restarted.RequireResourcePermission(user, "applications", "create"))
}

func TestFeaturePolicy_OrganizationsAreAdminOnly(t *testing.T) {
	s, repo, admin := newOrganizationTestService(t)
	root, err := repo.GetRootOrganization()
	require.NoError(t, err)
	user, _, err := s.CreateUserFor(admin, root.ID, "User", "org-feature@example.test", "password123", model.IAMRoleUser)
	require.NoError(t, err)
	require.ErrorIs(t, s.SetUserFeaturePolicy(admin, []string{FeatureOrganizations}), ErrInvalid)
	// 模拟以前留下的普通用户组织查看授权，仍必须拒绝访问。
	role, err := repo.GetIAMRoleByCode(model.IAMRoleUser)
	require.NoError(t, err)
	permission, err := repo.GetIAMPermissionByCode(FeatureOrganizations)
	require.NoError(t, err)
	require.NoError(t, repo.UpsertIAMRolePermission(&model.IAMRolePermission{RoleID: role.ID, PermissionID: permission.ID}))
	require.NoError(t, s.reloadAuthorization())
	_, err = s.ListOrganizationsFor(user)
	require.ErrorIs(t, err, ErrForbidden)
	features, err := s.EffectiveFeatures(user)
	require.NoError(t, err)
	require.False(t, features[FeatureOrganizations])
	_, err = s.ListOrganizationsFor(admin)
	require.NoError(t, err)
	_, err = s.CreateOrganizationFor(user, "Not allowed", "", &root.ID)
	require.True(t, errors.Is(err, ErrForbidden))
}
