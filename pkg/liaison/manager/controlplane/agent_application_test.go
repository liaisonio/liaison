package controlplane

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"strings"
	"testing"
)

type applicationAgentRepo struct {
	*entryAgentRepo
	app   model.AgentApplication
	saved int
}

func (r *applicationAgentRepo) GetAgentApplication(_ context.Context, owner uint, id string) (*model.AgentApplication, error) {
	if owner != r.app.OwnerID || id != r.app.ID {
		return nil, gorm.ErrRecordNotFound
	}
	copy := r.app
	return &copy, nil
}
func (r *applicationAgentRepo) SaveAgentApplication(_ context.Context, row *model.AgentApplication, _ bool) error {
	r.saved++
	r.app = *row
	return nil
}
func (r *applicationAgentRepo) SaveAgentAccess(_ context.Context, row *model.AgentAccess, _ bool) error {
	r.saved++
	r.entry = row
	return nil
}
func (r *applicationAgentRepo) DeleteAgentApplication(context.Context, uint, string) error {
	return dao.ErrAgentApplicationInUse
}

type installationAgentFrontier struct{ *agentFrontier }

type claudeInstallationFrontier struct{ *agentFrontier }

func (f *claudeInstallationFrontier) EdgeAgent(ctx context.Context, id uint64, req proto.EdgeAgentRPCRequest) (proto.EdgeAgentResult, error) {
	result, err := f.agentFrontier.EdgeAgent(ctx, id, req)
	result.Installations = []proto.AgentInstallation{{ID: strings.Repeat("b", 32), Kind: "claude"}}
	return result, err
}

func TestClaudeApplicationRequiresMatchingDiscoveryAndOwnedBinding(t *testing.T) {
	r := &applicationAgentRepo{entryAgentRepo: &entryAgentRepo{agentRepo: &agentRepo{installationRepo: &installationRepo{edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}, owner: 2}}}
	f := &claudeInstallationFrontier{&agentFrontier{}}
	cp := &controlPlane{repo: r, frontierBound: f, authorizeFeature: func(context.Context, string) error { return nil }}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	input := AgentApplicationInput{Name: "Claude Code", Kind: "codex", EdgeID: 7, InstallationID: strings.Repeat("b", 32)}
	_, err := cp.SaveAgentApplication(ctx, "", input)
	require.Error(t, err, "same installation ID with the wrong kind must not bind")
	require.Zero(t, r.saved)
	input.Kind = "claude"
	app, err := cp.SaveAgentApplication(ctx, "", input)
	require.NoError(t, err)
	access := AgentAccessInput{Name: "Claude access", Kind: "claude", EdgeID: 7, InstallationID: input.InstallationID, Project: "/project"}
	_, err = cp.SaveAgentAccess(ctx, "", access)
	require.Error(t, err, "Claude access requires a registered application")
	access.ApplicationID = app.ID
	row, err := cp.SaveAgentAccess(ctx, "", access)
	require.NoError(t, err)
	require.Equal(t, "claude", row.Kind)
	require.Equal(t, app.ID, row.ApplicationID)
	_, err = cp.SaveAgentAccess(context.WithValue(ctx, "user_id", uint(3)), "", access)
	require.Error(t, err)
	access.Kind = "codex"
	_, err = cp.SaveAgentAccess(ctx, "", access)
	require.Error(t, err)
	require.Equal(t, 2, r.saved)
}

func (f *installationAgentFrontier) EdgeAgent(ctx context.Context, id uint64, req proto.EdgeAgentRPCRequest) (proto.EdgeAgentResult, error) {
	result, err := f.agentFrontier.EdgeAgent(ctx, id, req)
	result.Installations = []proto.AgentInstallation{{ID: strings.Repeat("b", 32), Kind: "codex"}}
	return result, err
}

func TestAgentApplicationOwnershipAndAccessBinding(t *testing.T) {
	r := &applicationAgentRepo{entryAgentRepo: &entryAgentRepo{agentRepo: &agentRepo{installationRepo: &installationRepo{edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}, owner: 2}}, app: model.AgentApplication{ID: strings.Repeat("a", 32), OwnerID: 2, EdgeID: 7, Kind: "codex", InstallationID: strings.Repeat("b", 32), Name: "Codex"}}
	f := &installationAgentFrontier{&agentFrontier{}}
	cp := &controlPlane{repo: r, frontierBound: f, authorizeFeature: func(context.Context, string) error { return nil }}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	input := AgentAccessInput{ApplicationID: r.app.ID, Name: "Access", Project: "/project"}
	row, err := cp.SaveAgentAccess(ctx, "", input)
	require.NoError(t, err)
	require.Equal(t, r.app.InstallationID, row.InstallationID)
	require.Equal(t, r.app.ID, row.ApplicationID)
	require.Zero(t, f.calls)
	input.EdgeID = 8
	_, err = cp.SaveAgentAccess(ctx, "", input)
	require.Error(t, err)
	require.Equal(t, 1, r.saved)
	input.EdgeID = 0
	_, err = cp.SaveAgentAccess(context.WithValue(ctx, "user_id", uint(3)), "", input)
	require.Error(t, err)
	_, err = cp.SaveAgentApplication(ctx, r.app.ID, AgentApplicationInput{Name: "Rename", InstallationID: "different"})
	require.Error(t, err)
	_, err = cp.SaveAgentApplication(ctx, r.app.ID, AgentApplicationInput{Name: "Rename"})
	require.NoError(t, err)
	require.Zero(t, f.calls)
	require.Error(t, cp.DeleteAgentApplication(ctx, r.app.ID))
	_, err = cp.SaveAgentApplication(ctx, "", AgentApplicationInput{Name: "New", Kind: "codex", EdgeID: 7, InstallationID: strings.Repeat("c", 32)})
	require.Error(t, err)
	_, err = cp.SaveAgentApplication(ctx, "", AgentApplicationInput{Name: "New", Kind: "codex", EdgeID: 7, InstallationID: strings.Repeat("b", 32)})
	require.NoError(t, err)
	require.Equal(t, 2, f.calls)
}
