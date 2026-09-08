package management

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

type testIAM struct {
	active bool
	denied bool
	checks []string
}

func (i *testIAM) GetUserByID(id uint) (*model.User, error) {
	u := &model.User{Status: model.UserStatusActive}
	u.ID = id
	if !i.active {
		u.Status = "disabled"
	}
	return u, nil
}
func (i *testIAM) RequireOrganizationResourcePermission(_ *model.User, _ uint, r, a string) error {
	i.checks = append(i.checks, r+":"+a)
	if i.denied {
		return errors.New("denied")
	}
	return nil
}

type testCP struct {
	ControlPlane
	t     *testing.T
	calls int
}

func (c *testCP) ListEdges(ctx context.Context, r *v1.ListEdgesRequest) (*v1.ListEdgesResponse, error) {
	c.calls++
	require.Equal(c.t, uint(7), ctx.Value("user_id"))
	require.Equal(c.t, uint(7), ctx.Value("user").(*model.User).ID)
	require.Equal(c.t, int32(20), r.PageSize)
	return &v1.ListEdgesResponse{Data: &v1.Edges{Total: 1, Edges: []*v1.Edge{{Id: 1, Name: "own", Description: "secret-description", Device: &v1.Device{Name: "hidden-device"}}}}}, nil
}

func TestManagementExecutorIdentityRevocationAndWhitelist(t *testing.T) {
	iam := &testIAM{active: true}
	cp := &testCP{t: t}
	source, err := NewSource(cp, iam)
	require.NoError(t, err)
	registrations, err := source.Snapshot(context.Background())
	require.NoError(t, err)
	f := registrations[0].Factory
	for _, binding := range []tool.ToolBinding{{}, {SessionKind: tool.SessionAccess, Principal: tool.Principal{UserID: 7, OrganizationID: 2}}, {SessionKind: tool.SessionManagement, Principal: tool.Principal{UserID: 7}}} {
		_, err := f.Bind(context.Background(), binding)
		require.ErrorIs(t, err, tool.ErrPolicyDenied)
	}
	e, err := f.Bind(context.Background(), tool.ToolBinding{SessionKind: tool.SessionManagement, Principal: tool.Principal{UserID: 7, OrganizationID: 2}})
	require.NoError(t, err)
	// A stale/admin context must not override the bound authenticated user.
	ctx := context.WithValue(context.Background(), "user_id", uint(1))
	result, err := e.Execute(ctx, json.RawMessage(`{}`))
	require.NoError(t, err)
	require.Contains(t, string(result.Content), "own")
	require.NotContains(t, string(result.Content), "hidden-device")
	require.NotContains(t, string(result.Content), "secret-description")
	require.Equal(t, []string{"management_agent_sessions:use", "connectors:read"}, iam.checks)
	for _, input := range []string{`null`, `[]`, `{"user_id":1}`, `{"page_size":-1}`, `{"page_size":51}`, `{"page":-1}`, `{} {}`, `{"id":"1"}`} {
		_, err = e.Execute(ctx, json.RawMessage(input))
		require.Error(t, err)
	}
	require.Equal(t, 1, cp.calls)
	iam.denied = true
	_, err = e.Execute(ctx, json.RawMessage(`{}`))
	require.Error(t, err)
	iam.denied = false
	iam.active = false
	_, err = e.Execute(ctx, json.RawMessage(`{}`))
	require.ErrorIs(t, err, tool.ErrPolicyDenied)
	require.Equal(t, 1, cp.calls)
}
