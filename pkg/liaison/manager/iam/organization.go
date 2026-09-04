package iam

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/utils"
)

var (
	ErrForbidden = errors.New("forbidden")
	ErrNotFound  = errors.New("not found")
	ErrInvalid   = errors.New("invalid request")
)

type permissionSeed struct {
	code, resource, action string
	roles                  []model.IAMRoleCode
}

var builtInPermissionSeeds = []permissionSeed{
	{"users.collection.manage", "/users", "(read|create)", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"users.item.manage", "/users/:id", "(update|delete)", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"users.password.reset", "/users/:id/password", "reset", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"organizations.collection.manage", "/organizations", "(read|create)", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"organizations.item.manage", "/organizations/:id", "(read|update|delete|create_child)", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"organizations.members.manage", "/organizations/:id/members", "(read|write)", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"resources.admin", "/resources/*", "(read|create|update|delete|use)", []model.IAMRoleCode{model.IAMRoleAdmin}},
	{"organizations.collection.read", "/organizations", "read", []model.IAMRoleCode{model.IAMRoleUser}},
	{"organizations.item.read", "/organizations/:id", "read", []model.IAMRoleCode{model.IAMRoleUser}},
	{"organizations.members.read", "/organizations/:id/members", "read", []model.IAMRoleCode{model.IAMRoleUser}},
	{"resources.own", "/resources/*", "(read|create|update|delete|use)", []model.IAMRoleCode{model.IAMRoleUser}},
}

func (s *IAMService) ensureIAMCatalog() error {
	roles := []*model.IAMRole{
		{Code: model.IAMRoleAdmin, Name: "管理员", Description: "管理组织、用户及全部资源", BuiltIn: true},
		{Code: model.IAMRoleUser, Name: "用户", Description: "管理自己创建或被授权的资源", BuiltIn: true},
	}
	for _, role := range roles {
		if err := s.repo.UpsertIAMRole(role); err != nil {
			return fmt.Errorf("seed IAM role %s: %w", role.Code, err)
		}
	}
	for _, seed := range builtInPermissionSeeds {
		permission := &model.IAMPermission{Code: seed.code, Resource: seed.resource, Action: seed.action}
		if err := s.repo.UpsertIAMPermission(permission); err != nil {
			return fmt.Errorf("seed IAM permission %s: %w", seed.code, err)
		}
		storedPermission, err := s.repo.GetIAMPermissionByCode(seed.code)
		if err != nil {
			return err
		}
		for _, roleCode := range seed.roles {
			role, err := s.repo.GetIAMRoleByCode(roleCode)
			if err != nil {
				return err
			}
			if role == nil || storedPermission == nil {
				return fmt.Errorf("seed IAM catalog: role or permission missing")
			}
			if err := s.repo.UpsertIAMRolePermission(&model.IAMRolePermission{RoleID: role.ID, PermissionID: storedPermission.ID}); err != nil {
				return fmt.Errorf("bind IAM permission %s: %w", seed.code, err)
			}
		}
	}
	return nil
}

// EnsureOrganizationBootstrap creates the single immutable root organization.
// The first account receives an inherited admin binding; later accounts receive
// a user binding. No legacy role flags or migration path are involved.
func (s *IAMService) EnsureOrganizationBootstrap() error {
	users, _, err := s.repo.ListUsers(0, 100000)
	if err != nil {
		return fmt.Errorf("list users for organization bootstrap: %w", err)
	}
	if len(users) == 0 {
		return nil
	}
	sort.Slice(users, func(i, j int) bool { return users[i].ID < users[j].ID })
	root, err := s.repo.GetRootOrganization()
	if err != nil {
		return fmt.Errorf("get default organization: %w", err)
	}
	if root == nil {
		root = &model.Organization{Name: model.DefaultOrganizationName, Description: "系统根组织", IsRoot: true}
		if err := s.repo.CreateOrganization(root); err != nil {
			return fmt.Errorf("create default organization: %w", err)
		}
	}
	for i, user := range users {
		roleCode := model.IAMRoleUser
		if i == 0 {
			roleCode = model.IAMRoleAdmin
		}
		membership, err := s.repo.GetOrganizationMembership(root.ID, user.ID)
		if err != nil {
			return fmt.Errorf("get bootstrap membership: %w", err)
		}
		if membership == nil {
			if err := s.repo.UpsertOrganizationMembership(&model.OrganizationMembership{OrganizationID: root.ID, UserID: user.ID}); err != nil {
				return fmt.Errorf("create bootstrap membership: %w", err)
			}
		}
		if binding, err := s.repo.GetIAMRoleBinding(root.ID, user.ID); err != nil {
			return fmt.Errorf("get bootstrap role binding: %w", err)
		} else if binding == nil {
			role, err := s.repo.GetIAMRoleByCode(roleCode)
			if err != nil {
				return err
			}
			if role == nil {
				return fmt.Errorf("bootstrap role %s not found", roleCode)
			}
			if err := s.repo.UpsertIAMRoleBinding(&model.IAMRoleBinding{UserID: user.ID, RoleID: role.ID, OrganizationID: root.ID, InheritChildren: roleCode == model.IAMRoleAdmin}); err != nil {
				return fmt.Errorf("create bootstrap role binding: %w", err)
			}
		}
	}
	return s.reloadAuthorization()
}

