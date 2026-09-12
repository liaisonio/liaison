package web

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"io"
	"net/http"
)

type assistanceService interface {
	Suggest(context.Context, *model.User, string, string, assistance.Input, bool) (assistance.Suggestion, error)
}

// handleAssistanceHTTP generates text only; never writes to the PTY.
// @Summary Generate an inline suggestion
// @Router /api/v1/assistance/suggestions [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAssistanceHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	service, ok := web.agentService.(assistanceService)
	if !ok {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}
	var input struct {
		AgentSessionID string `json:"agent_session_id"`
		Handle         string `json:"handle_id"`
		Editor         string `json:"editor_id"`
		Revision       uint64 `json:"revision"`
		Text           string `json:"text"`
		Cursor         int    `json:"cursor"`
		ContextMode    string `json:"context_mode"`
	}
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if decoder.Decode(new(json.RawMessage)) != io.EOF {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if input.ContextMode != "" && input.ContextMode != "none" && input.ContextMode != "commands" && input.ContextMode != "output" {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid context mode"})
		return
	}
	editing := assistance.Input{AgentSessionID: input.AgentSessionID, Revision: input.Revision, Text: input.Text, Cursor: input.Cursor}
	if r.Method == http.MethodPost && (input.ContextMode == "commands" || input.ContextMode == "output") {
		d, err := web.accessSessions.Describe(r.Context(), input.Handle, actor.ID)
		if err != nil {
			writeJSON(w, 404, map[string]any{"code": 404, "message": "connection not found"})
			return
		}
		if d.Protocol != accesssession.ProtocolWebSSH {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "shell context requires WebSSH"})
			return
		}
		h, err := web.accessSessions.Resolve(r.Context(), accesssession.ResolveRequest{ID: d.ID, UserID: d.UserID, AccessID: d.AccessID, ApplicationID: d.ApplicationID, Protocol: d.Protocol, Generation: d.Generation})
		if err != nil {
			writeJSON(w, 409, map[string]any{"code": 409, "message": "connection changed"})
			return
		}
		if source, ok := h.Terminal.(interface {
			ShellContext(context.Context, bool) (json.RawMessage, error)
		}); ok {
			value, err := source.ShellContext(r.Context(), input.ContextMode == "output")
			if err != nil {
				writeAgentError(w, err)
				return
			}
			editing.ShellContext = string(value)
		}
	}
	result, err := service.Suggest(r.Context(), actor, input.Handle, input.Editor, editing, r.Method == http.MethodDelete)
	if err != nil {
		switch {
		case errors.Is(err, assistance.ErrSuggestion):
			writeJSON(w, http.StatusBadGateway, map[string]any{"code": 502, "message": "model returned an invalid suggestion; retry manually"})
		case errors.Is(err, assistance.ErrInvalid):
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": 400, "message": "invalid assistance request"})
		case errors.Is(err, assistance.ErrClosed), errors.Is(err, assistance.ErrStale):
			writeJSON(w, http.StatusConflict, map[string]any{"code": 409, "message": "assistance request is stale or closed"})
		case errors.Is(err, context.DeadlineExceeded):
			writeJSON(w, http.StatusGatewayTimeout, map[string]any{"code": 504, "message": "suggestion timed out; retry manually"})
		case errors.Is(err, context.Canceled):
			return
		default:
			writeAgentError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": map[string]any{"revision": result.Revision, "cursor": result.Cursor, "text": result.Text}})
}
