package web

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/dameng"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestDamengCapabilitiesRequireAuthentication(t *testing.T) {
	w, _, user := newPermissionHTTPTest(t)
	call := func(actor *model.User, method string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/api/v1/webdata/capabilities", nil)
		if actor != nil {
			r = r.WithContext(context.WithValue(r.Context(), "user", actor))
		}
		out := httptest.NewRecorder()
		w.handleWebDataCapabilitiesHTTP(out, r)
		return out
	}
	require.Equal(t, 401, call(nil, "GET").Code)
	require.Equal(t, 405, call(user, "POST").Code)
	response := call(user, "GET")
	require.Equal(t, 200, response.Code)
	if dameng.Available() {
		require.Contains(t, response.Body.String(), `"dameng":true`)
	} else {
		require.Contains(t, response.Body.String(), `"dameng":false`)
	}
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
}

func TestDamengConnectionRejectsUnsupportedOptions(t *testing.T) {
	for _, s := range []*webDataSession{{tlsMode: "require"}, {connectionParams: "dialName=other"}, {database: "another"}} {
		s.target = &controlplane.WebDataTarget{}
		require.Error(t, (&web{}).openWebDataDameng(context.Background(), s, "secret"))
	}
}
func TestDamengStatementsAndAudit(t *testing.T) {
	require.NotContains(t, safeDamengError(context.Background(), errors.New("dm://secret@example")).Error(), "secret")
	require.Equal(t, "SELECT 1", damengStatement(" SELECT 1; "))
	require.Equal(t, "BEGIN NULL; END;", damengStatement("BEGIN NULL; END;"))
	require.True(t, webDataExecuteIsQuery("dameng", "SELECT 1"))
	require.False(t, webDataExecuteIsQuery("dameng", "DELETE FROM T"))
	require.True(t, webDataShouldAuditExecute("dameng", "SELECT 1"))
	calls := 0
	s := &webDataSession{sqlRevoke: func() { calls++ }}
	s.close()
	s.close()
	require.Equal(t, 1, calls)
}
