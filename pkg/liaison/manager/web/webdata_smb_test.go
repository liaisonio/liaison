package web

import (
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSMBHTTP_RejectsUnauthenticatedAndCrossUserReads(t *testing.T) {
	w, admin, user := newPermissionHTTPTest(t)
	w.webData = newWebDataSessionStore()
	session, err := w.webData.create(&webDataSession{protocol: "smb", userID: user.ID})
	require.NoError(t, err)
	t.Cleanup(func() { w.webData.delete(session.token) })
	pat, err := w.iamService.CreatePAT(admin.ID, "smb-test", nil)
	require.NoError(t, err)
	for _, action := range []string{"list", "preview", "download"} {
		for _, token := range []string{"", pat.Token} {
			r := httptest.NewRequest("GET", "/api/v1/webdata/sessions/"+session.token+"/smb/"+action+"?path=/", nil)
			if token != "" {
				r.Header.Set("Authorization", "Bearer "+token)
			}
			rec := httptest.NewRecorder()
			w.handleWebSMBFilesHTTP(rec, r)
			require.Equal(t, 401, rec.Code, action)
			require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
		}
	}
	for _, action := range []string{"upload", "delete", "rename"} {
		r := httptest.NewRequest("POST", "/api/v1/webdata/sessions/"+session.token+"/smb/"+action, nil)
		rec := httptest.NewRecorder()
		w.handleWebSMBFilesHTTP(rec, r)
		require.Equal(t, 404, rec.Code)
	}
}
