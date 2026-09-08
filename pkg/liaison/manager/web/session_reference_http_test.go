package web

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"testing"
)

type referenceCP struct {
	controlplane.ControlPlane
	deny bool
	user uint
}

func (c *referenceCP) GetWebSSHTarget(ctx context.Context, _ uint) (*controlplane.WebSSHTarget, error) {
	c.user, _ = ctx.Value("user_id").(uint)
	if c.deny {
		return nil, errors.New("private error")
	}
	return &controlplane.WebSSHTarget{}, nil
}
func TestReferenceHTTPChecksOwnerAndCurrentResourceAccess(t *testing.T) {
	r := accesssession.NewRegistry()
	_, remove, err := r.Register(accesssession.Handle{Descriptor: accesssession.Descriptor{ID: "secret-token", UserID: 7, AccessID: 3, Protocol: accesssession.ProtocolWebSSH}, Data: &webDataAgentHandle{}})
	require.NoError(t, err)
	t.Cleanup(remove)
	for _, tc := range []struct {
		user   uint
		deny   bool
		status int
	}{{7, false, 200}, {8, false, 404}, {7, true, 404}} {
		cp := &referenceCP{deny: tc.deny}
		w := &web{accessSessions: r, controlPlane: cp}
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetPathValue("reference", accesssession.PublicReference("secret-token"))
		req = req.WithContext(context.WithValue(req.Context(), "user", &model.User{Model: gorm.Model{ID: tc.user}}))
		rec := httptest.NewRecorder()
		w.handleSSHSessionReferenceHTTP(rec, req)
		require.Equal(t, tc.status, rec.Code)
		require.NotContains(t, rec.Body.String(), "secret-token")
		require.NotContains(t, rec.Body.String(), "private error")
		if tc.user == 7 {
			require.Equal(t, uint(7), cp.user)
		}
	}
}
