package web

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestWebDataTargetHiddenResourceReturns404(t *testing.T) {
	conf := &config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "scope.db")}}
	r, err := repo.NewRepo(conf)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	admin := &model.User{Name: "Admin", Email: "admin@example.test", Status: model.UserStatusActive}
	require.NoError(t, r.CreateUser(admin))
	s, err := iam.NewIAMService(r)
	require.NoError(t, err)
	require.NoError(t, s.EnsureOrganizationBootstrap())
	root, err := r.GetRootOrganization()
	require.NoError(t, err)
	user, _, err := s.CreateUserFor(admin, root.ID, "User", "user@example.test", "fixture-password", model.IAMRoleUser)
	require.NoError(t, err)
	proxy := &model.Proxy{Name: "private-access", ApplicationID: 1, Status: model.ProxyStatusStopped}
	require.NoError(t, r.CreateProxy(proxy))
	require.NoError(t, r.UpsertIAMResourceRelation(&model.IAMResourceRelation{
		ResourceType: "access", ResourceID: uint64(proxy.ID), Relation: model.IAMRelationOwner,
		SubjectType: model.IAMSubjectUser, SubjectID: admin.ID, CreatedBy: admin.ID,
	}))
	cp, err := controlplane.NewControlPlane(conf, r, nil, nil)
	require.NoError(t, err)
	w := &web{iamService: s, controlPlane: cp}
	var responses []string
	for _, id := range []uint{proxy.ID, proxy.ID + 1000} {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/api/v1/webdata/proxies/%d", id), nil)
		req = req.WithContext(context.WithValue(req.Context(), "user", user))
		out := httptest.NewRecorder()
		w.handleWebDataTargetHTTP(out, req)
		require.Equal(t, http.StatusNotFound, out.Code, out.Body.String())
		require.NotContains(t, out.Body.String(), proxy.Name)
		responses = append(responses, out.Body.String())
		_, scopeErr := cp.GetWebDataTarget(context.WithValue(context.Background(), "user_id", user.ID), id)
		require.Error(t, scopeErr)
		require.Equal(t, http.StatusNotFound, webDataHTTPStatus(fmt.Errorf("wrapped: %w", scopeErr)))
	}
	require.Equal(t, responses[0], responses[1], "hidden and absent resources must be indistinguishable")
}

func TestWebDataHTTPStatusPreservesErrorCategories(t *testing.T) {
	for _, tc := range []struct {
		name   string
		err    error
		status int
	}{
		{"permission", iam.ErrForbidden, 403},
		{"control_plane_permission", controlplane.ErrForbidden(), 403},
		{"missing_record", gorm.ErrRecordNotFound, 404},
		{"invalid", errors.New("参数无效"), 400},
		{"offline", errors.New("连接器离线"), 409},
		{"unexpected", errors.New("storage failure"), 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.status, webDataHTTPStatus(tc.err))
			require.Equal(t, tc.status, webDataHTTPStatus(fmt.Errorf("wrapped: %w", tc.err)))
		})
	}
}
