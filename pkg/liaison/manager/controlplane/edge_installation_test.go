package controlplane

import (
	"context"
	"errors"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type installationRepo struct {
	repo.Repo
	visible []uint64
	edge    model.Edge
}

func (r *installationRepo) GetRootOrganization() (*model.Organization, error) {
	return &model.Organization{Model: gorm.Model{ID: 1}}, nil
}
func (r *installationRepo) GetIAMRoleBinding(uint, uint) (*model.IAMRoleBinding, error) {
	return nil, nil
}
func (r *installationRepo) ListIAMResourceIDsForSubject(string, model.IAMSubjectType, uint) ([]uint64, error) {
	return r.visible, nil
}
func (r *installationRepo) GetEdge(id uint64) (*model.Edge, error) {
	if id != uint64(r.edge.ID) {
		return nil, gorm.ErrRecordNotFound
	}
	return &r.edge, nil
}

type installationFrontier struct {
	frontierbound.FrontierBound
	calls   int
	err     error
	version int
}

func (f *installationFrontier) EdgeInstallationStatus(ctx context.Context, id uint64) (proto.InstallationStatus, error) {
	f.calls++
	if _, ok := ctx.Deadline(); !ok {
		return proto.InstallationStatus{}, errors.New("no deadline")
	}
	return proto.InstallationStatus{Version: f.version, OwnershipVerified: true, CanUninstall: true}, f.err
}

func TestInstallationPreflightAuthorizationAndCompatibility(t *testing.T) {
	for _, test := range []string{"success", "anonymous", "denied", "invisible", "offline", "old edge", "unknown version"} {
		t.Run(test, func(t *testing.T) {
			r := &installationRepo{visible: []uint64{7}, edge: model.Edge{Model: gorm.Model{ID: 7}, Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}}
			f := &installationFrontier{version: 1}
			cp := &controlPlane{repo: r, frontierBound: f}
			cp.authorizeFeature = func(_ context.Context, feature string) error {
				require.Equal(t, iam.FeatureConnectorUninstall, feature)
				if test == "denied" {
					return iam.ErrForbidden
				}
				return nil
			}
			ctx := context.WithValue(context.Background(), "user_id", uint(2))
			if test == "anonymous" {
				ctx = context.Background()
			}
			if test == "invisible" {
				r.visible = nil
			}
			if test == "offline" {
				r.edge.Online = model.EdgeOnlineStatusOffline
			}
			if test == "old edge" {
				f.err = errors.New("RPC missing")
			}
			if test == "unknown version" {
				f.version = 2
			}
			status, err := cp.GetEdgeInstallation(ctx, 7)
			if test == "anonymous" || test == "denied" || test == "invisible" {
				require.Error(t, err)
				require.Zero(t, f.calls)
				return
			}
			require.NoError(t, err)
			require.Equal(t, test == "success", status.CanUninstall)
			if test == "offline" {
				require.Zero(t, f.calls)
				require.Equal(t, "connector_offline", status.Reason)
			} else {
				require.Equal(t, 1, f.calls)
			}
			if test == "success" {
				require.True(t, status.OwnershipVerified)
				require.Empty(t, status.Reason)
			}
			if test == "old edge" {
				require.Equal(t, "preflight_unavailable", status.Reason)
			}
			if test == "unknown version" {
				require.Equal(t, "edge_upgrade_required", status.Reason)
			}
		})
	}
}
