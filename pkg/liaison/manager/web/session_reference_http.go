package web

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"net/http"
	"regexp"
)

var publicConnectionReference = regexp.MustCompile(`^conn_[a-f0-9]{24}$`)

// @Summary Resolve an owned SSH connection reference to authorized routing metadata
// @Router /api/v1/webssh/session-references/{reference} [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handleSSHSessionReferenceHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	ref := agentPathValue(r, "reference")
	notFound := func() {
		writeJSON(w, http.StatusNotFound, map[string]any{"code": 404, "message": "connection session unavailable"})
	}
	if !publicConnectionReference.MatchString(ref) {
		notFound()
		return
	}
	var accessID uint
	if d, err := web.accessSessions.DescribeReference(r.Context(), ref, actor.ID); err == nil && d.Protocol == accesssession.ProtocolWebSSH {
		accessID = d.AccessID
	}
	if accessID == 0 && web.agentService != nil && r.URL.Query().Get("agent") != "" {
		detail, err := web.agentService.GetSession(r.Context(), actor, r.URL.Query().Get("agent"))
		if err == nil {
			for _, a := range detail.Attachments {
				if string(a.Protocol) == string(accesssession.ProtocolWebSSH) && accesssession.PublicReference(a.ID) == ref {
					accessID = a.AccessID
					break
				}
			}
		}
	}
	if accessID == 0 {
		notFound()
		return
	}
	// Revalidate current access visibility/feature permissions through the same
	// business method as the normal target endpoint, even for historic sessions.
	ctx := context.WithValue(r.Context(), "user_id", actor.ID)
	if _, err := web.controlPlane.GetWebSSHTarget(ctx, accessID); err != nil {
		notFound()
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "data": map[string]any{"access_id": accessID}})
}
