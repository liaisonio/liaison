package controlplane

import (
	"context"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentFilesRequireAccessOwnershipAndFilePermissions(t *testing.T) {
	id := strings.Repeat("a", 32)
	repo := &entryAgentRepo{agentRepo: &agentRepo{installationRepo: &installationRepo{edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}, owner: 2}, entry: &model.AgentAccess{ID: id, OwnerID: 2, EdgeID: 7, Project: "/project"}}
	frontier := &agentFrontier{}
	deny := ""
	cp := &controlPlane{repo: repo, frontierBound: frontier, authorizeFeature: func(_ context.Context, feature string) error {
		if feature == deny {
			return iam.ErrForbidden
		}
		return nil
	}}
	ctx := context.WithValue(context.Background(), "user_id", uint(2))
	req := proto.EdgeAgentRequest{EdgeID: 7, AccessID: id, SessionID: strings.Repeat("s", 32), Action: "file_begin", File: &proto.AgentFileOperation{Name: "test.txt", Size: 1}}
	deny = iam.FeatureFilesUpload
	_, err := cp.EdgeAgent(ctx, req)
	require.ErrorIs(t, err, iam.ErrForbidden)
	require.Zero(t, frontier.calls)
	req.Action = "file_read"
	req.File = &proto.AgentFileOperation{Path: "test.txt"}
	deny = iam.FeatureFilesRead
	_, err = cp.EdgeAgent(ctx, req)
	require.ErrorIs(t, err, iam.ErrForbidden)
	require.Zero(t, frontier.calls)
	deny = ""
	repo.entry.OwnerID = 3
	_, err = cp.EdgeAgent(ctx, req)
	require.Error(t, err)
	require.Zero(t, frontier.calls)
	repo.entry.OwnerID = 2
	_, err = cp.EdgeAgent(ctx, req)
	require.NoError(t, err)
	require.Equal(t, 1, frontier.calls)
	frontier.afterCall = func() { deny = iam.FeatureFilesRead }
	_, err = cp.EdgeAgent(ctx, req)
	require.ErrorIs(t, err, iam.ErrForbidden, "Revoke read permission while a file chunk is in flight")
}
