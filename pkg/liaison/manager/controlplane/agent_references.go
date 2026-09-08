package controlplane

import "context"

// ResolveAgentResource uses the same ownership scope as resource lists, and
// returns only a name. A reference never grants access or exposes credentials.
func (cp *controlPlane) ResolveAgentResource(ctx context.Context, userID uint, resourceType string, resourceID uint64) (string, error) {
	if userID == 0 || resourceID == 0 {
		return "", notFound("RESOURCE_NOT_FOUND", "资源不存在", nil)
	}
	ctx = context.WithValue(ctx, "user_id", userID)
	if err := requireVisibleResource(ctx, cp.repo, resourceType, resourceID); err != nil {
		return "", err
	}
	switch resourceType {
	case resourceConnector:
		item, err := cp.repo.GetEdge(resourceID)
		if err != nil {
			return "", err
		}
		return item.Name, nil
	case resourceDevice:
		item, err := cp.repo.GetDeviceByID(uint(resourceID))
		if err != nil {
			return "", err
		}
		return item.Name, nil
	case resourceApplication:
		item, err := cp.repo.GetApplicationByID(uint(resourceID))
		if err != nil {
			return "", err
		}
		return item.Name, nil
	}
	return "", notFound("RESOURCE_NOT_FOUND", "资源不存在", nil)
}
