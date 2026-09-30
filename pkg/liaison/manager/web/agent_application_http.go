package web

import (
	"context"
	"encoding/json"
	"errors"
	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type agentApplicationService interface {
	GetAgentApplication(context.Context, string) (*model.AgentApplication, error)
	AgentApplications(context.Context, int, int, uint64) (controlplane.AgentApplicationList, error)
	SaveAgentApplication(context.Context, string, controlplane.AgentApplicationInput) (*model.AgentApplication, error)
	DeleteAgentApplication(context.Context, string) error
}

// @Summary Manage reusable Agent application entries owned by the current user
// @Router /api/v1/agent-applications [get]
// @Router /api/v1/agent-applications [post]
// @Router /api/v1/agent-applications/{id} [put]
// @Router /api/v1/agent-applications/{id} [delete]
// @Success 200 {object} controlplane.AgentApplicationList
func (web *web) handleAgentApplicationHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if web.iamService == nil || web.iamService.RequireResourcePermission(actor, "connectors", "update") != nil {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "forbidden"})
		return
	}
	svc, ok := web.controlPlane.(agentApplicationService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "unavailable"})
		return
	}
	ctx := context.WithValue(r.Context(), "user", actor)
	ctx = context.WithValue(ctx, "user_id", actor.ID)
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/agent-applications")
	id = strings.TrimPrefix(id, "/")
	var data any
	switch {
	case r.Method == http.MethodGet && len(id) == 32:
		data, err = svc.GetAgentApplication(ctx, id)
	case r.Method == http.MethodGet && id == "":
		page, size := 1, 20
		if s := r.URL.Query().Get("page"); s != "" {
			page, _ = strconv.Atoi(s)
		}
		if s := r.URL.Query().Get("page_size"); s != "" {
			size, _ = strconv.Atoi(s)
		}
		edge, parseErr := strconv.ParseUint(r.URL.Query().Get("edge_id"), 10, 64)
		if r.URL.Query().Get("edge_id") != "" && parseErr != nil {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid connector"})
			return
		}
		data, err = svc.AgentApplications(ctx, page, size, edge)
	case (r.Method == http.MethodPost && id == "") || (r.Method == http.MethodPut && len(id) == 32):
		var input controlplane.AgentApplicationInput
		d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		d.DisallowUnknownFields()
		if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid request"})
			return
		}
		data, err = svc.SaveAgentApplication(ctx, id, input)
	case r.Method == http.MethodDelete && len(id) == 32:
		err = svc.DeleteAgentApplication(ctx, id)
	default:
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	if err != nil {
		code := 500
		if errors.Is(err, iam.ErrForbidden) {
			code = 403
		} else if mapped := kerrors.FromError(mapServiceError(err)); mapped != nil && mapped.Code >= 400 && mapped.Code < 500 {
			code = int(mapped.Code)
		}
		writeJSON(w, code, map[string]any{"code": code, "message": "Agent application unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}
