package web

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type lifecycleRoutingCP struct {
	uninstallCP
	edge uint64
	task string
}

func (c *lifecycleRoutingCP) GetEdgeInstallation(ctx context.Context, id uint64) (proto.InstallationStatus, error) {
	c.edge = id
	return proto.InstallationStatus{Version: 1}, nil
}
func (c *lifecycleRoutingCP) CreateEdgeUninstall(ctx context.Context, id uint64, name, instance string) (*model.EdgeUninstallTask, error) {
	c.edge = id
	return &model.EdgeUninstallTask{ID: "job"}, nil
}
func (c *lifecycleRoutingCP) GetEdgeUninstall(ctx context.Context, id uint64, task string) (*model.EdgeUninstallTask, error) {
	c.edge, c.task = id, task
	return &model.EdgeUninstallTask{ID: task}, nil
}
func (c *lifecycleRoutingCP) ReportEdgeUninstall(ctx context.Context, id, token, status string) error {
	c.task = id
	return nil
}

// Exercise the production Kratos router, not only direct handlers with Go's
// Request.SetPathValue: Kratos populates Gorilla variables instead.
func TestLifecycleKratosRouting(t *testing.T) {
	for _, tc := range []struct {
		method, path, body string
		code               int
		edge               uint64
		task               string
	}{
		{"GET", "/api/v1/edges/7/installation", "", 200, 7, ""},
		{"POST", "/api/v1/edges/7/uninstall", `{"confirm_name":"test","instance_id":"instance"}`, 202, 7, ""},
		{"GET", "/api/v1/edges/7/uninstall/active", "", 200, 7, "active"},
		{"GET", "/api/v1/edges/7/uninstall/job", "", 200, 7, "job"},
		{"POST", "/api/v1/edge-uninstall-results/job", `{"status":"completed"}`, 200, 0, "job"},
	} {
		t.Run(tc.method+tc.path, func(t *testing.T) {
			cp := &lifecycleRoutingCP{}
			web := &web{controlPlane: cp}
			srv := kratoshttp.NewServer()
			srv.HandleFunc("/api/v1/edges/{id}/installation", web.handleEdgeInstallationHTTP)
			srv.HandleFunc("/api/v1/edges/{id}/uninstall", web.handleEdgeUninstallHTTP)
			srv.HandleFunc("/api/v1/edges/{id}/uninstall/{task}", web.handleEdgeUninstallHTTP)
			srv.HandleFunc("/api/v1/edge-uninstall-results/{task}", web.handleEdgeUninstallResultHTTP)
			req := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			req.Header.Set("Authorization", "Bearer "+strings.Repeat("a", 64))
			req = req.WithContext(context.WithValue(req.Context(), "user", &model.User{Model: gorm.Model{ID: 9}}))
			rsp := httptest.NewRecorder()
			srv.ServeHTTP(rsp, req)
			require.Equal(t, tc.code, rsp.Code, rsp.Body.String())
			require.Equal(t, tc.edge, cp.edge)
			require.Equal(t, tc.task, cp.task)
		})
	}
}
