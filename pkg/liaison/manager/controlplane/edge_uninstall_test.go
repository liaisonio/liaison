package controlplane

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
	"time"
)

type uninstallFrontier struct {
	frontierbound.FrontierBound
	command proto.UninstallCommand
	calls   int
	err     error
}

func (f *uninstallFrontier) EdgeInstallationStatus(context.Context, uint64) (proto.InstallationStatus, error) {
	return proto.InstallationStatus{RuntimeID: strings.Repeat("e", 32), Version: 1, OwnershipVerified: true, CanUninstall: true, InstanceID: strings.Repeat("a", 32)}, nil
}
func (f *uninstallFrontier) UninstallEdge(_ context.Context, _ uint64, command proto.UninstallCommand) error {
	f.calls++
	f.command = command
	return f.err
}

func TestUninstallAuthorizationTaskBindingAndIdempotency(t *testing.T) {
	cp, r := newTestControlPlane(t)
	edge, _ := createTestEdgeApplication(t, r)
	id := uint64(edge.ID)
	grantTestResourceToUsers(t, r, "connector", edge.ID, 11)
	f := &uninstallFrontier{}
	cp.frontierBound = f
	ctx := context.WithValue(context.Background(), "user_id", uint(11))
	ctx = context.WithValue(ctx, "user_email", "operator@example.test")
	instance := strings.Repeat("a", 32)
	for _, bad := range []context.Context{context.Background(), context.WithValue(ctx, "user_id", uint(12))} {
		_, err := cp.CreateEdgeUninstall(bad, id, edge.Name, instance)
		require.Error(t, err)
	}
	_, err := cp.CreateEdgeUninstall(ctx, id, "wrong name", instance)
	require.Error(t, err)
	_, err = cp.CreateEdgeUninstall(ctx, id, edge.Name, "wrong instance")
	require.Error(t, err)
	require.Zero(t, f.calls)
	task, err := cp.CreateEdgeUninstall(ctx, id, edge.Name, instance)
	require.NoError(t, err)
	require.Equal(t, "accepted", task.Status)
	require.Equal(t, 1, f.calls)
	again, err := cp.CreateEdgeUninstall(ctx, id, edge.Name, instance)
	require.NoError(t, err)
	require.Equal(t, task.ID, again.ID)
	require.Equal(t, 1, f.calls)
	require.NotEqual(t, task.TokenHash, f.command.CallbackToken)
	require.ErrorIs(t, cp.ReportEdgeUninstall(ctx, task.ID, strings.Repeat("c", 64), "completed"), iam.ErrForbidden)
	require.ErrorIs(t, cp.ReportEdgeUninstall(ctx, strings.Repeat("d", 32), f.command.CallbackToken, "completed"), iam.ErrForbidden)
	require.Error(t, cp.ReportEdgeUninstall(ctx, task.ID, f.command.CallbackToken, "accepted"))
	require.NoError(t, cp.ReportEdgeUninstall(ctx, task.ID, f.command.CallbackToken, "running"))
	require.NoError(t, cp.ReportEdgeUninstall(ctx, task.ID, f.command.CallbackToken, "completed"))
	require.NoError(t, cp.ReportEdgeUninstall(ctx, task.ID, f.command.CallbackToken, "completed"))
	require.Error(t, cp.ReportEdgeUninstall(ctx, task.ID, f.command.CallbackToken, "running"))
	_, err = cp.GetEdgeUninstall(context.WithValue(ctx, "user_id", uint(12)), id, task.ID)
	require.Error(t, err)
	active, err := cp.GetEdgeUninstall(ctx, id, "active")
	require.NoError(t, err)
	require.Nil(t, active)
}

func TestUninstallAmbiguousDeliveryNeverRetries(t *testing.T) {
	cp, r := newTestControlPlane(t)
	edge, _ := createTestEdgeApplication(t, r)
	id := uint64(edge.ID)
	grantTestResourceToUsers(t, r, "connector", edge.ID, 11)
	f := &uninstallFrontier{err: errors.New("connection closed after dispatch")}
	cp.frontierBound = f
	ctx := context.WithValue(context.Background(), "user_id", uint(11))
	ctx = context.WithValue(ctx, "user_email", "operator@example.test")
	task, err := cp.CreateEdgeUninstall(ctx, id, edge.Name, strings.Repeat("a", 32))
	require.NoError(t, err)
	require.Equal(t, "unknown", task.Status)
	_, err = cp.CreateEdgeUninstall(ctx, id, edge.Name, task.InstanceID)
	require.NoError(t, err)
	require.Equal(t, 1, f.calls)
	// A late signed result resolves uncertainty; dropping a tunnel never does.
	require.NoError(t, cp.ReportEdgeUninstall(ctx, task.ID, f.command.CallbackToken, "completed"))
	require.WithinDuration(t, time.Now().Add(5*time.Minute), task.ExpiresAt, time.Second)
}
