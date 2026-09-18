package web

import (
	"github.com/liaisonio/liaison/pkg/dameng"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"net/http"
)

// handleWebDataCapabilitiesHTTP reports optional compiled-in drivers, not credentials.
// @Summary Available optional database drivers
// @Router /api/v1/webdata/capabilities [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handleWebDataCapabilitiesHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
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
	if actor.Status != model.UserStatusActive {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "account unavailable"})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": map[string]bool{"dameng": dameng.Available()}})
}
