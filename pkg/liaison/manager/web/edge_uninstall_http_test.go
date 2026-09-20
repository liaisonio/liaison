package web

import (
	"context"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http/httptest"
	"strings"
	"testing"
)

type uninstallCP struct {
	controlplane.ControlPlane
	calls int
	actor uint
	err   error
}

func (c *uninstallCP) CreateEdgeUninstall(ctx context.Context, id uint64, name, instance string) (*model.EdgeUninstallTask, error) {
	c.calls++
	c.actor, _ = ctx.Value("user_id").(uint)
	return &model.EdgeUninstallTask{ID: "job", TokenHash: "do-not-expose"}, c.err
}
func (c *uninstallCP) GetEdgeUninstall(ctx context.Context, id uint64, task string) (*model.EdgeUninstallTask, error) {
	c.calls++
	return &model.EdgeUninstallTask{ID: task}, c.err
}
func (c *uninstallCP) ReportEdgeUninstall(ctx context.Context, id, token, status string) error {
	c.calls++
	return c.err
}

func TestUninstallHTTPBoundary(t *testing.T) {
	for _, tc := range []struct {
		name, method, body string
		auth               bool
		err                error
		code               int
		calls              int
	}{
		{"create", "POST", `{"confirm_name":"edge","instance_id":"test"}`, true, nil, 202, 1},
		{"anonymous", "POST", `{}`, false, nil, 401, 0},
		{"wrong method", "DELETE", `{}`, true, nil, 405, 0},
		{"unknown field", "POST", `{"executable":"/other"}`, true, nil, 400, 0},
		{"multiple bodies", "POST", `{} {}`, true, nil, 400, 0},
		{"oversize", "POST", `{"confirm_name":"` + strings.Repeat("x", 4096) + `"}`, true, nil, 400, 0},
		{"denied", "POST", `{}`, true, iam.ErrForbidden, 403, 1},
		{"resource denied", "POST", `{}`, true, controlplane.ErrForbidden(), 403, 1},
		{"invalid confirmation", "POST", `{}`, true, kerrors.BadRequest("CONFIRMATION_REQUIRED", "do-not-expose"), 400, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp := &uninstallCP{err: tc.err}
			web := &web{controlPlane: cp}
			req := httptest.NewRequest(tc.method, "/api/v1/edges/7/uninstall", strings.NewReader(tc.body))
			req.SetPathValue("id", "7")
			if tc.auth {
				req = req.WithContext(context.WithValue(req.Context(), "user", &model.User{Model: gorm.Model{ID: 9}}))
			}
			rsp := httptest.NewRecorder()
			web.handleEdgeUninstallHTTP(rsp, req)
			require.Equal(t, tc.code, rsp.Code)
			require.Equal(t, tc.calls, cp.calls)
			require.NotContains(t, rsp.Body.String(), "do-not-expose")
			if tc.code == 202 {
				require.EqualValues(t, 9, cp.actor)
			}
		})
	}
}
func TestUninstallCallbackRequiresDedicatedToken(t *testing.T) {
	cp := &uninstallCP{}
	web := &web{controlPlane: cp}
	for _, token := range []string{"", "Bearer user-jwt", "Bearer " + strings.Repeat("a", 64)} {
		req := httptest.NewRequest("POST", "/api/v1/edge-uninstall-results/task", strings.NewReader(`{"status":"completed"}`))
		req.Header.Set("Authorization", token)
		req.SetPathValue("task", "task")
		rsp := httptest.NewRecorder()
		web.handleEdgeUninstallResultHTTP(rsp, req)
		if len(token) == 71 {
			require.Equal(t, 200, rsp.Code)
		} else {
			require.Equal(t, 401, rsp.Code)
		}
	}
	require.Equal(t, 1, cp.calls)
}