func accountName(email string) string {
	name, _, _ := strings.Cut(strings.TrimSpace(email), "@")
	if name == "" {
		return "user"
	}
	return name
}

func (s *IAMService) reloadAuthorization() error {
	s.authorizer.reset()
	roles, err := s.repo.ListIAMRoles()
	if err != nil {
		return fmt.Errorf("list roles for authorization: %w", err)
	}
	for _, role := range roles {
		permissions, err := s.repo.ListIAMPermissionsByRole(role.ID)
		if err != nil {
			return fmt.Errorf("list permissions for role %s: %w", role.Code, err)
		}
		for _, permission := range permissions {
			if err := s.authorizer.addPermission(role.Code, permission.Resource, permission.Action); err != nil {
				return err
			}
		}
	}
	bindings, err := s.repo.ListIAMRoleBindings()
	if err != nil {
		return fmt.Errorf("list role bindings for authorization: %w", err)
	}
	organizations, err := s.repo.ListOrganizations()
	if err != nil {
		return fmt.Errorf("list organizations for authorization: %w", err)
	}
	for _, binding := range bindings {
		if binding.Role == nil {
			return fmt.Errorf("role binding %d has no role", binding.ID)
		}
		if err := s.authorizer.addBinding(binding.UserID, binding.Role.Code, organizationDomain(binding.OrganizationID)); err != nil {
			return err
		}
		if binding.Organization != nil && binding.Organization.IsRoot {
			if err := s.authorizer.addBinding(binding.UserID, binding.Role.Code, globalDomain); err != nil {
				return err
			}
		}
		if binding.InheritChildren {
			for _, organization := range organizations {
				if organization.ID != binding.OrganizationID && isOrganizationDescendant(organizations, organization.ID, binding.OrganizationID) {
					if err := s.authorizer.addBinding(binding.UserID, binding.Role.Code, organizationDomain(organization.ID)); err != nil {
						return err
					}
				}
			}
		}
	}
	return s.authorizer.rebuildRoleLinks()
}

func isOrganizationDescendant(organizations []*model.Organization, candidateID, ancestorID uint) bool {
	parents := make(map[uint]*uint, len(organizations))
	for _, organization := range organizations {
		parents[organization.ID] = organization.ParentID
	}
	current := candidateID
	for hops := 0; hops < len(organizations); hops++ {
		parent := parents[current]
		if parent == nil {
			return false
		}
		if *parent == ancestorID {
			return true
		}
		current = *parent
	}
	return false
}

func (s *IAMService) requirePermission(actor *model.User, domain, object, action string) error {
	if actor == nil {
		return ErrForbidden
	}
	allowed, err := s.authorizer.enforce(actor.ID, domain, object, action)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrForbidden
	}
	return nil
}

func (s *IAMService) hasPermission(actor *model.User, domain, object, action string) (bool, error) {
	if actor == nil {
		return false, nil
	}
	return s.authorizer.enforce(actor.ID, domain, object, action)
}

