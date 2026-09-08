package iam

import (
	"fmt"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

const (
	FeatureAudit         = "audit.read"
	FeatureOrganizations = "organizations.read"
	FeatureHomeAI        = "ai.home.use"
	FeatureAccessAI      = "ai.access.use"
	FeatureSettingsRead  = "settings.global.read"
	FeatureSettingsWrite = "settings.global.update"
	FeaturePermissions   = "permissions.manage"
)

type Feature struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

var featureCatalog = []Feature{
	{FeatureAudit, "日志与审计"},
	{FeatureHomeAI, "首页 AI"}, {FeatureAccessAI, "访问层 AI"},
	{FeatureSettingsRead, "查看全局配置"}, {FeatureSettingsWrite, "修改全局配置"},
}

func allFeatures() []Feature {
	return append(append([]Feature{}, featureCatalog...), Feature{FeaturePermissions, "权限管理"}, Feature{FeatureOrganizations, "组织架构"})
}

// Optional grants are never seeded for users, so a saved empty policy survives restart.
func (s *IAMService) ensureFeatureCatalog() error {
	admin, err := s.repo.GetIAMRoleByCode(model.IAMRoleAdmin)
	if err != nil {
		return err
	}
	if admin == nil {
		return ErrForbidden
	}
	for _, feature := range allFeatures() {
		p := &model.IAMPermission{Code: feature.Code, Resource: "/features/" + feature.Code, Action: "use"}
		if err := s.repo.UpsertIAMPermission(p); err != nil {
			return err
		}
		stored, err := s.repo.GetIAMPermissionByCode(feature.Code)
		if err != nil {
			return err
		}
		if err := s.repo.UpsertIAMRolePermission(&model.IAMRolePermission{RoleID: admin.ID, PermissionID: stored.ID}); err != nil {
			return err
		}
	}
	return nil
}

func (s *IAMService) RequireFeature(actor *model.User, code string) error {
	// 组织架构属于管理功能，旧的普通用户授权不能绕过管理权限检查。
	if code == FeatureOrganizations {
		if err := s.RequireFeature(actor, FeaturePermissions); err != nil {
			return err
		}
	}
	root, err := s.repo.GetRootOrganization()
	if err != nil {
		return err
	}
	if root == nil || actor == nil || actor.ID == 0 {
		return ErrForbidden
	}
	allowed, err := s.hasPermission(actor, organizationDomain(root.ID), "/features/"+code, "use")
	if err != nil {
		return err
	}
	if allowed {
		return nil
	}
	if code == FeaturePermissions {
		return ErrForbidden
	}
	// The user role is the baseline feature policy for accounts in any organization.
	// Evaluate that role through Casbin, not by reading permission rows as booleans.
	bindings, err := s.repo.ListIAMRoleBindingsByUser(actor.ID)
	if err != nil {
		return err
	}
	if len(bindings) == 0 {
		return ErrForbidden
	}
	s.authorizationMu.RLock()
	allowed, err = s.authorizer.enforcer.Enforce(roleSubject(model.IAMRoleUser), organizationDomain(root.ID), "/features/"+code, "use")
	s.authorizationMu.RUnlock()
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func (s *IAMService) EffectiveFeatures(actor *model.User) (map[string]bool, error) {
	result := make(map[string]bool)
	for _, feature := range allFeatures() {
		err := s.RequireFeature(actor, feature.Code)
		if err != nil && err != ErrForbidden {
			return nil, err
		}
		result[feature.Code] = err == nil
	}
	return result, nil
}

type UserFeaturePolicy struct {
	Catalog []Feature `json:"catalog"`
	Enabled []string  `json:"enabled"`
}

func (s *IAMService) UserFeaturePolicy(actor *model.User) (UserFeaturePolicy, error) {
	if err := s.RequireFeature(actor, FeaturePermissions); err != nil {
		return UserFeaturePolicy{}, err
	}
	role, err := s.repo.GetIAMRoleByCode(model.IAMRoleUser)
	if err != nil {
		return UserFeaturePolicy{}, err
	}
	if role == nil {
		return UserFeaturePolicy{}, ErrForbidden
	}
	permissions, err := s.repo.ListIAMPermissionsByRole(role.ID)
	if err != nil {
		return UserFeaturePolicy{}, err
	}
	policy := UserFeaturePolicy{Catalog: append([]Feature{}, featureCatalog...), Enabled: []string{}}
	for _, f := range featureCatalog {
		for _, p := range permissions {
			if p.Code == f.Code {
				policy.Enabled = append(policy.Enabled, f.Code)
			}
		}
	}
	return policy, nil
}

func (s *IAMService) SetUserFeaturePolicy(actor *model.User, enabled []string) error {
	s.featureMu.Lock()
	defer s.featureMu.Unlock()
	if err := s.RequireFeature(actor, FeaturePermissions); err != nil {
		return err
	}
	selected := map[string]bool{}
	for _, code := range enabled {
		valid := false
		for _, f := range featureCatalog {
			if f.Code == code {
				valid = true
			}
		}
		if !valid {
			return fmt.Errorf("%w: unknown feature", ErrInvalid)
		}
		selected[code] = true
	}
	if selected[FeatureSettingsWrite] && !selected[FeatureSettingsRead] {
		return fmt.Errorf("%w: modifying settings requires read permission", ErrInvalid)
	}
	role, err := s.repo.GetIAMRoleByCode(model.IAMRoleUser)
	if err != nil {
		return err
	}
	if role == nil {
		return ErrForbidden
	}
	var all, grants []uint
	for _, f := range featureCatalog {
		p, err := s.repo.GetIAMPermissionByCode(f.Code)
		if err != nil {
			return err
		}
		if p == nil {
			return ErrInvalid
		}
		all = append(all, p.ID)
		if selected[f.Code] {
			grants = append(grants, p.ID)
		}
	}
	if err := s.repo.ReplaceIAMRolePermissionSubset(role.ID, all, grants); err != nil {
		return err
	}
	return s.reloadAuthorization()
}

func (s *IAMService) requireResourceFeature(actor *model.User, resource string) error {
	switch resource {
	case "logs":
		return s.RequireFeature(actor, FeatureAudit)
	case "agent_sessions":
		return s.RequireFeature(actor, FeatureAccessAI)
	case "management_agent_sessions":
		return s.RequireFeature(actor, FeatureHomeAI)
	}
	return nil
}
