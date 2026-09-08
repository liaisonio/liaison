package policy

import (
	"context"
	"errors"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/stretchr/testify/require"
)

func TestAuthorizationPolicy_RechecksUseAndRequiresApprovalForMediumRisk(t *testing.T) {
	var authorized tool.Principal
	policy, err := NewAuthorizationPolicy(func(_ context.Context, principal tool.Principal, resource, action string) error {
		authorized = principal
		require.Equal(t, "agent_sessions", resource)
		require.Equal(t, "use", action)
		return nil
	})
	require.NoError(t, err)
	invocation := tool.ToolInvocation{
		Call:    tool.ToolCall{Input: []byte(`{"command":"uptime"}`)},
		Binding: tool.ToolBinding{Principal: tool.Principal{UserID: 7, OrganizationID: 9}},
	}
	decision, err := policy.Evaluate(context.Background(), invocation, tool.ToolDescriptor{
		Risk: tool.RiskMedium, Approval: tool.ApprovalByPolicy,
	})
	require.NoError(t, err)
	require.Equal(t, invocation.Binding.Principal, authorized)
	require.Equal(t, tool.DecisionRequireApproval, decision.Effect)
	require.Equal(t, "agent-medium-risk-v1", decision.ApprovalPolicy)
}

func TestAuthorizationPolicy_AllowsReadOnlyAndRejectsMissingPrincipal(t *testing.T) {
	policy, err := NewAuthorizationPolicy(func(context.Context, tool.Principal, string, string) error { return nil })
	require.NoError(t, err)
	decision, err := policy.Evaluate(context.Background(), tool.ToolInvocation{
		Call:    tool.ToolCall{Input: []byte(`{}`)},
		Binding: tool.ToolBinding{Principal: tool.Principal{UserID: 1, OrganizationID: 2}},
	}, tool.ToolDescriptor{Risk: tool.RiskReadOnly, Approval: tool.ApprovalNever})
	require.NoError(t, err)
	require.Equal(t, tool.DecisionAllow, decision.Effect)

	decision, err = policy.Evaluate(context.Background(), tool.ToolInvocation{}, tool.ToolDescriptor{})
	require.NoError(t, err)
	require.Equal(t, tool.DecisionDeny, decision.Effect)
}

func TestAuthorizationPolicy_PropagatesRevocation(t *testing.T) {
	revoked := errors.New("revoked")
	policy, err := NewAuthorizationPolicy(func(context.Context, tool.Principal, string, string) error { return revoked })
	require.NoError(t, err)
	_, err = policy.Evaluate(context.Background(), tool.ToolInvocation{
		Binding: tool.ToolBinding{Principal: tool.Principal{UserID: 1, OrganizationID: 2}},
	}, tool.ToolDescriptor{})
	require.ErrorIs(t, err, revoked)
}

func TestAuthorizationPolicy_RechecksResourcePermissionOnEveryInvocation(t *testing.T) {
	var checks []string
	revoked := false
	denied := errors.New("resource permission revoked")
	p, err := NewAuthorizationPolicy(func(_ context.Context, principal tool.Principal, resource, action string) error {
		require.Equal(t, uint(7), principal.UserID)
		checks = append(checks, resource+":"+action)
		if resource == "connectors" && revoked {
			return denied
		}
		return nil
	})
	require.NoError(t, err)
	descriptor := tool.ToolDescriptor{Permission: &tool.ResourcePermission{Resource: "connectors", Action: "update"}, Risk: tool.RiskMedium, Approval: tool.ApprovalByPolicy}
	call := tool.ToolInvocation{Binding: tool.ToolBinding{SessionKind: tool.SessionManagement, Principal: tool.Principal{UserID: 7, OrganizationID: 9}}}
	decision, err := p.Evaluate(context.Background(), call, descriptor)
	require.NoError(t, err)
	require.Equal(t, tool.DecisionRequireApproval, decision.Effect)
	revoked = true
	_, err = p.Evaluate(context.Background(), call, descriptor)
	require.ErrorIs(t, err, denied)
	require.Equal(t, []string{"management_agent_sessions:use", "connectors:update", "management_agent_sessions:use", "connectors:update"}, checks)
}