// RequireResourcePermission authorizes an action in any organization where the
// actor has a role binding. Resource visibility is additionally constrained by
// iam_resource_relations in the control plane.
func (s *IAMService) RequireResourcePermission(actor *model.User, resource, action string) error {
	resource = strings.Trim(strings.TrimSpace(resource), "/")
	if resource == "" {
		return fmt.Errorf("%w: resource type is required", ErrInvalid)
	}
	if actor == nil {
		return ErrForbidden
	}
	bindings, err := s.repo.ListIAMRoleBindingsByUser(actor.ID)
	if err != nil {
		return err
	}
	for _, binding := range bindings {
		allowed, err := s.hasPermission(actor, organizationDomain(binding.OrganizationID), "/resources/"+resource, action)
		if err != nil {
			return err
		}
		if allowed {
			return nil
		}
	}
	return ErrForbidden
}

func (s *IAMService) ListUsersFor(actor *model.User) ([]*model.User, int64, error) {
	if err := s.requirePermission(actor, globalDomain, "/users", "read"); err != nil {
		return nil, 0, err
	}
	return s.repo.ListUsers(0, 100000)
}

func (s *IAMService) CreateUserFor(actor *model.User, organizationID uint, name, email, password string, roleCode model.IAMRoleCode) (*model.User, string, error) {
	if err := s.requirePermission(actor, globalDomain, "/users", "create"); err != nil {
		return nil, "", err
	}
	if organizationID == 0 {
		return nil, "", fmt.Errorf("%w: organization is required", ErrInvalid)
	}
	organization, err := s.repo.GetOrganizationByID(organizationID)
	if err != nil {
		return nil, "", err
	}
	if organization == nil {
		return nil, "", fmt.Errorf("%w: organization not found", ErrInvalid)
	}
	if err := s.requirePermission(actor, organizationDomain(organizationID), organizationObject(organizationID), "update"); err != nil {
		return nil, "", err
	}
	name, email = strings.TrimSpace(name), strings.ToLower(strings.TrimSpace(email))
	if email == "" || !strings.Contains(email, "@") {
		return nil, "", fmt.Errorf("%w: valid email required", ErrInvalid)
	}
	if exists, err := s.repo.CheckUserExists(email); err != nil {
		return nil, "", err
	} else if exists {
		return nil, "", fmt.Errorf("%w: email already exists", ErrInvalid)
	}
	if roleCode == "" {
		roleCode = model.IAMRoleUser
	}
	if !model.IsValidIAMRoleCode(roleCode) {
		return nil, "", fmt.Errorf("%w: invalid role", ErrInvalid)
	}
	generated := ""
	if password == "" {
		var err error
		generated, err = utils.GenerateRandomPassword(16)
		if err != nil {
			return nil, "", fmt.Errorf("generate password: %w", err)
		}
		password = generated
	}
	if len(password) < 8 {
		return nil, "", fmt.Errorf("%w: password must contain at least 8 characters", ErrInvalid)
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return nil, "", fmt.Errorf("hash password: %w", err)
	}
	if name == "" {
		name = accountName(email)
	}
	user := &model.User{Name: name, Email: email, Password: hash, Status: model.UserStatusActive}
	if err := s.repo.CreateUser(user); err != nil {
		return nil, "", fmt.Errorf("create user: %w", err)
	}
	if err := s.setOrganizationRole(organization.ID, user.ID, roleCode, roleCode == model.IAMRoleAdmin); err != nil {
		return nil, "", err
	}
	return user, generated, nil
}

func (s *IAMService) UpdateUserFor(actor *model.User, id uint, name string, status model.UserStatus, roleCode *model.IAMRoleCode) (*model.User, error) {
	if err := s.requirePermission(actor, globalDomain, "/users/"+fmt.Sprint(id), "update"); err != nil {
		return nil, err
	}
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrNotFound
	}
	if strings.TrimSpace(name) != "" {
		user.Name = strings.TrimSpace(name)
	}
	if status != "" {
		switch status {
		case model.UserStatusActive, model.UserStatusInactive, model.UserStatusLocked:
			user.Status = status
		default:
			return nil, fmt.Errorf("%w: invalid status", ErrInvalid)
		}
	}
	if roleCode != nil {
		if !model.IsValidIAMRoleCode(*roleCode) {
			return nil, fmt.Errorf("%w: invalid role", ErrInvalid)
		}
		if actor.ID == id && *roleCode != model.IAMRoleAdmin {
			return nil, fmt.Errorf("%w: cannot remove your own administrator role", ErrInvalid)
		}
	}
	if err := s.repo.UpdateUser(user); err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}
	if roleCode != nil {
		root, err := s.repo.GetRootOrganization()
		if err != nil {
			return nil, err
		}
		if root == nil {
			return nil, ErrNotFound
		}
		if err := s.setOrganizationRole(root.ID, user.ID, *roleCode, *roleCode == model.IAMRoleAdmin); err != nil {
			return nil, err
		}
	}
	return user, nil
}

