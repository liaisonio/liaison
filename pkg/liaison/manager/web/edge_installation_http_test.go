package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type installationCP struct {
	controlplane.ControlPlane
	actor uint
	err   error
}

func (c *installationCP) GetEdgeInstallation(ctx context.Context, id uint64) (proto.InstallationStatus, error) {
	c.actor, _ = ctx.Value("user_id").(uint)
	return proto.InstallationStatus{Version: 1, Reason: "installation_unverified"}, c.err
}

func TestInstallationHTTPReadOnly(t *testing.T) {
	for _, tc := range []struct {
		method, id string
		err        error
		code       int
	}{{"GET", "7", nil, 200}, {"POST", "7", nil, 405}, {"DELETE", "7", nil, 405}, {"GET", "0", nil, 400}, {"GET", "bad", nil, 400}, {"GET", "7", iam.ErrForbidden, 403}, {"GET", "7", errors.New("private credentials detail"), 500}} {
		c := &installationCP{err: tc.err}
		w := &web{controlPlane: c}
		req := httptest.NewRequest(tc.method, "/api/v1/edges/"+tc.id+"/installation", nil)
		req.SetPathValue("id", tc.id)
		req = req.WithContext(context.WithValue(req.Context(), "user", &model.User{Model: gorm.Model{ID: 9}}))
		recorder := httptest.NewRecorder()
		w.handleEdgeInstallationHTTP(recorder, req)
		require.Equal(t, tc.code, recorder.Code)
		require.False(t, strings.Contains(recorder.Body.String(), "private credentials"))
		if tc.code == 200 {
			require.Equal(t, uint(9), c.actor)
		}
	}
	w := &web{}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/edges/7/installation", nil)
	recorder := httptest.NewRecorder()
	w.handleEdgeInstallationHTTP(recorder, req)
	require.Equal(t, 401, recorder.Code)
}
