package web

import (
	"context"
	"encoding/json"
	"errors"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/proto"
	"io"
	"net/http"
)

type applicationProbeService interface {
	ProbeApplication(context.Context, controlplane.ApplicationProbeRequest) (proto.TCPProbeResult, error)
}

// handleApplicationProbeHTTP tests TCP reachability through the selected Edge.
// @Summary Test application TCP reachability without saving
// @Router /api/v1/applications/probe [post]
// @Success 200 {object} proto.TCPProbeResult
func (web *web) handleApplicationProbeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	var request controlplane.ApplicationProbeRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid probe"})
		return
	}
	if decoder.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid probe"})
		return
	}
	action := "create"
	if request.ApplicationID != 0 {
		action = "update"
	}
	if web.iamService == nil || web.iamService.RequireResourcePermission(actor, "applications", action) != nil {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "forbidden"})
		return
	}
	service, ok := web.controlPlane.(applicationProbeService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "probe unavailable"})
		return
	}
	ctx := context.WithValue(r.Context(), "user", actor)
	ctx = context.WithValue(ctx, "user_id", actor.ID)
	result, err := service.ProbeApplication(ctx, request)
	if err != nil {
		code := 500
		if errors.Is(err, iam.ErrForbidden) {
			code = 403
		} else if mapped := kerrors.FromError(mapServiceError(err)); mapped != nil && mapped.Code >= 400 && mapped.Code < 500 {
			code = int(mapped.Code)
		}
		writeJSON(w, code, map[string]any{"code": code, "message": "probe unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": result})
}
