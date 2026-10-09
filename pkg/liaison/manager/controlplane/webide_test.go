package controlplane

import (
	"context"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type webIDETestRepo struct {
	repo.Repo
	access         model.WebIDEAccess
	app            model.WebIDEApplication
	connectorOwner uint
	filter         string
}

func (r *webIDETestRepo) WebIDEAccesses(_ context.Context, owner uint, _, _ int, names ...string) ([]model.WebIDEAccess, int64, error) {
	if len(names) > 0 {
		r.filter = names[0]
	}
	if r.access.OwnerID != owner {
		return nil, 0, nil
	}
	return []model.WebIDEAccess{r.access}, 1, nil
}
func (r *webIDETestRepo) GetEdge(id uint64) (*model.Edge, error) {
	if id != r.app.EdgeID {
		return nil, gorm.ErrRecordNotFound
	}
	return &model.Edge{Name: "Device", Online: model.EdgeOnlineStatusOnline, Status: model.EdgeStatusRunning}, nil
}

func TestWebIDEListSummariesRespectConnectorOwnership(t *testing.T) {
	r := &webIDETestRepo{app: model.WebIDEApplication{ID: strings.Repeat("b", 32), OwnerID: 2, EdgeID: 7, Name: "Application", Mode: "managed"}, connectorOwner: 2}
	r.access = model.WebIDEAccess{ID: strings.Repeat("a", 32), OwnerID: 2, ApplicationID: r.app.ID, Name: "project"}
	cp := &controlPlane{repo: r}
	ctx := context.WithValue(t.Context(), "user_id", uint(2))
	list, err := cp.WebIDEAccesses(ctx, 1, 20, "project")
	require.NoError(t, err)
	require.Equal(t, "project", r.filter)
	require.Len(t, list.Items, 1)
	require.Equal(t, "Application", list.Items[0].ApplicationName)
	require.Equal(t, "Device", list.Items[0].ConnectorName)
	require.True(t, list.Items[0].ConnectorOnline)
	r.connectorOwner = 3
	list, err = cp.WebIDEAccesses(ctx, 1, 20)
	require.NoError(t, err)
	require.False(t, list.Items[0].ConnectorAvailable)
	require.Empty(t, list.Items[0].ConnectorName)
}

func (r *webIDETestRepo) GetWebIDEAccess(_ context.Context, owner uint, id string) (*model.WebIDEAccess, error) {
	if r.access.ID != id || r.access.OwnerID != owner {
		return nil, gorm.ErrRecordNotFound
	}
	v := r.access
	return &v, nil
}
func (r *webIDETestRepo) GetWebIDEApplication(_ context.Context, owner uint, id string) (*model.WebIDEApplication, error) {
	if r.app.ID != id || r.app.OwnerID != owner {
		return nil, gorm.ErrRecordNotFound
	}
	v := r.app
	return &v, nil
}
func (r *webIDETestRepo) ListIAMResourceRelations(_ string, edge uint64) ([]*model.IAMResourceRelation, error) {
	if edge != r.app.EdgeID {
		return nil, nil
	}
	return []*model.IAMResourceRelation{{SubjectType: model.IAMSubjectUser, SubjectID: r.connectorOwner, Relation: model.IAMRelationOwner}}, nil
}

type webIDETestFrontier struct {
	frontierbound.FrontierBound
	requests  []proto.WebIDERequest
	instances []proto.WebIDEInstance
}

func (f *webIDETestFrontier) WebIDE(_ context.Context, _ uint64, q proto.WebIDERequest) (proto.WebIDEResult, error) {
	f.requests = append(f.requests, q)
	return proto.WebIDEResult{Version: 1, Status: "ok", Instances: f.instances}, nil
}

func (r *webIDETestRepo) SaveWebIDEAccess(_ context.Context, row *model.WebIDEAccess, _ bool) error {
	r.access = *row
	return nil
}

func TestWebIDEOrphanInstanceRecovery(t *testing.T) {
	r := &webIDETestRepo{app: model.WebIDEApplication{ID: strings.Repeat("b", 32), OwnerID: 2, EdgeID: 7, Mode: "managed", InstallationID: "installation", Name: "Development"}, connectorOwner: 2}
	instance := proto.WebIDEInstance{ID: strings.Repeat("c", 24), AccessID: strings.Repeat("a", 32), InstallationID: "installation", Project: "/workspace/project", Status: "running"}
	f := &webIDETestFrontier{instances: []proto.WebIDEInstance{instance, {ID: "other", InstallationID: "other"}}}
	cp := &controlPlane{repo: r, frontierBound: f}
	ctx := context.WithValue(t.Context(), "user_id", uint(2))
	rows, err := cp.WebIDEApplicationInstances(ctx, r.app.ID)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	require.False(t, rows[0].Linked)
	f.instances[0].ApplicationID = strings.Repeat("f", 32)
	hidden, err := cp.WebIDEApplicationInstances(ctx, r.app.ID)
	require.NoError(t, err)
	require.Empty(t, hidden)
	_, err = cp.WebIDEApplicationInstanceAction(ctx, r.app.ID, instance.ID, "recover")
	require.Error(t, err)
	f.instances[0].ApplicationID = r.app.ID
	_, err = cp.WebIDEApplicationInstanceAction(ctx, r.app.ID, "other", "recover")
	require.Error(t, err)
	_, err = cp.WebIDEApplicationInstanceAction(context.WithValue(ctx, "user_id", uint(3)), r.app.ID, instance.ID, "recover")
	require.Error(t, err)
	recovered, err := cp.WebIDEApplicationInstanceAction(ctx, r.app.ID, instance.ID, "recover")
	require.NoError(t, err)
	require.True(t, recovered.Linked)
	require.Equal(t, instance.AccessID, r.access.ID)
	require.Equal(t, instance.Project, r.access.Project)
	require.True(t, r.access.Enabled)
	for _, q := range f.requests {
		require.Equal(t, "instances", q.Action)
		require.Equal(t, "2", q.OwnerID)
	}
	_, err = cp.WebIDEApplicationInstanceAction(ctx, r.app.ID, instance.ID, "stop")
	require.NoError(t, err)
	q := f.requests[len(f.requests)-1]
	require.Equal(t, "stop", q.Action)
	require.Equal(t, instance.AccessID, q.AccessID)
	require.Equal(t, instance.ID, q.InstanceID)
	require.Equal(t, "2", q.OwnerID)
	_, err = cp.WebIDEApplicationInstanceAction(ctx, r.app.ID, instance.ID, "delete")
	require.Error(t, err)
	r.access.ApplicationID = "another-app"
	rows, err = cp.WebIDEApplicationInstances(ctx, r.app.ID)
	require.NoError(t, err)
	require.Empty(t, rows)
}

func TestWebIDEDiagnosticRequestIDForwarded(t *testing.T) {
	r := &webIDETestRepo{app: model.WebIDEApplication{ID: strings.Repeat("b", 32), OwnerID: 2, EdgeID: 7}, connectorOwner: 2}
	f := &webIDETestFrontier{}
	cp := &controlPlane{repo: r, frontierBound: f}
	ctx := context.WithValue(t.Context(), "user_id", uint(2))
	ctx = proto.WithWebIDERequestID(ctx, strings.Repeat("a", 32))
	_, err := cp.WebIDEControl(ctx, 7, proto.WebIDERequest{Action: "instances", RequestID: "forged"})
	require.NoError(t, err)
	require.Equal(t, strings.Repeat("a", 32), f.requests[0].RequestID)
}

func TestWebIDEDisabledAccessCanOnlyBeStoppedByOwner(t *testing.T) {
	r := &webIDETestRepo{
		access:         model.WebIDEAccess{ID: strings.Repeat("a", 32), OwnerID: 2, ApplicationID: strings.Repeat("b", 32)},
		app:            model.WebIDEApplication{ID: strings.Repeat("b", 32), OwnerID: 2, EdgeID: 7, Mode: "managed", InstallationID: "installation"},
		connectorOwner: 2,
	}
	f := &webIDETestFrontier{}
	cp := &controlPlane{repo: r, frontierBound: f}
	ctx := context.WithValue(t.Context(), "user_id", uint(2))
	_, _, err := cp.WebIDETarget(ctx, r.access.ID)
	require.ErrorIs(t, err, iam.ErrForbidden)
	_, err = cp.WebIDERuntime(ctx, r.access.ID, proto.WebIDERequest{Action: "start", Project: "/project"})
	require.ErrorIs(t, err, iam.ErrForbidden)
	_, err = cp.OpenWebIDEStream(ctx, r.access.ID, strings.Repeat("c", 24))
	require.ErrorIs(t, err, iam.ErrForbidden)
	require.Empty(t, f.requests)
	stop := proto.WebIDERequest{Action: "stop", InstanceID: strings.Repeat("c", 24), OwnerID: "forged", AccessID: "forged"}
	_, err = cp.WebIDERuntime(ctx, r.access.ID, stop)
	require.NoError(t, err)
	require.Len(t, f.requests, 1)
	require.Equal(t, "2", f.requests[0].OwnerID)
	require.Equal(t, r.access.ID, f.requests[0].AccessID)
	for _, boundary := range []string{"access", "application", "connector"} {
		t.Run(boundary, func(t *testing.T) {
			r.access.OwnerID, r.app.OwnerID, r.connectorOwner = 2, 2, 2
			switch boundary {
			case "access":
				r.access.OwnerID = 3
			case "application":
				r.app.OwnerID = 3
			case "connector":
				r.connectorOwner = 3
			}
			_, err := cp.WebIDERuntime(ctx, r.access.ID, stop)
			require.Error(t, err)
			require.Len(t, f.requests, 1)
		})
	}
}
