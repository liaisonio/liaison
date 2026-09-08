package web

import (
	"context"
	"encoding/json"
	"errors"
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
		Handle   string `json:"handle_id"`
		Editor   string `json:"editor_id"`
		Revision uint64 `json:"revision"`
		Text     string `json:"text"`
		Cursor   int    `json:"cursor"`
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
	result, err := service.Suggest(r.Context(), actor, input.Handle, input.Editor, assistance.Input{Revision: input.Revision, Text: input.Text, Cursor: input.Cursor}, r.Method == http.MethodDelete)
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
