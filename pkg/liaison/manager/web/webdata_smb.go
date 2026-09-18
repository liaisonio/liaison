package web

import (
	"context"
	"errors"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/manager/smbfiles"
)

func (web *web) openWebDataSMB(ctx context.Context, s *webDataSession, password string) error {
	if !smbfiles.ValidShare(s.database) || s.connectionParams != "" || s.authMechanism != "" || s.tlsMode != "" && s.tlsMode != "disable" {
		return smbfiles.ErrInvalid
	}
	raw, target, err := web.controlPlane.OpenWebDataStream(ctx, s.proxyID)
	if err != nil {
		return err
	}
	if target.Protocol != "smb" || target.ApplicationID != s.target.ApplicationID || target.TargetHost != s.target.TargetHost || target.TargetPort != s.target.TargetPort {
		raw.Close()
		return smbfiles.ErrInvalid
	}
	files, err := smbfiles.New(ctx, raw, target.TargetHost, s.username, password, s.schema, s.database)
	if err != nil {
		return err
	}
	if _, err = files.List(ctx, "/"); err != nil {
		files.Close()
		return err
	}
	s.smbClient = files
	return nil
}

// handleWebSMBFilesHTTP serves bounded, read-only operations in one SMB share.
// @Summary Browse or download files in an authorized SMB session
// @Router /api/v1/webdata/sessions/{token}/smb/{action} [get]
// @Success 200 {object} map[string]interface{}
func (web *web) handleWebSMBFilesHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	action := path.Base(r.URL.Path)
	if action != "list" && action != "preview" && action != "download" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, 405, map[string]any{"code": 405, "message": "method not allowed"})
		return
	}
	actor, err := web.fileActor(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if web.iamService.RequireFeature(actor, iam.FeatureFilesRead) != nil {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "file access denied"})
		return
	}
	token, err := parseWebDataSessionToken(r, "/smb/"+action)
	if err != nil {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "invalid session"})
		return
	}
	s, ok := web.webData.get(token)
	if !ok || s.userID != actor.ID || s.protocol != "smb" {
		writeJSON(w, 401, map[string]any{"code": 401, "message": "invalid or expired session"})
		return
	}
	ctx, cancel := context.WithTimeout(context.WithValue(r.Context(), "user_id", actor.ID), 60*time.Second)
	defer cancel()
	if web.ensureWebDataSessionActive(ctx, s) != nil {
		writeJSON(w, 409, map[string]any{"code": 409, "message": "access unavailable"})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.smbClient == nil {
		writeJSON(w, 409, map[string]any{"code": 409, "message": "session closed"})
		return
	}
	name := r.URL.Query().Get("path")
	if name == "" {
		name = "/"
	}
	started := time.Now()
	var data any
	var bytes []byte
	switch action {
	case "list":
		data, err = s.smbClient.List(ctx, name)
	case "preview":
		var text string
		text, err = s.smbClient.Preview(ctx, name)
		data = map[string]string{"text": text}
	case "download":
		bytes, err = s.smbClient.Read(ctx, name, smbfiles.TransferLimit)
	}
	safeError := ""
	if err != nil {
		safeError = "SMB operation failed"
	}
	web.recordWebDataAudit(r, s.target, actor.ID, "smb_"+action, "smb", s.database, webDataStatementPreview(name), err == nil, int64(len(bytes)), time.Since(started).Milliseconds(), safeError)
	if err != nil {
		status := 502
		if errors.Is(err, smbfiles.ErrInvalid) {
			status = 400
		}
		if errors.Is(err, smbfiles.ErrLimit) {
			status = 413
		}
		writeJSON(w, status, map[string]any{"code": status, "reason": "SMB_FAILED", "message": "SMB operation failed; check path, size and permissions, or reconnect"})
		return
	}
	if action == "download" {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(strings.TrimSuffix(name, "/"))}))
		if _, err = w.Write(bytes); err != nil {
			return
		}
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}
