package policy

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

const agentSessionResource = "agent_sessions"

// AuthorizeFunc revalidates authorization at invocation time. This is
// deliberately separate from session creation authorization: a long-running
// Agent session must observe role and organization membership changes.
type AuthorizeFunc func(ctx context.Context, principal tool.Principal, resource, action string) error

type AuthorizationPolicy struct {
	authorize AuthorizeFunc
}

func NewAuthorizationPolicy(authorize AuthorizeFunc) (*AuthorizationPolicy, error) {
	if authorize == nil {
		return nil, errors.New("agent tool policy requires an authorizer")
	}
	return &AuthorizationPolicy{authorize: authorize}, nil
}

func (policy *AuthorizationPolicy) Evaluate(ctx context.Context, invocation tool.ToolInvocation, descriptor tool.ToolDescriptor) (tool.Decision, error) {
	principal := invocation.Binding.Principal
	if principal.UserID == 0 || principal.OrganizationID == 0 {
		return tool.Decision{Effect: tool.DecisionDeny, Reason: "missing principal binding"}, nil
	}
	resource := agentSessionResource
	if invocation.Binding.SessionKind == tool.SessionManagement {
		resource = "management_agent_sessions"
	}
	if err := policy.authorize(ctx, principal, resource, "use"); err != nil {
		return tool.Decision{}, err
	}
	if permission := descriptor.Permission; permission != nil {
		if permission.Resource == "" || permission.Action == "" {
			return tool.Decision{Effect: tool.DecisionDeny, Reason: "invalid resource permission"}, nil
		}
		if err := policy.authorize(ctx, principal, permission.Resource, permission.Action); err != nil {
			return tool.Decision{}, err
		}
	}

	decision := tool.Decision{
		Effect:          tool.DecisionAllow,
		Risk:            descriptor.Risk,
		NormalizedInput: append(json.RawMessage(nil), invocation.Call.Input...),
	}
	if descriptor.Approval == tool.ApprovalAlways ||
		(descriptor.Approval == tool.ApprovalByPolicy && descriptor.Risk >= tool.RiskMedium) {
		decision.Effect = tool.DecisionRequireApproval
		decision.Reason = "operation requires explicit approval"
		decision.ApprovalPolicy = "agent-medium-risk-v1"
	}
	return decision, nil
}
