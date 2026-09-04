package controlplane

import (
	"context"

	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

const (
	resourceConnector   = "connector"
	resourceDevice      = "device"
	resourceApplication = "application"
	resourceAccess      = "access"
)

func actorUserID(ctx context.Context) (uint, bool) {
	if ctx == nil {
		return 0, false
	}
	id, ok := ctx.Value("user_id").(uint)
	return id, ok && id > 0
}

func isRootAdministrator(r repo.Repo, userID uint) (bool, error) {
	root, err := r.GetRootOrganization()
	if err != nil || root == nil {
		return false, err
	}
	binding, err := r.GetIAMRoleBinding(root.ID, userID)
	if err != nil || binding == nil || binding.Role == nil {
		return false, err
	}
	return binding.Role.Code == model.IAMRoleAdmin && binding.InheritChildren, nil
}

func visibleResourceIDs(ctx context.Context, r repo.Repo, resourceType string) ([]uint64, bool, error) {
	userID, scoped := actorUserID(ctx)
	if !scoped {
		return nil, false, nil
	}
	admin, err := isRootAdministrator(r, userID)
	if err != nil || admin {
		return nil, !admin, err
	}
	ids, err := r.ListIAMResourceIDsForSubject(resourceType, model.IAMSubjectUser, userID)
	return ids, true, err
}

func requireVisibleResource(ctx context.Context, r repo.Repo, resourceType string, resourceID uint64) error {
	ids, scoped, err := visibleResourceIDs(ctx, r, resourceType)
	if err != nil || !scoped {
		return err
	}
	for _, id := range ids {
		if id == resourceID {
			return nil
		}
	}
	return notFound("RESOURCE_NOT_FOUND", "资源不存在", nil)
}

func claimResource(ctx context.Context, r repo.Repo, resourceType string, resourceID uint64) error {
	userID, scoped := actorUserID(ctx)
	if !scoped {
		return nil
	}
	root, err := r.GetRootOrganization()
	if err != nil {
		return err
	}
	if root == nil {
		return notFound("ROOT_ORGANIZATION_NOT_FOUND", "根组织不存在", nil)
	}
	if err := r.UpsertIAMResourceRelation(&model.IAMResourceRelation{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Relation:     model.IAMRelationOwner,
		SubjectType:  model.IAMSubjectUser,
		SubjectID:    userID,
		CreatedBy:    userID,
	}); err != nil {
		return err
	}
	return r.UpsertIAMResourceRelation(&model.IAMResourceRelation{
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Relation:     model.IAMRelationBelongsTo,
		SubjectType:  model.IAMSubjectOrganization,
		SubjectID:    root.ID,
		CreatedBy:    userID,
	})
}

func intersectResourceIDs(left, right []uint64) []uint64 {
	if len(left) == 0 || len(right) == 0 {
		return nil
	}
	wanted := make(map[uint64]struct{}, len(right))
	for _, id := range right {
		wanted[id] = struct{}{}
	}
	result := make([]uint64, 0, len(left))
	for _, id := range left {
		if _, ok := wanted[id]; ok {
			result = append(result, id)
		}
	}
	return result
}
