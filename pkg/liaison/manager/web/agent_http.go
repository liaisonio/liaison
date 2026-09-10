package web

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentapplication "github.com/liaisonio/liaison/pkg/liaison/manager/agent/application"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/modelsettings"
	agentruntime "github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type AgentService interface {
	Available() bool
	CreateSession(ctx context.Context, request agentapplication.CreateSessionRequest) (agentapplication.SessionDetail, error)
	ListSessions(ctx context.Context, actor *model.User) ([]agentruntime.Session, error)
	GetSession(ctx context.Context, actor *model.User, sessionID string) (agentapplication.SessionDetail, error)
	ArchiveSession(ctx context.Context, actor *model.User, sessionID string, expectedVersion uint64) (agentruntime.Session, error)
	RunTurn(ctx context.Context, request agentapplication.RunTurnRequest) (agentruntime.RunResult, error)
	ResolveApproval(ctx context.Context, request agentapplication.ResolveApprovalRequest) (agentruntime.RunResult, error)
}

func agentPathValue(r *http.Request, key string) string {
	// Kratos HandleFunc routes use gorilla/mux, not net/http ServeMux.
	if value := mux.Vars(r)[key]; value != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(r.PathValue(key))
}

// @Summary Get Agent availability and public model choices
// @Router /api/v1/agent/status [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAgentStatusHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if web.iamService == nil {
		writeIAMError(w, iam.ErrForbidden)
		return
	}
	grants, err := web.iamService.EffectiveFeatures(actor)
	if err != nil {
		writeIAMError(w, err)
		return
	}
	if !grants[iam.FeatureHomeAI] && !grants[iam.FeatureAccessAI] {
		writeIAMError(w, iam.ErrForbidden)
		return
	}
	choices := []modelsettings.Choice{}
	if configured, ok := web.agentService.(interface{ ModelSettings() *modelsettings.Manager }); ok && configured.ModelSettings() != nil {
		choices, err = configured.ModelSettings().Choices(r.Context())
		if err != nil {
			writeAgentError(w, agentapplication.ErrUnavailable)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "data": map[string]any{"enabled": web.agentService.Available(), "models": choices}})
}

type createAgentSessionHTTPRequest struct {
	Kind     tool.SessionKind `json:"kind"`
	HandleID string           `json:"handle_id"`
	Title    string           `json:"title"`
}

type archiveAgentSessionHTTPRequest struct {
	Version uint64 `json:"version"`
}

type runAgentTurnHTTPRequest struct {
	References     []agentruntime.ResourceReference `json:"references"`
	Prompt         string                           `json:"prompt"`
	ModelSelection agentruntime.ModelSelection      `json:"model_selection"`
}

type resolveAgentApprovalHTTPRequest struct {
	Decision string `json:"decision"`
	Note     string `json:"note"`
}

// handleAgentSessionsHTTP creates or lists Agent sessions.
// @Summary Create or list Agent sessions
// @Router /api/v1/agent/sessions [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAgentSessionsHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	switch r.Method {
	case http.MethodGet:
		sessions, err := web.agentService.ListSessions(r.Context(), actor)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": map[string]any{"items": sessions}})
	case http.MethodPost:
		var request createAgentSessionHTTPRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeAgentError(w, fmt.Errorf("%w: invalid request body", agentapplication.ErrInvalid))
			return
		}
		detail, err := web.agentService.CreateSession(r.Context(), agentapplication.CreateSessionRequest{
			Actor: actor, HandleID: request.HandleID, Title: request.Title, Kind: request.Kind,
		})
		if err != nil {
			writeAgentError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"code": 201, "message": "created", "data": detail})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
	}
}

// handleAgentTurnHTTP runs one turn while model and tool progress is emitted
// through the session SSE stream.
// @Summary Run an Agent turn
// @Router /api/v1/agent/sessions/{id}/turns [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAgentTurnHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	var request runAgentTurnHTTPRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 72*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAgentError(w, fmt.Errorf("%w: invalid request body", agentapplication.ErrInvalid))
		return
	}
	result, err := web.agentService.RunTurn(r.Context(), agentapplication.RunTurnRequest{
		Actor: actor, SessionID: agentPathValue(r, "id"), Prompt: request.Prompt, ModelSelection: request.ModelSelection, References: request.References,
	})
	if err != nil {
		// Do not log prompts, tool output or provider errors (may contain secrets).
		log.Warnf("agent run failed: session_id=%q turn_id=%q user_id=%d error_code=%q", agentPathValue(r, "id"), result.Turn.ID, actor.ID, result.Turn.ErrorCode)
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": result})
}

