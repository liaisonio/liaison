package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/management"
	"github.com/liaisonio/liaison/pkg/liaison/manager/aigateway"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// ManagementLLMOverview is a credential-free projection for the home Agent.
// The tool rechecks organization permissions; this layer enforces resource scope.
func (cp *controlPlane) ManagementLLMOverview(ctx context.Context, id uint, hours int) (*management.LLMOverview, error) {
	user, ok := actorUserID(ctx)
	if !ok || user == 0 {
		return nil, iam.ErrForbidden
	}
	if hours != 1 && hours != 6 && hours != 24 && hours != 168 && hours != 720 {
		return nil, ErrAIInvalid
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceAccess, uint64(id)); err != nil {
		return nil, err
	}
	p, err := cp.repo.GetProxyByID(id)
	if err != nil {
		return nil, err
	}
	if p.AccessProtocol != model.AccessProtocolAI {
		return nil, ErrAIInvalid
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceApplication, uint64(p.ApplicationID)); err != nil {
		return nil, err
	}
	app, err := cp.repo.GetApplicationByID(p.ApplicationID)
	if err != nil {
		return nil, err
	}
	if !model.IsLLMApplicationType(app.ApplicationType) {
		return nil, ErrAIInvalid
	}
	upstream, err := cp.repo.GetAIApplication(ctx, p.ApplicationID)
	if err != nil {
		return nil, err
	}
	access, err := cp.repo.GetAIAccess(ctx, id)
	if err != nil {
		return nil, err
	}
	var models map[string]string
	if err := json.Unmarshal([]byte(access.Models), &models); err != nil {
		return nil, err
	}
	usage, err := cp.repo.GetLLMTokenUsage(ctx, id, user, time.Now().UTC().Add(-time.Duration(hours)*time.Hour))
	if err != nil {
		return nil, err
	}
	return &management.LLMOverview{Name: p.Name, Enabled: access.Enabled && p.Status == model.ProxyStatusRunning, Models: aigateway.ModelAliases(models), ClientProtocols: aigateway.NativeClientProtocols(upstream.Protocol), Usage: usage.Summary, Since: usage.Since, Path: fmt.Sprintf("/ai/%d", id)}, nil
}
