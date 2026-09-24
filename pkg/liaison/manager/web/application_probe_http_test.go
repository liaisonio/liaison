package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

type probeHTTPControlPlane struct {
	controlplane.ControlPlane
	calls int
}

func (cp *probeHTTPControlPlane) ProbeApplication(ctx context.Context, r controlplane.ApplicationProbeRequest) (proto.TCPProbeResult, error) {
	cp.calls++
	return proto.TCPProbeResult{Version: 1, Status: "reachable"}, nil
}
func TestApplicationProbeHTTPPermissions(t *testing.T) {
	r, err := repo.NewRepo(&config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "probe.db")}})
	require.NoError(t, err)
	t.Cleanup(func() { r.Close() })
	admin := &model.User{Name: "Admin", Email: "admin@example.test", Status: model.UserStatusActive}
	require.NoError(t, r.CreateUser(admin))
	service, err := iam.NewIAMService(r)
	require.NoError(t, err)
	require.NoError(t, service.EnsureOrganizationBootstrap())
	user := &model.User{Name: "Unbound user", Email: "user@example.test", Status: model.UserStatusActive}
	require.NoError(t, r.CreateUser(user))
	cp := &probeHTTPControlPlane{}
	w := &web{iamService: service, controlPlane: cp}
	for _, tc := range []struct {
		name, method, body string
		actor              *model.User
		code               int
	}{
		{"create", "POST", `{"edge_id":7,"host":"localhost","port":443}`, admin, 200},
		{"edit", "POST", `{"application_id":7}`, admin, 200},
		{"denied", "POST", `{"edge_id":7,"host":"localhost","port":443}`, user, 403},
		{"anonymous", "POST", `{}`, nil, 401},
		{"method", "GET", `{}`, admin, 405},
		{"unknown field", "POST", `{"password":"never forward"}`, admin, 400},
		{"extra JSON", "POST", `{} {}`, admin, 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := cp.calls
			req := httptest.NewRequest(tc.method, "/api/v1/applications/probe", strings.NewReader(tc.body))
			if tc.actor != nil {
				req = req.WithContext(context.WithValue(req.Context(), "user", tc.actor))
			}
			out := httptest.NewRecorder()
			w.handleApplicationProbeHTTP(out, req)
			require.Equal(t, tc.code, out.Code, out.Body.String())
			require.Equal(t, "no-store", out.Header().Get("Cache-Control"))
			if tc.code != http.StatusOK {
				require.Equal(t, before, cp.calls)
			}
		})
	}
}
