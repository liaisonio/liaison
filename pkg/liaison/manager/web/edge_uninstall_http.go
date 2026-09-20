package web

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type edgeUninstallService interface {
	CreateEdgeUninstall(context.Context, uint64, string, string) (*model.EdgeUninstallTask, error)
	GetEdgeUninstall(context.Context, uint64, string) (*model.EdgeUninstallTask, error)
	ReportEdgeUninstall(context.Context, string, string, string) error
}

func uninstallHTTPError(w http.ResponseWriter, err error) {
	code := 500
	if errors.Is(err, iam.ErrForbidden) {
		code = 403
	} else if e := kerrors.FromError(mapServiceError(err)); e != nil && e.Code >= 400 && e.Code < 500 {
		code = int(e.Code)
	}
	writeJSON(w, code, map[string]any{"code": code, "message": "uninstall unavailable"})
}

func decodeUninstallBody(w http.ResponseWriter, r *http.Request, v any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(v); err != nil {
		return err
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return errors.New("invalid request body")
	}
	return nil
}

func (web *web) handleEdgeUninstallHTTP(w http.ResponseWriter, r *http.Request) {
	expected := http.MethodPost
	if agentPathValue(r, "task") != "" {
		expected = http.MethodGet
	}
	if r.Method != expected {
		w.Header().Set("Allow", expected)
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	id, err := strconv.ParseUint(agentPathValue(r, "id"), 10, 64)
	if err != nil || id == 0 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid connector id"})
		return
	}
	service, ok := web.controlPlane.(edgeUninstallService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "uninstall unavailable"})
		return
	}
	ctx := context.WithValue(r.Context(), "user", actor)
	ctx = context.WithValue(ctx, "user_id", actor.ID)
	var task *model.EdgeUninstallTask
	code := 200
	if expected == http.MethodPost {
		var body struct {
			Name     string `json:"confirm_name"`
			Instance string `json:"instance_id"`
		}
		if err = decodeUninstallBody(w, r, &body); err != nil {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid request body"})
			return
		}
		task, err = service.CreateEdgeUninstall(ctx, id, body.Name, body.Instance)
		code = 202
	} else {
		task, err = service.GetEdgeUninstall(ctx, id, agentPathValue(r, "task"))
	}
	if err != nil {
		uninstallHTTPError(w, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, code, map[string]any{"code": code, "message": "success", "data": task})
}

// This callback uses a short-lived, single-task token, never a user/PAT token.
func (web *web) handleEdgeUninstallResultHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	token, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !ok || len(token) != 64 {
		writeUnauthorized(w)
		return
	}
	var body struct {
		Status string `json:"status"`
		Reason string `json:"reason"`
	}
	if err := decodeUninstallBody(w, r, &body); err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid request body"})
		return
	}
	service, ok := web.controlPlane.(edgeUninstallService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "uninstall unavailable"})
		return
	}
	if err := service.ReportEdgeUninstall(r.Context(), agentPathValue(r, "task"), token, body.Status); err != nil {
		uninstallHTTPError(w, err)
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success"})
}
