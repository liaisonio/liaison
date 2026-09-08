package dao

import (
	"errors"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (d *dao) CreateOrganization(organization *model.Organization) error {
	return d.getDB().Create(organization).Error
}

func (d *dao) GetOrganizationByID(id uint) (*model.Organization, error) {
	var organization model.Organization
	if err := d.getDB().First(&organization, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &organization, nil
}

func (d *dao) GetOrganizationByName(name string) (*model.Organization, error) {
	var organization model.Organization
	if err := d.getDB().Where("name = ?", name).First(&organization).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &organization, nil
}

func (d *dao) GetRootOrganization() (*model.Organization, error) {
	var organization model.Organization
	if err := d.getDB().Where("is_root = ?", true).First(&organization).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &organization, nil
}

func (d *dao) ListOrganizations() ([]*model.Organization, error) {
	var organizations []*model.Organization
	err := d.getDB().Order("parent_id IS NOT NULL, parent_id, id").Find(&organizations).Error
	return organizations, err
}

func (d *dao) UpdateOrganization(organization *model.Organization) error {
	return d.getDB().Save(organization).Error
}

func (d *dao) DeleteOrganization(id uint) error {
	return d.getDB().Delete(&model.Organization{}, id).Error
}

func (d *dao) CountOrganizationChildren(id uint) (int64, error) {
	var count int64
	err := d.getDB().Model(&model.Organization{}).Where("parent_id = ?", id).Count(&count).Error
	return count, err
}

func (d *dao) UpsertOrganizationMembership(membership *model.OrganizationMembership) error {
	return d.getDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "deleted_at"}),
	}).Create(membership).Error
}

func (d *dao) GetOrganizationMembership(organizationID, userID uint) (*model.OrganizationMembership, error) {
	var membership model.OrganizationMembership
	err := d.getDB().Where("organization_id = ? AND user_id = ?", organizationID, userID).First(&membership).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

func (d *dao) ListOrganizationMembers(organizationID uint) ([]*model.OrganizationMembership, error) {
	var memberships []*model.OrganizationMembership
	err := d.getDB().Preload("User").Where("organization_id = ?", organizationID).Order("id").Find(&memberships).Error
	return memberships, err
}

func (d *dao) ListUserOrganizations(userID uint) ([]*model.OrganizationMembership, error) {
	var memberships []*model.OrganizationMembership
	err := d.getDB().Preload("Organization").Where("user_id = ?", userID).Order("id").Find(&memberships).Error
	return memberships, err
}

func (d *dao) DeleteOrganizationMembership(organizationID, userID uint) error {
	return d.getDB().Where("organization_id = ? AND user_id = ?", organizationID, userID).Delete(&model.OrganizationMembership{}).Error
}

func (d *dao) DeleteOrganizationMembershipsByOrganization(organizationID uint) error {
	return d.getDB().Where("organization_id = ?", organizationID).Delete(&model.OrganizationMembership{}).Error
}

func (d *dao) DeleteOrganizationMembershipsByUser(userID uint) error {
	return d.getDB().Where("user_id = ?", userID).Delete(&model.OrganizationMembership{}).Error
}

func (d *dao) UpsertIAMRole(role *model.IAMRole) error {
	return d.getDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"name", "description", "built_in", "updated_at", "deleted_at"}),
	}).Create(role).Error
}

func (d *dao) GetIAMRoleByCode(code model.IAMRoleCode) (*model.IAMRole, error) {
	var role model.IAMRole
	err := d.getDB().Where("code = ?", code).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &role, err
}

func (d *dao) ListIAMRoles() ([]*model.IAMRole, error) {
	var roles []*model.IAMRole
	err := d.getDB().Order("id").Find(&roles).Error
	return roles, err
}

func (d *dao) UpsertIAMPermission(permission *model.IAMPermission) error {
	return d.getDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "code"}},
		DoUpdates: clause.AssignmentColumns([]string{"resource", "action", "updated_at", "deleted_at"}),
	}).Create(permission).Error
}

func (d *dao) GetIAMPermissionByCode(code string) (*model.IAMPermission, error) {
	var permission model.IAMPermission
	err := d.getDB().Where("code = ?", code).First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &permission, err
}

