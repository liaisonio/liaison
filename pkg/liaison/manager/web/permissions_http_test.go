package web

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func newPermissionHTTPTest(t *testing.T) (*web, *model.User, *model.User) {
	t.Helper()
	r, err := repo.NewRepo(&config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "permissions.db")}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	admin := &model.User{Name: "Admin", Email: "admin@example.test", Status: model.UserStatusActive}
	require.NoError(t, r.CreateUser(admin))
	s, err := iam.NewIAMService(r)
	require.NoError(t, err)
	require.NoError(t, s.EnsureOrganizationBootstrap())
	root, err := r.GetRootOrganization()
	require.NoError(t, err)
	user, _, err := s.CreateUserFor(admin, root.ID, "User", "user@example.test", "password123", model.IAMRoleUser)
	require.NoError(t, err)
	return &web{iamService: s, agentService: fakeAgentService{enabled: true}}, admin, user
}

func TestPermissionHTTP_UserCannotGrantSelfAndRevocationIsImmediate(t *testing.T) {
	w, admin, user := newPermissionHTTPTest(t)
	call := func(actor *model.User, method, body string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/iam/roles/user/permissions", strings.NewReader(body))
		r = r.WithContext(context.WithValue(r.Context(), "user", actor))
		rec := httptest.NewRecorder()
		w.handleUserPermissionsHTTP(rec, r)
		return rec
	}
	require.Equal(t, 403, call(user, http.MethodPut, `{"enabled":["ai.home.use"]}`).Code)
	require.Equal(t, 403, call(user, http.MethodGet, "").Code)
	require.Equal(t, 400, call(admin, http.MethodPut, `{"enabled":["permissions.manage"]}`).Code)
	require.Equal(t, 200, call(admin, http.MethodPut, `{"enabled":["ai.home.use"]}`).Code)
	require.NoError(t, w.iamService.RequireFeature(user, iam.FeatureHomeAI))
	require.ErrorIs(t, w.iamService.RequireFeature(user, iam.FeatureAccessAI), iam.ErrForbidden)
	require.Equal(t, 200, call(admin, http.MethodPut, `{"enabled":[]}`).Code)
	r := httptest.NewRequest(http.MethodGet, "/api/v1/agent/status", nil)
	r = r.WithContext(context.WithValue(r.Context(), "user", user))
	rec := httptest.NewRecorder()
	w.handleAgentStatusHTTP(rec, r)
	require.Equal(t, 403, rec.Code)
}