func (s *IAMService) UserRole(userID uint) (model.IAMRoleCode, error) {
	bindings, err := s.repo.ListIAMRoleBindingsByUser(userID)
	if err != nil {
		return "", err
	}
	role := model.IAMRoleCode("")
	for _, binding := range bindings {
		if binding.Role == nil {
			continue
		}
		if binding.Role.Code == model.IAMRoleAdmin {
			return model.IAMRoleAdmin, nil
		}
		if binding.Role.Code == model.IAMRoleUser {
			role = model.IAMRoleUser
		}
	}
	if role == "" {
		return "", ErrNotFound
	}
	return role, nil
}

func (s *IAMService) ResetUserPasswordFor(actor *model.User, id uint, password string) error {
	if err := s.requirePermission(actor, globalDomain, "/users/"+fmt.Sprint(id)+"/password", "reset"); err != nil {
		return err
	}
	if len(password) < 8 {
		return fmt.Errorf("%w: password must contain at least 8 characters", ErrInvalid)
	}
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	hash, err := utils.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}
	user.Password = hash
	return s.repo.UpdateUser(user)
}

func (s *IAMService) DeleteUserFor(actor *model.User, id uint) error {
	if err := s.requirePermission(actor, globalDomain, "/users/"+fmt.Sprint(id), "delete"); err != nil {
		return err
	}
	if actor.ID == id {
		return fmt.Errorf("%w: cannot delete your own account", ErrInvalid)
	}
	user, err := s.repo.GetUserByID(id)
	if err != nil {
		return err
	}
	if user == nil {
		return ErrNotFound
	}
	tx := s.repo.Begin()
	if err := tx.DeleteIAMRoleBindingsByUser(id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete role bindings: %w", err)
	}
	if err := tx.DeleteOrganizationMembershipsByUser(id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete memberships: %w", err)
	}
	if err := tx.DeleteUser(id); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("delete user: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.authorizer.removeUser(id)
}

func (s *IAMService) setOrganizationRole(organizationID, userID uint, roleCode model.IAMRoleCode, inheritChildren bool) error {
	if !model.IsValidIAMRoleCode(roleCode) {
		return fmt.Errorf("%w: invalid role", ErrInvalid)
	}
	role, err := s.repo.GetIAMRoleByCode(roleCode)
	if err != nil {
		return err
	}
	if role == nil {
		return fmt.Errorf("%w: role not found", ErrNotFound)
	}
	tx := s.repo.Begin()
	if err := tx.UpsertOrganizationMembership(&model.OrganizationMembership{OrganizationID: organizationID, UserID: userID}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save organization membership: %w", err)
	}
	if err := tx.DeleteIAMRoleBinding(organizationID, userID); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("replace role binding: %w", err)
	}
	if err := tx.UpsertIAMRoleBinding(&model.IAMRoleBinding{OrganizationID: organizationID, UserID: userID, RoleID: role.ID, InheritChildren: inheritChildren}); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("save role binding: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit role binding: %w", err)
	}
	return s.reloadAuthorization()
}

func (s *IAMService) ListOrganizationsFor(actor *model.User) ([]*model.Organization, error) {
	if err := s.requirePermission(actor, globalDomain, "/organizations", "read"); err != nil {
		return nil, err
	}
	return s.repo.ListOrganizations()
}

func (s *IAMService) canManageOrganization(actor *model.User, organizationID uint) (bool, error) {
	return s.hasPermission(actor, organizationDomain(organizationID), organizationObject(organizationID), "update")
}

func (s *IAMService) CanManageOrganization(actor *model.User, organizationID uint) (bool, error) {
	return s.canManageOrganization(actor, organizationID)
}

func (s *IAMService) CanDeleteOrganization(actor *model.User, organizationID uint) (bool, error) {
	return s.hasPermission(actor, organizationDomain(organizationID), organizationObject(organizationID), "delete")
}

