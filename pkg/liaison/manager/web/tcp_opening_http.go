package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
)

// Bound concurrent control operations independently of per-access locks.
var tcpOpeningHTTPRequests = make(chan struct{}, 64)

func (web *web) handleTCPOpeningHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	fail := func(status int, message string) {
		writeJSON(w, status, map[string]any{"code": status, "message": message})
	}
	if _, ok := bearerToken(r); !ok {
		writeUnauthorized(w)
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if web.iamService.RequireResourcePermission(user, "accesses", "use") != nil {
		fail(http.StatusForbidden, "无权使用此访问")
		return
	}
	if !web.checkWebSSHOrigin(r) || r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		fail(http.StatusForbidden, "跨站请求被拒绝")
		return
	}
	select {
	case tcpOpeningHTTPRequests <- struct{}{}:
		defer func() { <-tcpOpeningHTTPRequests }()
	default:
		fail(503, "TCP_OPENING_BUSY")
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/proxies/"), "/")
	if r.URL.RawQuery != "" || r.URL.RawPath != "" || len(parts) < 2 || len(parts) > 3 || parts[1] != "tcp_openings" {
		fail(400, "请求路径无效")
		return
	}
	id, err := strconv.ParseUint(parts[0], 10, 32)
	if err != nil || id == 0 {
		fail(400, "访问 ID 无效")
		return
	}
	var requestID, operation string
	if len(parts) == 2 && r.Method == http.MethodPost {
		var body struct {
			RequestID string `json:"request_id"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&body) != nil {
			fail(400, "请求内容无效")
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			fail(400, "请求内容无效")
			return
		}
		requestID, operation = body.RequestID, "open"
	} else if len(parts) == 3 && (r.Method == http.MethodGet || r.Method == http.MethodDelete) {
		requestID, operation = parts[2], "get"
		if r.Method == http.MethodDelete {
			operation = "revoke"
		}
	} else {
		fail(405, "请求方法无效")
		return
	}
	ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), "user_id", user.ID), 15*time.Second)
	defer cancel()
	data, err := web.controlPlane.TCPOpening(ctx, uint(id), requestID, operation)
	if err != nil {
		var expected *controlplane.HTTPError
		if errors.As(err, &expected) {
			fail(expected.Status(), expected.Message())
			return
		}
		fail(503, "TCP_OPENING_UNAVAILABLE")
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}
