package web

import (
	"context"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
)

func (web *web) handleWebDataAuditListHTTP(w http.ResponseWriter, r *http.Request) {
	web.handleAccessAuditListHTTP(w, r)
}

func (web *web) handleAccessAuditListHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	query, err := parseWebDataAuditListQuery(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": err.Error()})
		return
	}
	result, err := web.controlPlane.ListWebDataAuditEntries(ctx, query)
	if err != nil {
		status := webDataHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": result})
}

// @Summary List management audit logs
// @Tags Audit
// @Produce json
// @Param page query int false "Page"
// @Param page_size query int false "Page size"
// @Success 200 {object} map[string]any
// @Failure 400 {object} map[string]any
// @Failure 401 {object} map[string]any
// @Router /api/v1/audits/management [get]
func (web *web) handleManagementAuditListHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	page, err := parsePositiveIntQuery(r.URL.Query().Get("page"), 1, 100000)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid page"})
		return
	}
	pageSize, err := parsePositiveIntQuery(r.URL.Query().Get("page_size"), 20, 500)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid page size"})
		return
	}
	var success *bool
	if raw := strings.TrimSpace(r.URL.Query().Get("success")); raw != "" {
		parsed, parseErr := strconv.ParseBool(raw)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid result"})
			return
		}
		success = &parsed
	}
	startTime, err := parseAuditTimeQuery(r.URL.Query().Get("start_time"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid start time"})
		return
	}
	endTime, err := parseAuditTimeQuery(r.URL.Query().Get("end_time"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid end time"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	result, err := web.controlPlane.ListManagementAudits(ctx, &controlplane.ManagementAuditListQuery{Module: strings.TrimSpace(r.URL.Query().Get("module")), Action: strings.TrimSpace(r.URL.Query().Get("action")), Success: success, Keyword: strings.TrimSpace(r.URL.Query().Get("keyword")), StartTime: startTime, EndTime: endTime, Page: page, PageSize: pageSize})
	if err != nil {
		if errors.Is(err, iam.ErrForbidden) {
			writeIAMError(w, err)
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "failed to load management logs"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": result})
}

func parseWebDataAuditListQuery(r *http.Request) (*controlplane.WebDataAuditListQuery, error) {
	q := r.URL.Query()
	page, err := parsePositiveIntQuery(q.Get("page"), 1, 100000)
	if err != nil {
		return nil, err
	}
	pageSize, err := parsePositiveIntQuery(q.Get("page_size"), 20, 500)
	if err != nil {
		return nil, err
	}
	proxyID, err := parseUintQuery(q.Get("proxy_id"))
	if err != nil {
		return nil, err
	}
	var success *bool
	if raw := strings.TrimSpace(q.Get("success")); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, err
		}
		success = &parsed
	}
	startTime, err := parseAuditTimeQuery(q.Get("start_time"))
	if err != nil {
		return nil, err
	}
	endTime, err := parseAuditTimeQuery(q.Get("end_time"))
	if err != nil {
		return nil, err
	}
	return &controlplane.WebDataAuditListQuery{
		ProxyID:   proxyID,
		Protocol:  strings.TrimSpace(q.Get("protocol")),
		Action:    strings.TrimSpace(q.Get("action")),
		Success:   success,
		Keyword:   strings.TrimSpace(q.Get("keyword")),
		StartTime: startTime,
		EndTime:   endTime,
		Page:      page,
		PageSize:  pageSize,
	}, nil
}

func parsePositiveIntQuery(raw string, fallback, max int) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > max {
		return 0, strconv.ErrSyntax
	}
	return value, nil
}

func parseUintQuery(raw string) (uint, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	value, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, err
	}
	return uint(value), nil
}

func parseAuditTimeQuery(raw string) (*time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if ts, err := time.Parse(time.RFC3339, raw); err == nil {
		return &ts, nil
	}
	if ts, err := time.Parse("2006-01-02 15:04:05", raw); err == nil {
		return &ts, nil
	}
	if ts, err := time.Parse("2006-01-02", raw); err == nil {
		return &ts, nil
	}
	return nil, strconv.ErrSyntax
}