func (s *IAMService) CreateOrganizationFor(actor *model.User, name, description string, parentID *uint) (*model.Organization, error) {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 128 {
		return nil, fmt.Errorf("%w: organization name is required and must be at most 128 characters", ErrInvalid)
	}
	if parentID == nil {
		root, err := s.repo.GetRootOrganization()
		if err != nil {
			return nil, err
		}
		if root != nil {
			id := root.ID
			parentID = &id
		}
	}
	if parentID != nil {
		parent, err := s.repo.GetOrganizationByID(*parentID)
		if err != nil {
			return nil, err
		}
		if parent == nil {
			return nil, fmt.Errorf("%w: parent organization not found", ErrInvalid)
		}
		if err := s.requirePermission(actor, organizationDomain(*parentID), organizationObject(*parentID), "create_child"); err != nil {
			return nil, err
		}
	} else if err := s.requirePermission(actor, globalDomain, "/organizations", "create"); err != nil {
		return nil, err
	}
	org := &model.Organization{Name: name, Description: strings.TrimSpace(description), ParentID: parentID}
	tx := s.repo.Begin()
	if err := tx.CreateOrganization(org); err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("create organization: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit organization creation: %w", err)
	}
	if actor != nil {
		if err := s.setOrganizationRole(org.ID, actor.ID, model.IAMRoleAdmin, true); err != nil {
			return nil, err
		}
	}
	return org, nil
}

func (s *IAMService) UpdateOrganizationFor(actor *model.User, id uint, name, description string, parentID *uint, setParent bool) (*model.Organization, error) {
	if err := s.requirePermission(actor, organizationDomain(id), organizationObject(id), "update"); err != nil {
		return nil, err
	}
	org, err := s.repo.GetOrganizationByID(id)
	if err != nil {
		return nil, err
	}
	if org == nil {
		return nil, ErrNotFound
	}
	if org.IsRoot && strings.TrimSpace(name) != "" && strings.TrimSpace(name) != org.Name {
		return nil, fmt.Errorf("%w: default organization cannot be renamed", ErrInvalid)
	}
	if strings.TrimSpace(name) != "" {
		org.Name = strings.TrimSpace(name)
	}
	org.Description = strings.TrimSpace(description)
	if setParent {
		if org.IsRoot && parentID != nil {
			return nil, fmt.Errorf("%w: root organization cannot be moved", ErrInvalid)
		}
		if parentID != nil {
			if *parentID == id {
				return nil, fmt.Errorf("%w: organization cannot be its own parent", ErrInvalid)
			}
			if err := s.requirePermission(actor, organizationDomain(*parentID), organizationObject(*parentID), "create_child"); err != nil {
				return nil, err
			}
			cursor := *parentID
			for hops := 0; hops < 1024; hops++ {
				ancestor, err := s.repo.GetOrganizationByID(cursor)
				if err != nil {
					return nil, err
				}
				if ancestor == nil {
					return nil, fmt.Errorf("%w: parent organization not found", ErrInvalid)
				}
				if ancestor.ID == id {
					return nil, fmt.Errorf("%w: organization hierarchy cycle", ErrInvalid)
				}
				if ancestor.ParentID == nil {
					break
				}
				cursor = *ancestor.ParentID
			}
		}
		org.ParentID = parentID
	}
	if err := s.repo.UpdateOrganization(org); err != nil {
		return nil, fmt.Errorf("update organization: %w", err)
	}
	return org, nil
}

