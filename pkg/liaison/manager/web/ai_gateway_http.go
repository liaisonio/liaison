package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/aigateway"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func aiStatus(err error) int {
	switch {
	case errors.Is(err, iam.ErrForbidden):
		return 403
	case errors.Is(err, controlplane.ErrAIInvalid), errors.Is(err, aigateway.ErrUnsupported):
		return 400
	case errors.Is(err, aigateway.ErrModelDenied):
		return 403
	case errors.Is(err, controlplane.ErrAIUnavailable):
		return 503
	default:
		return 500
	}
}
func aiError(w http.ResponseWriter, status int, reasons ...string) {
	message := http.StatusText(status)
	reason := map[int]string{400: "INVALID_REQUEST", 401: "AUTHENTICATION_REQUIRED", 403: "PERMISSION_DENIED", 405: "UNSUPPORTED_OPERATION", 413: "REQUEST_TOO_LARGE", 429: "RATE_LIMITED", 502: "UPSTREAM_ERROR", 503: "SERVICE_UNAVAILABLE", 504: "UPSTREAM_TIMEOUT"}[status]
	if reason == "" {
		reason = "INTERNAL_ERROR"
	}
	if len(reasons) > 0 {
		reason = reasons[0]
	}
	writeJSON(w, status, map[string]any{"code": status, "message": message, "error": map[string]string{"type": "liaison_gateway_error", "code": reason, "message": message, "request_id": w.Header().Get("X-Request-ID")}})
}
func aiDecode(w http.ResponseWriter, r *http.Request, v any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return controlplane.ErrAIInvalid
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return controlplane.ErrAIInvalid
	}
	return nil
}

