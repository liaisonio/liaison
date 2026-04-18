package web

import (
	"net/http"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
)

// handleClientIP returns the caller's source IP address. Open (no auth) —
// public-IP self-discovery is not sensitive and this is what the firewall
// panel uses for the "My IP" one-click-add affordance.
func (web *web) handleClientIP(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    200,
		"message": "success",
		"data":    map[string]any{"ip": iam.ExtractClientIP(r)},
	})
}