func (d *dao) ListIAMPermissionsByRole(roleID uint) ([]*model.IAMPermission, error) {
	var permissions []*model.IAMPermission
	err := d.getDB().Table("iam_permissions AS p").
		Joins("JOIN iam_role_permissions AS rp ON rp.permission_id = p.id AND rp.deleted_at IS NULL").
		Where("rp.role_id = ? AND p.deleted_at IS NULL", roleID).Order("p.id").Find(&permissions).Error
	return permissions, err
}

func (d *dao) UpsertIAMRolePermission(rolePermission *model.IAMRolePermission) error {
	return d.getDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "role_id"}, {Name: "permission_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"updated_at", "deleted_at"}),
	}).Create(rolePermission).Error
}

func (d *dao) UpsertIAMRoleBinding(binding *model.IAMRoleBinding) error {
	return d.getDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}, {Name: "role_id"}, {Name: "organization_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"inherit_children", "updated_at", "deleted_at"}),
	}).Create(binding).Error
}

func (d *dao) ReplaceIAMRolePermissionSubset(roleID uint, subset, enabled []uint) error {
	return d.getDB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Unscoped().Where("role_id = ? AND permission_id IN ?", roleID, subset).Delete(&model.IAMRolePermission{}).Error; err != nil {
			return err
		}
		for _, id := range enabled {
			if err := tx.Create(&model.IAMRolePermission{RoleID: roleID, PermissionID: id}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (d *dao) GetIAMRoleBinding(organizationID, userID uint) (*model.IAMRoleBinding, error) {
	var binding model.IAMRoleBinding
	err := d.getDB().Preload("Role").Where("organization_id = ? AND user_id = ?", organizationID, userID).First(&binding).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &binding, err
}

func (d *dao) ListIAMRoleBindings() ([]*model.IAMRoleBinding, error) {
	var bindings []*model.IAMRoleBinding
	err := d.getDB().Preload("Role").Preload("Organization").Order("id").Find(&bindings).Error
	return bindings, err
}

func (d *dao) ListIAMRoleBindingsByUser(userID uint) ([]*model.IAMRoleBinding, error) {
	var bindings []*model.IAMRoleBinding
	err := d.getDB().Preload("Role").Preload("Organization").Where("user_id = ?", userID).Order("id").Find(&bindings).Error
	return bindings, err
}

func (d *dao) DeleteIAMRoleBinding(organizationID, userID uint) error {
	return d.getDB().Where("organization_id = ? AND user_id = ?", organizationID, userID).Delete(&model.IAMRoleBinding{}).Error
}

func (d *dao) DeleteIAMRoleBindingsByOrganization(organizationID uint) error {
	return d.getDB().Where("organization_id = ?", organizationID).Delete(&model.IAMRoleBinding{}).Error
}

func (d *dao) DeleteIAMRoleBindingsByUser(userID uint) error {
	return d.getDB().Where("user_id = ?", userID).Delete(&model.IAMRoleBinding{}).Error
}

func (d *dao) UpsertIAMResourceRelation(relation *model.IAMResourceRelation) error {
	return d.getDB().Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "resource_type"}, {Name: "resource_id"}, {Name: "relation"}, {Name: "subject_type"}, {Name: "subject_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"created_by", "updated_at", "deleted_at"}),
	}).Create(relation).Error
}

func (d *dao) ListIAMResourceRelations(resourceType string, resourceID uint64) ([]*model.IAMResourceRelation, error) {
	var relations []*model.IAMResourceRelation
	err := d.getDB().Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Order("id").Find(&relations).Error
	return relations, err
}

func (d *dao) ListIAMResourceIDsForSubject(resourceType string, subjectType model.IAMSubjectType, subjectID uint) ([]uint64, error) {
	var ids []uint64
	err := d.getDB().Model(&model.IAMResourceRelation{}).Distinct("resource_id").
		Where("resource_type = ? AND subject_type = ? AND subject_id = ?", resourceType, subjectType, subjectID).
		Order("resource_id").Pluck("resource_id", &ids).Error
	return ids, err
}

func (d *dao) DeleteIAMResourceRelations(resourceType string, resourceID uint64) error {
	return d.getDB().Where("resource_type = ? AND resource_id = ?", resourceType, resourceID).Delete(&model.IAMResourceRelation{}).Error
}
