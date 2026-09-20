package web

import (
	"context"
	"errors"
	"net/http"
	"strconv"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/proto"
)

type edgeInstallationService interface {
	GetEdgeInstallation(context.Context, uint64) (proto.InstallationStatus, error)
}

// handleEdgeInstallationHTTP inspects installation capability without mutation.
// @Summary Inspect connector installation and uninstall capability
// @Router /api/v1/edges/{id}/installation [get]
// @Success 200 {object} proto.InstallationStatus
func (web *web) handleEdgeInstallationHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	id, err := strconv.ParseUint(agentPathValue(r, "id"), 10, 64)
	if err != nil || id == 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid connector id"})
		return
	}
	service, ok := web.controlPlane.(edgeInstallationService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "installation preflight unavailable"})
		return
	}
	ctx := context.WithValue(r.Context(), "user", actor)
	ctx = context.WithValue(ctx, "user_id", actor.ID)
	status, err := service.GetEdgeInstallation(ctx, id)
	if err != nil {
		code := 500
		if errors.Is(err, iam.ErrForbidden) {
			code = 403
		} else if e := kerrors.FromError(mapServiceError(err)); e != nil && e.Code >= 400 && e.Code < 500 {
			code = int(e.Code)
		}
		writeJSON(w, code, map[string]any{"code": code, "message": "installation preflight unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": status})
}
