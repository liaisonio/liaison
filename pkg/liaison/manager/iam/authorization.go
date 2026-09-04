package iam

import (
	_ "embed"
	"fmt"
	"strconv"

	"github.com/casbin/casbin/v2"
	casbinmodel "github.com/casbin/casbin/v2/model"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

const globalDomain = "*"

//go:embed casbin/model.conf
var casbinModel string

type authorizer struct {
	enforcer *casbin.SyncedEnforcer
}

func newAuthorizer() (*authorizer, error) {
	m, err := casbinmodel.NewModelFromString(casbinModel)
	if err != nil {
		return nil, fmt.Errorf("parse casbin model: %w", err)
	}
	enforcer, err := casbin.NewSyncedEnforcer(m)
	if err != nil {
		return nil, fmt.Errorf("create casbin enforcer: %w", err)
	}
	enforcer.EnableAutoSave(false)
	return &authorizer{enforcer: enforcer}, nil
}

func userSubject(userID uint) string {
	return "user:" + strconv.FormatUint(uint64(userID), 10)
}

func roleSubject(code model.IAMRoleCode) string {
	return "role:" + string(code)
}

func organizationDomain(id uint) string {
	return "org:" + strconv.FormatUint(uint64(id), 10)
}

func organizationObject(id uint) string {
	return "/organizations/" + strconv.FormatUint(uint64(id), 10)
}

func organizationMembersObject(id uint) string {
	return organizationObject(id) + "/members"
}

func (a *authorizer) enforce(userID uint, domain, object, action string) (bool, error) {
	allowed, err := a.enforcer.Enforce(userSubject(userID), domain, object, action)
	if err != nil {
		return false, fmt.Errorf("enforce casbin policy: %w", err)
	}
	return allowed, nil
}

func (a *authorizer) reset() {
	a.enforcer.ClearPolicy()
}

func (a *authorizer) addPermission(role model.IAMRoleCode, resource, action string) error {
	if _, err := a.enforcer.AddPolicy(roleSubject(role), globalDomain, resource, action); err != nil {
		return fmt.Errorf("load role permission: %w", err)
	}
	return nil
}

func (a *authorizer) addBinding(userID uint, role model.IAMRoleCode, domain string) error {
	if _, err := a.enforcer.AddGroupingPolicy(userSubject(userID), roleSubject(role), domain); err != nil {
		return fmt.Errorf("load role binding: %w", err)
	}
	return nil
}

func (a *authorizer) rebuildRoleLinks() error {
	if err := a.enforcer.BuildRoleLinks(); err != nil {
		return fmt.Errorf("rebuild casbin role links: %w", err)
	}
	return nil
}

func (a *authorizer) removeUser(userID uint) error {
	if _, err := a.enforcer.RemoveFilteredGroupingPolicy(0, userSubject(userID)); err != nil {
		return fmt.Errorf("remove user role bindings: %w", err)
	}
	return nil
}
