package model

import "gorm.io/gorm"

const DefaultOrganizationName = "默认组织"

// Organization is a node in the organization tree. The single IsRoot row is
// the installation root; platform-wide administration is represented by an
// administrator role binding on that organization, not by a separate scope.
type Organization struct {
	gorm.Model
	Name        string `gorm:"column:name;type:varchar(128);uniqueIndex;not null" json:"name"`
	Description string `gorm:"column:description;type:varchar(512);not null;default:''" json:"description"`
	ParentID    *uint  `gorm:"column:parent_id;index" json:"parent_id,omitempty"`
	IsRoot      bool   `gorm:"column:is_root;not null;default:false;index" json:"is_root"`
}

func (Organization) TableName() string { return "iam_organizations" }

// OrganizationMembership only records membership. Authorization is expressed
// separately by IAMRoleBinding so future roles do not alter this table.
type OrganizationMembership struct {
	gorm.Model
	OrganizationID uint          `gorm:"column:organization_id;not null;uniqueIndex:idx_iam_organization_user" json:"organization_id"`
	UserID         uint          `gorm:"column:user_id;not null;uniqueIndex:idx_iam_organization_user;index" json:"user_id"`
	User           *User         `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Organization   *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
	Role           IAMRoleCode   `gorm:"-" json:"role"`
}

func (OrganizationMembership) TableName() string { return "iam_organization_memberships" }

type IAMRoleCode string

const (
	IAMRoleAdmin IAMRoleCode = "admin"
	IAMRoleUser  IAMRoleCode = "user"
)

// OrganizationRole remains an API-level alias while persistence lives in
// iam_role_bindings.
type OrganizationRole = IAMRoleCode

const (
	OrganizationRoleAdmin  = IAMRoleAdmin
	OrganizationRoleMember = IAMRoleUser
)

func IsValidOrganizationRole(code OrganizationRole) bool { return IsValidIAMRoleCode(code) }

func IsValidIAMRoleCode(code IAMRoleCode) bool { return code == IAMRoleAdmin || code == IAMRoleUser }

type IAMRole struct {
	gorm.Model
	Code        IAMRoleCode `gorm:"column:code;type:varchar(32);uniqueIndex;not null" json:"code"`
	Name        string      `gorm:"column:name;type:varchar(64);not null" json:"name"`
	Description string      `gorm:"column:description;type:varchar(255);not null;default:''" json:"description"`
	BuiltIn     bool        `gorm:"column:built_in;not null;default:false" json:"built_in"`
}

func (IAMRole) TableName() string { return "iam_roles" }

type IAMPermission struct {
	gorm.Model
	Code     string `gorm:"column:code;type:varchar(128);uniqueIndex;not null" json:"code"`
	Resource string `gorm:"column:resource;type:varchar(128);not null" json:"resource"`
	Action   string `gorm:"column:action;type:varchar(64);not null" json:"action"`
}

func (IAMPermission) TableName() string { return "iam_permissions" }

type IAMRolePermission struct {
	gorm.Model
	RoleID       uint `gorm:"column:role_id;not null;uniqueIndex:idx_iam_role_permission" json:"role_id"`
	PermissionID uint `gorm:"column:permission_id;not null;uniqueIndex:idx_iam_role_permission" json:"permission_id"`
}

func (IAMRolePermission) TableName() string { return "iam_role_permissions" }

// IAMRoleBinding grants a role to a user within an organization. A root admin
// binding with InheritChildren=true grants installation-wide administration.
type IAMRoleBinding struct {
	gorm.Model
	UserID          uint          `gorm:"column:user_id;not null;uniqueIndex:idx_iam_role_binding" json:"user_id"`
	RoleID          uint          `gorm:"column:role_id;not null;uniqueIndex:idx_iam_role_binding" json:"role_id"`
	OrganizationID  uint          `gorm:"column:organization_id;not null;uniqueIndex:idx_iam_role_binding;index" json:"organization_id"`
	InheritChildren bool          `gorm:"column:inherit_children;not null;default:false" json:"inherit_children"`
	Role            *IAMRole      `gorm:"foreignKey:RoleID" json:"role,omitempty"`
	Organization    *Organization `gorm:"foreignKey:OrganizationID" json:"organization,omitempty"`
}

func (IAMRoleBinding) TableName() string { return "iam_role_bindings" }

type IAMSubjectType string
type IAMResourceRelationType string

const (
	IAMSubjectUser         IAMSubjectType          = "user"
	IAMSubjectOrganization IAMSubjectType          = "organization"
	IAMRelationOwner       IAMResourceRelationType = "owner"
	IAMRelationBelongsTo   IAMResourceRelationType = "belongs_to"
)

// IAMResourceRelation is the generic object-level authorization tuple and is
// deliberately independent from individual resource tables.
type IAMResourceRelation struct {
	gorm.Model
	ResourceType string                  `gorm:"column:resource_type;type:varchar(64);not null;uniqueIndex:idx_iam_resource_relation" json:"resource_type"`
	ResourceID   uint64                  `gorm:"column:resource_id;not null;uniqueIndex:idx_iam_resource_relation" json:"resource_id"`
	Relation     IAMResourceRelationType `gorm:"column:relation;type:varchar(32);not null;uniqueIndex:idx_iam_resource_relation" json:"relation"`
	SubjectType  IAMSubjectType          `gorm:"column:subject_type;type:varchar(32);not null;uniqueIndex:idx_iam_resource_relation" json:"subject_type"`
	SubjectID    uint                    `gorm:"column:subject_id;not null;uniqueIndex:idx_iam_resource_relation;index" json:"subject_id"`
	CreatedBy    uint                    `gorm:"column:created_by;not null" json:"created_by"`
}

func (IAMResourceRelation) TableName() string { return "iam_resource_relations" }