func (s *IAMService) DeleteOrganizationFor(actor *model.User, id uint) error {
	if err := s.requirePermission(actor, organizationDomain(id), organizationObject(id), "delete"); err != nil {
		return err
	}
	org, err := s.repo.GetOrganizationByID(id)
	if err != nil {
		return err
	}
	if org == nil {
		return ErrNotFound
	}
	if org.IsRoot {
		return fmt.Errorf("%w: default organization cannot be deleted", ErrInvalid)
	}
	if count, err := s.repo.CountOrganizationChildren(id); err != nil {
		return err
	} else if count > 0 {
		return fmt.Errorf("%w: move child organizations first", ErrInvalid)
	}
	tx := s.repo.Begin()
	if err := tx.DeleteIAMRoleBindingsByOrganization(id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.DeleteOrganizationMembershipsByOrganization(id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.DeleteOrganization(id); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.reloadAuthorization()
}

func (s *IAMService) ListOrganizationMembersFor(actor *model.User, organizationID uint) ([]*model.OrganizationMembership, error) {
	if err := s.requirePermission(actor, organizationDomain(organizationID), organizationMembersObject(organizationID), "read"); err != nil {
		return nil, err
	}
	return s.listOrganizationMembersWithRoles(organizationID)
}

func (s *IAMService) UpsertOrganizationMemberFor(actor *model.User, organizationID, userID uint, role model.OrganizationRole) (*model.OrganizationMembership, error) {
	if err := s.requirePermission(actor, organizationDomain(organizationID), organizationMembersObject(organizationID), "write"); err != nil {
		return nil, err
	}
	if !model.IsValidOrganizationRole(role) {
		return nil, fmt.Errorf("%w: invalid organization role", ErrInvalid)
	}
	if org, err := s.repo.GetOrganizationByID(organizationID); err != nil {
		return nil, err
	} else if org == nil {
		return nil, ErrNotFound
	}
	if user, err := s.repo.GetUserByID(userID); err != nil {
		return nil, err
	} else if user == nil {
		return nil, fmt.Errorf("%w: user not found", ErrInvalid)
	}
	current, err := s.repo.GetIAMRoleBinding(organizationID, userID)
	if err != nil {
		return nil, err
	}
	if current != nil && current.Role != nil && current.Role.Code == model.IAMRoleAdmin && role != model.IAMRoleAdmin {
		members, err := s.listOrganizationMembersWithRoles(organizationID)
		if err != nil {
			return nil, err
		}
		admins := 0
		for _, member := range members {
			if member.Role == model.OrganizationRoleAdmin {
				admins++
			}
		}
		if admins <= 1 {
			return nil, fmt.Errorf("%w: organization must keep at least one administrator", ErrInvalid)
		}
	}
	if err := s.setOrganizationRole(organizationID, userID, role, role == model.IAMRoleAdmin); err != nil {
		return nil, err
	}
	membership, err := s.repo.GetOrganizationMembership(organizationID, userID)
	if membership != nil {
		membership.Role = role
	}
	return membership, err
}

func (s *IAMService) UpsertOrganizationMemberByEmailFor(actor *model.User, organizationID uint, email string, role model.OrganizationRole) (*model.OrganizationMembership, error) {
	user, err := s.repo.GetUserByEmail(strings.ToLower(strings.TrimSpace(email)))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("%w: user not found", ErrInvalid)
	}
	return s.UpsertOrganizationMemberFor(actor, organizationID, user.ID, role)
}

func (s *IAMService) RemoveOrganizationMemberFor(actor *model.User, organizationID, userID uint) error {
	if err := s.requirePermission(actor, organizationDomain(organizationID), organizationMembersObject(organizationID), "write"); err != nil {
		return err
	}
	members, err := s.listOrganizationMembersWithRoles(organizationID)
	if err != nil {
		return err
	}
	admins := 0
	for _, member := range members {
		if member.Role == model.OrganizationRoleAdmin {
			admins++
		}
	}
	current, err := s.repo.GetIAMRoleBinding(organizationID, userID)
	if err != nil {
		return err
	}
	if current == nil {
		return ErrNotFound
	}
	if current.Role != nil && current.Role.Code == model.IAMRoleAdmin && admins <= 1 {
		return fmt.Errorf("%w: organization must keep at least one administrator", ErrInvalid)
	}
	tx := s.repo.Begin()
	if err := tx.DeleteIAMRoleBinding(organizationID, userID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.DeleteOrganizationMembership(organizationID, userID); err != nil {
		_ = tx.Rollback()
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	return s.reloadAuthorization()
}

func (s *IAMService) listOrganizationMembersWithRoles(organizationID uint) ([]*model.OrganizationMembership, error) {
	members, err := s.repo.ListOrganizationMembers(organizationID)
	if err != nil {
		return nil, err
	}
	for _, member := range members {
		binding, err := s.repo.GetIAMRoleBinding(organizationID, member.UserID)
		if err != nil {
			return nil, err
		}
		if binding != nil && binding.Role != nil {
			member.Role = binding.Role.Code
		}
	}
	return members, nil
}
