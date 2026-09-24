package web

import (
	"context"
	"errors"
	"net/http"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
)

// handleAISetupHTTP probes drafts or atomically saves a new model access.
// @Summary Discover a draft model service or create its application and access
// @Router /api/v1/ai/setup/probe [post]
// @Router /api/v1/ai/setup [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAISetupHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || r.URL.RawQuery != "" || (r.URL.Path != "/api/v1/ai/setup" && r.URL.Path != "/api/v1/ai/setup/probe") {
		aiError(w, 400)
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", actor.ID)
	var v controlplane.AISetupRequest
	var data any
	err = aiDecode(w, r, &v)
	if err == nil {
		if r.URL.Path == "/api/v1/ai/setup/probe" {
			data, err = web.aiGateway.ProbeSetup(ctx, v)
		} else {
			data, err = web.aiGateway.CreateSetup(ctx, v)
		}
	}
	if errors.Is(err, controlplane.ErrAIExistingApplication) {
		aiError(w, 409, "APPLICATION_ALREADY_EXISTS")
		return
	}
	if err != nil {
		aiError(w, aiStatus(err))
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}
