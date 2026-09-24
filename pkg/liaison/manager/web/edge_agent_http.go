package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/proto"
)

type edgeAgentService interface {
	AgentConnectors(context.Context) ([]controlplane.AgentConnector, error)
	EdgeAgent(context.Context, proto.EdgeAgentRequest) (proto.EdgeAgentResult, error)
}

// handleEdgeAgentHTTP provides temporary, owner-only local Agent sessions.
// @Summary Use the existing Agent on an owned connector
// @Router /api/v1/edge-agents [post]
// @Router /api/v1/edge-agents/connectors [get]
// @Success 200 {object} proto.EdgeAgentResult
func (web *web) handleEdgeAgentHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	list := r.URL.Path == "/api/v1/edge-agents/connectors"
	want := http.MethodPost
	if list {
		want = http.MethodGet
	}
	if r.Method != want {
		w.Header().Set("Allow", want)
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if web.iamService == nil || web.iamService.RequireResourcePermission(actor, "connectors", "update") != nil {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "forbidden"})
		return
	}
	svc, ok := web.controlPlane.(edgeAgentService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "agent unavailable"})
		return
	}
	ctx := context.WithValue(r.Context(), "user", actor)
	ctx = context.WithValue(ctx, "user_id", actor.ID)
	var data any
	if list {
		data, err = svc.AgentConnectors(ctx)
	} else {
		var req proto.EdgeAgentRequest
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32768))
		d.DisallowUnknownFields()
		if d.Decode(&req) != nil || d.Decode(new(any)) != io.EOF {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid agent request"})
			return
		}
		data, err = svc.EdgeAgent(ctx, req)
	}
	if err != nil {
		code := 500
		if errors.Is(err, iam.ErrForbidden) {
			code = 403
		} else if mapped := kerrors.FromError(mapServiceError(err)); mapped != nil && mapped.Code >= 400 && mapped.Code < 500 {
			code = int(mapped.Code)
		}
		writeJSON(w, code, map[string]any{"code": code, "message": "agent unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}
