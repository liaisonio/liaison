ALTER TABLE users ADD COLUMN name varchar(128) NOT NULL DEFAULT '';

CREATE TABLE IF NOT EXISTS iam_organizations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    name VARCHAR(128) NOT NULL,
    description VARCHAR(512) NOT NULL DEFAULT '',
    parent_id INTEGER NULL REFERENCES iam_organizations(id),
    is_root BOOLEAN NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_organizations_name ON iam_organizations(name);
CREATE INDEX IF NOT EXISTS idx_iam_organizations_parent_id ON iam_organizations(parent_id);
CREATE INDEX IF NOT EXISTS idx_iam_organizations_is_root ON iam_organizations(is_root);

CREATE TABLE IF NOT EXISTS iam_organization_memberships (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    organization_id INTEGER NOT NULL REFERENCES iam_organizations(id),
    user_id INTEGER NOT NULL REFERENCES users(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_organization_user ON iam_organization_memberships(organization_id, user_id);
CREATE INDEX IF NOT EXISTS idx_iam_organization_memberships_user_id ON iam_organization_memberships(user_id);

CREATE TABLE IF NOT EXISTS iam_roles (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    built_in BOOLEAN NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_roles_code ON iam_roles(code);

CREATE TABLE IF NOT EXISTS iam_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    code VARCHAR(128) NOT NULL,
    resource VARCHAR(128) NOT NULL,
    action VARCHAR(64) NOT NULL
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_permissions_code ON iam_permissions(code);

CREATE TABLE IF NOT EXISTS iam_role_permissions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    role_id INTEGER NOT NULL REFERENCES iam_roles(id),
    permission_id INTEGER NOT NULL REFERENCES iam_permissions(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_role_permission ON iam_role_permissions(role_id, permission_id);

CREATE TABLE IF NOT EXISTS iam_role_bindings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    user_id INTEGER NOT NULL REFERENCES users(id),
    role_id INTEGER NOT NULL REFERENCES iam_roles(id),
    organization_id INTEGER NOT NULL REFERENCES iam_organizations(id),
    inherit_children BOOLEAN NOT NULL DEFAULT 0
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_role_binding ON iam_role_bindings(user_id, role_id, organization_id);
CREATE INDEX IF NOT EXISTS idx_iam_role_bindings_organization_id ON iam_role_bindings(organization_id);

CREATE TABLE IF NOT EXISTS iam_resource_relations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME NOT NULL,
    updated_at DATETIME NOT NULL,
    deleted_at DATETIME NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id INTEGER NOT NULL,
    relation VARCHAR(32) NOT NULL,
    subject_type VARCHAR(32) NOT NULL,
    subject_id INTEGER NOT NULL,
    created_by INTEGER NOT NULL REFERENCES users(id)
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_iam_resource_relation ON iam_resource_relations(resource_type, resource_id, relation, subject_type, subject_id);
CREATE INDEX IF NOT EXISTS idx_iam_resource_relations_subject_id ON iam_resource_relations(subject_id);