// handleAgentApprovalHTTP resolves one frozen tool invocation. Approval never
// accepts a tool call or resource binding from the browser.
// @Summary Resolve an Agent tool approval
// @Router /api/v1/agent/sessions/{id}/approvals/{approval_id} [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAgentApprovalHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	var request resolveAgentApprovalHTTPRequest
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		writeAgentError(w, fmt.Errorf("%w: invalid request body", agentapplication.ErrInvalid))
		return
	}
	decision := strings.ToLower(strings.TrimSpace(request.Decision))
	if decision != "approve" && decision != "deny" {
		writeAgentError(w, fmt.Errorf("%w: decision must be approve or deny", agentapplication.ErrInvalid))
		return
	}
	result, err := web.agentService.ResolveApproval(r.Context(), agentapplication.ResolveApprovalRequest{
		Actor: actor, SessionID: agentPathValue(r, "id"), ApprovalID: agentPathValue(r, "approval_id"),
		Approve: decision == "approve", Note: request.Note,
	})
	if err != nil {
		writeAgentError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": result})
}

// handleAgentSessionHTTP reads or archives one Agent session.
// @Summary Read or archive an Agent session
// @Router /api/v1/agent/sessions/{id} [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAgentSessionHTTP(w http.ResponseWriter, r *http.Request) {
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	sessionID := agentPathValue(r, "id")
	switch r.Method {
	case http.MethodGet:
		detail, err := web.agentService.GetSession(r.Context(), actor, sessionID)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": detail})
	case http.MethodDelete:
		var request archiveAgentSessionHTTPRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4*1024))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&request); err != nil {
			writeAgentError(w, fmt.Errorf("%w: invalid request body", agentapplication.ErrInvalid))
			return
		}
		session, err := web.agentService.ArchiveSession(r.Context(), actor, sessionID, request.Version)
		if err != nil {
			writeAgentError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": session})
	default:
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
	}
}

// handleAgentSessionEventsHTTP streams ephemeral runtime deltas. Clients must
// first restore the durable session snapshot through the session GET endpoint.
// @Summary Stream Agent session events
// @Router /api/v1/agent/sessions/{id}/events [get]
// @Success 200 {string} string
func (web *web) handleAgentSessionEventsHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	sessionID := agentPathValue(r, "id")
	if _, err := web.agentService.GetSession(r.Context(), actor, sessionID); err != nil {
		writeAgentError(w, err)
		return
	}
	if web.agentEvents == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]any{"code": http.StatusServiceUnavailable, "message": "agent event stream unavailable"})
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "streaming unsupported"})
		return
	}
	subscription, err := web.agentEvents.Subscribe(r.Context(), sessionID)
	if err != nil {
		writeAgentError(w, err)
		return
	}
	defer subscription.Close()
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache, no-store")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	flusher.Flush()
	keepAlive := time.NewTicker(15 * time.Second)
	defer keepAlive.Stop()
	checkAccess := func() error {
		if service, ok := web.agentService.(interface {
			AuthorizeSession(context.Context, *model.User, string) error
		}); ok {
			return service.AuthorizeSession(r.Context(), actor, sessionID)
		}
		_, err := web.agentService.GetSession(r.Context(), actor, sessionID)
		return err
	}
	for {
		select {
		case event, open := <-subscription.Events:
			if !open {
				return
			}
			if err := checkAccess(); err != nil {
				return
			}
			if err := writeAgentSSE(w, event); err != nil {
				return
			}
			flusher.Flush()
		case <-keepAlive.C:
			if err := checkAccess(); err != nil {
				return
			}
			if _, err := fmt.Fprint(w, ": keepalive\n\n"); err != nil {
				return
			}
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func writeAgentSSE(w http.ResponseWriter, event agentruntime.Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "id: %s\nevent: %s\ndata: %s\n\n", strconv.FormatUint(event.Sequence, 10), event.Type, payload)
	return err
}

func writeAgentError(w http.ResponseWriter, err error) {
	status := http.StatusInternalServerError
	message := "agent operation failed"
	switch {
	case errors.Is(err, agentapplication.ErrConnectionUnavailable), errors.Is(err, accesssession.ErrHandleNotFound), errors.Is(err, accesssession.ErrHandleMismatch):
		status, message = http.StatusConflict, "agent connection unavailable; reconnect and start a new agent session"
	case errors.Is(err, agentapplication.ErrReferenceUnavailable):
		status, message = http.StatusNotFound, "referenced resource unavailable; remove it and select again"
	case errors.Is(err, agentapplication.ErrInvalid):
		status, message = http.StatusBadRequest, "invalid agent request"
	case errors.Is(err, agentapplication.ErrNotFound), errors.Is(err, agentruntime.ErrSessionNotFound):
		status, message = http.StatusNotFound, "agent session not found"
	case errors.Is(err, iam.ErrForbidden):
		status, message = http.StatusForbidden, "forbidden"
	case errors.Is(err, agentapplication.ErrUnavailable):
		status, message = http.StatusServiceUnavailable, "agent runtime unavailable"
	case errors.Is(err, agentruntime.ErrVersionConflict), errors.Is(err, agentruntime.ErrTurnAlreadyActive):
		status, message = http.StatusConflict, "agent session changed; refresh and retry"
	case errors.Is(err, agentruntime.ErrApprovalConflict):
		status, message = http.StatusConflict, "agent approval changed; refresh and retry"
	case errors.Is(err, agentruntime.ErrApprovalExpired):
		status, message = http.StatusGone, "agent approval expired"
	}
	writeJSON(w, status, map[string]any{"code": status, "message": message})
}
