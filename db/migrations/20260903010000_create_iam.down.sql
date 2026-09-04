DROP TABLE IF EXISTS iam_resource_relations;
DROP TABLE IF EXISTS iam_role_bindings;
DROP TABLE IF EXISTS iam_role_permissions;
DROP TABLE IF EXISTS iam_permissions;
DROP TABLE IF EXISTS iam_roles;
DROP TABLE IF EXISTS iam_organization_memberships;
DROP TABLE IF EXISTS iam_organizations;
ALTER TABLE users DROP COLUMN name;