// handleAIGatewayHTTP manages connector model configuration and scoped keys.
// @Summary Configure AI API applications, accesses, keys and request metadata
// @Router /api/v1/ai/applications/{id} [get]
// @Router /api/v1/ai/applications/{id} [put]
// @Router /api/v1/ai/applications/{id}/probe [post]
// @Router /api/v1/ai/accesses/{id} [get]
// @Router /api/v1/ai/accesses/{id}/workspace [get]
// @Router /api/v1/ai/accesses/{id} [put]
// @Router /api/v1/ai/accesses/{id}/keys [get]
// @Router /api/v1/ai/accesses/{id}/keys [post]
// @Router /api/v1/ai/accesses/{id}/keys/{key_id} [delete]
// @Router /api/v1/ai/accesses/{id}/keys/{key_id}/quota [put]
// @Router /api/v1/ai/accesses/{id}/requests [get]
// @Router /api/v1/ai/accesses/{id}/usage [get]
// @Router /api/v1/ai/accesses/{id}/test [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAIGatewayHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if web.aiGateway == nil {
		aiError(w, 503)
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/ai/"), "/")
	if len(parts) < 2 || r.URL.RawQuery != "" || r.URL.RawPath != "" {
		aiError(w, 400)
		return
	}
	parsed, err := strconv.ParseUint(parts[1], 10, 32)
	if err != nil || parsed == 0 {
		aiError(w, 400)
		return
	}
	id := uint(parsed)
	suffix := strings.Join(parts[2:], "/")
	if parts[0] == "accesses" && strings.HasPrefix(suffix, "v1/") {
		web.handleAIInference(w, r, id, suffix, false)
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", actor.ID)
	if parts[0] == "accesses" && suffix == "test" {
		web.handleAIInference(w, r.WithContext(ctx), id, "v1/chat/completions", true)
		return
	}
	var data any
	switch {
	case parts[0] == "accesses" && suffix == "workspace" && r.Method == "GET":
		data, err = web.aiGateway.Workspace(ctx, id)
	case parts[0] == "applications" && suffix == "" && r.Method == "GET":
		data, err = web.aiGateway.GetApplication(ctx, id)
	case parts[0] == "applications" && suffix == "" && r.Method == "PUT":
		var v controlplane.AIApplicationConfig
		err = aiDecode(w, r, &v)
		if err == nil {
			data, err = web.aiGateway.SaveApplication(ctx, id, v)
		}
	case parts[0] == "applications" && suffix == "probe" && r.Method == "POST":
		data, err = web.aiGateway.Probe(ctx, id)
	case parts[0] == "accesses" && suffix == "" && r.Method == "GET":
		data, err = web.aiGateway.GetAccess(ctx, id)
	case parts[0] == "accesses" && suffix == "" && r.Method == "PUT":
		var v controlplane.AIAccessConfig
		err = aiDecode(w, r, &v)
		if err == nil {
			data, err = web.aiGateway.SaveAccess(ctx, id, v)
		}
	case parts[0] == "accesses" && suffix == "keys" && r.Method == "GET":
		data, err = web.aiGateway.Keys(ctx, id)
	case parts[0] == "accesses" && suffix == "keys" && r.Method == "POST":
		var v controlplane.AIKeyRequest
		err = aiDecode(w, r, &v)
		if err == nil {
			data, err = web.aiGateway.CreateKey(ctx, id, v)
		}
	case parts[0] == "accesses" && len(parts) == 4 && parts[2] == "keys" && r.Method == "DELETE":
		keyID, e := strconv.ParseUint(parts[3], 10, 32)
		if e != nil || keyID == 0 {
			err = controlplane.ErrAIInvalid
		} else {
			err = web.aiGateway.RevokeKey(ctx, id, uint(keyID))
		}
	case parts[0] == "accesses" && suffix == "requests" && r.Method == "GET":
		data, err = web.aiGateway.Requests(ctx, id)
	case parts[0] == "accesses" && len(parts) == 5 && parts[2] == "keys" && parts[4] == "quota" && r.Method == "PUT":
		keyID, e := strconv.ParseUint(parts[3], 10, 32)
		var v struct {
			TokenLimit json.RawMessage `json:"token_limit"`
		}
		err = aiDecode(w, r, &v)
		if e != nil || keyID == 0 || len(v.TokenLimit) == 0 {
			err = controlplane.ErrAIInvalid
		}
		if err == nil {
			var limit *int64
			if json.Unmarshal(v.TokenLimit, &limit) != nil {
				err = controlplane.ErrAIInvalid
			} else {
				err = web.aiGateway.UpdateKeyQuota(ctx, id, uint(keyID), limit)
			}
		}
	case parts[0] == "accesses" && suffix == "usage" && r.Method == "GET":
		data, err = web.aiGateway.TokenUsage(ctx, id)
	default:
		aiError(w, 405)
		return
	}
	if err != nil {
		aiError(w, aiStatus(err))
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}

// handleAIInference exposes protocol-native JSON/SSE, never dashboard cookies.
// @Summary Call an authorized internal model through its Liaison connector
// @Router /api/v1/ai/accesses/{id}/v1/models [get]
// @Router /api/v1/ai/accesses/{id}/v1/chat/completions [post]
// @Success 200 {object} map[string]interface{}
func (web *web) handleAIInference(w http.ResponseWriter, r *http.Request, id uint, operation string, debug bool) {
	if !(operation == "v1/models" && r.Method == "GET" || operation == "v1/chat/completions" && r.Method == "POST") {
		aiError(w, 405)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Minute)
	defer cancel()
	var grant *controlplane.AIGrant
	var err error
	if debug {
		grant, err = web.aiGateway.DebugGrant(ctx, id)
	} else {
		secret, ok := bearerToken(r)
		if !ok {
			aiError(w, 401)
			return
		}
		grant, err = web.aiGateway.Grant(ctx, id, secret)
	}
	if err != nil {
		status := aiStatus(err)
		if status == 403 && !debug {
			status = 401
		}
		aiError(w, status)
		return
	}
	defer grant.Upstream.Close()
	if operation == "v1/models" {
		items := make([]map[string]any, 0, len(grant.Models))
		for _, alias := range aigateway.ModelAliases(grant.Models) {
			items = append(items, map[string]any{"id": alias, "object": "model", "created": 0, "owned_by": "liaison"})
		}
		writeJSON(w, 200, map[string]any{"object": "list", "data": items})
		return
	}
	select {
	case web.aiSlots <- struct{}{}:
		defer func() { <-web.aiSlots }()
	default:
		w.Header().Set("Retry-After", "2")
		aiError(w, 429)
		return
	}
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
	if err != nil {
		aiError(w, 413)
		return
	}
	prepared, err := aigateway.Prepare(raw, grant.Models, grant.Protocol)
	if err != nil {
		aiError(w, aiStatus(err))
		return
	}
	random := make([]byte, 16)
	if _, err = rand.Read(random); err != nil {
		aiError(w, 500)
		return
	}
	record := &model.AIRequest{RequestID: hex.EncodeToString(random), KeyID: grant.KeyID, ProxyID: id, UserID: grant.UserID, Model: prepared.Alias, Status: 502}
	w.Header().Set("X-Request-ID", record.RequestID)
	if err = web.aiGateway.CheckKeyQuota(ctx, grant); err != nil {
		switch {
		case errors.Is(err, controlplane.ErrAITokenQuota):
			aiError(w, 429, "TOKEN_QUOTA_EXHAUSTED")
		case errors.Is(err, controlplane.ErrAITokenUsage):
			aiError(w, 429, "TOKEN_USAGE_UNCONFIRMED")
		default:
			aiError(w, aiStatus(err))
		}
		return
	}
	if grant.Metered && prepared.Stream && grant.Protocol == "openai-compatible" {
		if err = aigateway.IncludeStreamUsage(&prepared); err != nil {
			aiError(w, 400)
			return
		}
	}
	started := time.Now()
	usage := aigateway.Usage{}
	defer func() {
		record.DurationMS = time.Since(started).Milliseconds()
		record.InputTokens = usage.Input
		record.OutputTokens = usage.Output
		record.Complete = usage.Complete
		if ctx.Err() != nil {
			record.Status = 499
		}
		auditCtx, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := web.aiGateway.Record(auditCtx, record); err != nil {
			slog.Error("AI API audit write failed", "request_id", record.RequestID)
		}
	}()
	// Revoke/disable cancels a running stream within two seconds; never retries.
	watcherDone := make(chan struct{})
	defer close(watcherDone)
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-watcherDone:
				return
			case <-ctx.Done():
				return
			case <-ticker.C:
				if grant.Revalidate(ctx) != nil {
					cancel()
					return
				}
			}
		}
	}()
	resp, err := grant.Upstream.RequestProtocol(ctx, "POST", prepared.Operation, grant.UpstreamKey, grant.Protocol, bytes.NewReader(prepared.Body))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			record.Status = 504
			aiError(w, 504)
			return
		}
		aiError(w, 502)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		status := 502
		reason := "UPSTREAM_ERROR"
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			reason = "UPSTREAM_AUTHENTICATION_FAILED"
		}
		if resp.StatusCode == 404 {
			reason = "UPSTREAM_MODEL_OR_ENDPOINT_NOT_FOUND"
		}
		if resp.StatusCode == 429 {
			status = 429
			reason = "UPSTREAM_RATE_LIMITED"
			w.Header().Set("Retry-After", "2")
		}
		record.Status = status
		aiError(w, status, reason)
		return
	}
	if prepared.Stream {
		if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
			aiError(w, 502)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("X-Accel-Buffering", "no")
		controller := http.NewResponseController(w)
		err = aigateway.RelaySSE(resp.Body, grant.Protocol, prepared.Alias, func(data []byte) error {
			if _, e := w.Write(data); e != nil {
				return e
			}
			return controller.Flush()
		}, &usage)
		if err != nil {
			// Stream already started: emit a sanitized error, never a successful DONE.
			if _, e := io.WriteString(w, "data: {\"error\":{\"type\":\"upstream_error\",\"message\":\"Stream interrupted\"}}\n\n"); e == nil {
				if e = controller.Flush(); e != nil {
					cancel()
				}
			}
			return
		}
	} else {
		body, e := io.ReadAll(io.LimitReader(resp.Body, (8<<20)+1))
		if e != nil || len(body) > 8<<20 {
			aiError(w, 502)
			return
		}
		body, e = aigateway.RewriteJSON(body, grant.Protocol, prepared.Alias, &usage)
		if e != nil {
			aiError(w, 502)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if _, e = w.Write(body); e != nil {
			usage.Complete = false
			return
		}
	}
	record.Status = 200
}
