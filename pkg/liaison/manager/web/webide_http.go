package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	kerrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

type webIDEService interface {
	WebIDEApplicationInstances(context.Context, string) ([]controlplane.WebIDEManagedInstance, error)
	WebIDEApplicationInstanceAction(context.Context, string, string, string) (controlplane.WebIDEManagedInstance, error)
	WebIDEConnectors(context.Context) ([]controlplane.AgentConnector, error)
	WebIDEControl(context.Context, uint64, proto.WebIDERequest) (proto.WebIDEResult, error)
	WebIDEApplications(context.Context, int, int) (controlplane.WebIDEList[model.WebIDEApplication], error)
	SaveWebIDEApplication(context.Context, string, controlplane.WebIDEApplicationInput) (*model.WebIDEApplication, error)
	DeleteWebIDEApplication(context.Context, string) error
	WebIDEAccesses(context.Context, int, int, ...string) (controlplane.WebIDEList[controlplane.WebIDEAccessSummary], error)
	SaveWebIDEAccess(context.Context, string, controlplane.WebIDEAccessInput) (*model.WebIDEAccess, error)
	DeleteWebIDEAccess(context.Context, string) error
	WebIDERuntime(context.Context, string, proto.WebIDERequest) (proto.WebIDEResult, error)
	WebIDETarget(context.Context, string) (*model.WebIDEAccess, *model.WebIDEApplication, error)
	OpenWebIDEStream(context.Context, string, string) (net.Conn, error)
}

func decodeWebIDE(w http.ResponseWriter, r *http.Request, v any) bool {
	d := json.NewDecoder(http.MaxBytesReader(w, r.Body, 16<<10))
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || d.Decode(new(any)) != io.EOF {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "Invalid WebIDE request"})
		return false
	}
	return true
}

// @Summary Manage owned WebIDE applications, accesses and device runtimes
// @Router /api/v1/webide/{resource} [get]
// @Router /api/v1/webide/{resource} [post]
// @Router /api/v1/webide/{resource}/{id} [put]
// @Router /api/v1/webide/{resource}/{id} [delete]
// @Success 200 {object} proto.WebIDEResult
func (web *web) handleWebIDEHTTP(w http.ResponseWriter, r *http.Request) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "IDE unavailable"})
		return
	}
	requestID := hex.EncodeToString(random[:])
	w.Header().Set("X-Request-ID", requestID)
	r = r.WithContext(proto.WithWebIDERequestID(r.Context(), requestID))
	tracked := &ideResponseStatus{ResponseWriter: w, status: 200}
	w = tracked
	started := time.Now()
	defer func() {
		slog.InfoContext(r.Context(), "IDE API operation", "request_id", requestID, "stage", "manager", "method", r.Method, "status", tracked.status, "duration_ms", time.Since(started).Milliseconds())
	}()
	w.Header().Set("Cache-Control", "no-store")
	actor, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	if actor.Status != model.UserStatusActive || web.iamService == nil {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "Forbidden"})
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/v1/webide/"), "/")
	if len(parts) == 1 && parts[0] == "capabilities" && r.Method == http.MethodGet {
		writeJSON(w, 200, map[string]any{"code": 200, "data": map[string]any{"enabled": web.ideGateway != nil, "ready": web.ideGateway != nil && web.ideGateway.ready(), "agent_integration": false}})
		return
	}
	if web.ideGateway == nil {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "WebIDE is disabled"})
		return
	}
	svc, ok := web.controlPlane.(webIDEService)
	if !ok {
		writeJSON(w, 503, map[string]any{"code": 503, "message": "WebIDE unavailable"})
		return
	}
	resource, verb := "accesses", "read"
	if parts[0] == "applications" {
		resource = "applications"
	}
	if r.Method != http.MethodGet {
		switch r.Method {
		case http.MethodPost:
			verb = "create"
		case http.MethodPut:
			verb = "update"
		case http.MethodDelete:
			verb = "delete"
		}
	}
	if parts[0] == "connectors" {
		resource = "connectors"
		verb = "read"
	}
	if len(parts) == 3 && parts[0] == "accesses" {
		verb = "use"
	}
	if len(parts) == 4 && parts[0] == "applications" && parts[2] == "instances" {
		verb = "update"
	}
	if err = web.iamService.RequireResourcePermission(actor, resource, verb); err != nil {
		writeJSON(w, 403, map[string]any{"code": 403, "message": "Forbidden"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", actor.ID)
	ctx = context.WithValue(ctx, "user", actor)
	var data any
	page, size := 1, 20
	if v := r.URL.Query().Get("page"); v != "" {
		page, _ = strconv.Atoi(v)
	}
	if v := r.URL.Query().Get("page_size"); v != "" {
		size, _ = strconv.Atoi(v)
	}
	if page < 1 || page > 10000 || size < 1 || size > 100 {
		writeJSON(w, 400, map[string]any{"code": 400, "message": "Invalid page"})
		return
	}
	switch {
	case len(parts) == 1 && parts[0] == "connectors" && r.Method == http.MethodGet:
		data, err = svc.WebIDEConnectors(ctx)
	case len(parts) == 2 && parts[0] == "connectors" && r.Method == http.MethodPost:
		var in struct {
			Action    string `json:"action"`
			Directory string `json:"directory"`
		}
		if !decodeWebIDE(w, r, &in) {
			return
		}
		edge, e := strconv.ParseUint(parts[1], 10, 64)
		if e != nil {
			err = e
			break
		}
		if in.Action != "discover" && in.Action != "install" && in.Action != "instances" && in.Action != "directories" {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "Invalid action"})
			return
		}
		if in.Action == "install" {
			if err = web.iamService.RequireResourcePermission(actor, "connectors", "update"); err != nil {
				writeJSON(w, 403, map[string]any{"code": 403, "message": "Forbidden"})
				return
			}
		}
		data, err = svc.WebIDEControl(ctx, edge, proto.WebIDERequest{Action: in.Action, Directory: in.Directory})
	case len(parts) == 1 && parts[0] == "applications" && r.Method == http.MethodGet:
		data, err = svc.WebIDEApplications(ctx, page, size)
	case len(parts) == 1 && parts[0] == "accesses" && r.Method == http.MethodGet:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if len(name) > 480 || strings.ContainsRune(name, 0) {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "Invalid IDE filter"})
			return
		}
		data, err = svc.WebIDEAccesses(ctx, page, size, name)
	case len(parts) == 2 && parts[0] == "accesses" && r.Method == http.MethodGet:
		var access *model.WebIDEAccess
		var application *model.WebIDEApplication
		access, application, err = svc.WebIDETarget(ctx, parts[1])
		data = map[string]any{"access": access, "application": application}
	case (len(parts) == 1 && r.Method == http.MethodPost || len(parts) == 2 && r.Method == http.MethodPut) && parts[0] == "applications":
		var in controlplane.WebIDEApplicationInput
		if !decodeWebIDE(w, r, &in) {
			return
		}
		id := ""
		if len(parts) == 2 {
			id = parts[1]
		}
		data, err = svc.SaveWebIDEApplication(ctx, id, in)
	case (len(parts) == 1 && r.Method == http.MethodPost || len(parts) == 2 && r.Method == http.MethodPut) && parts[0] == "accesses":
		var in controlplane.WebIDEAccessInput
		if !decodeWebIDE(w, r, &in) {
			return
		}
		id := ""
		if len(parts) == 2 {
			id = parts[1]
		}
		key := id
		if key == "" {
			key = in.ID
		}
		err = web.ideGateway.accessOperation(ctx, key, func() error {
			row, saveErr := svc.SaveWebIDEAccess(ctx, id, in)
			data = row
			if saveErr != nil {
				return saveErr
			}
			if row != nil && !row.Enabled {
				return web.ideGateway.releaseEndpoints(row.ID, "")
			}
			return nil
		})
	case len(parts) == 3 && parts[0] == "applications" && parts[2] == "instances" && r.Method == http.MethodGet:
		data, err = svc.WebIDEApplicationInstances(ctx, parts[1])
	case len(parts) == 4 && parts[0] == "applications" && parts[2] == "instances" && r.Method == http.MethodPost:
		var in struct {
			Action string `json:"action"`
		}
		if !decodeWebIDE(w, r, &in) {
			return
		}
		if in.Action == "recover" {
			if err = web.iamService.RequireResourcePermission(actor, "accesses", "create"); err != nil {
				writeJSON(w, 403, map[string]any{"code": 403, "message": "Forbidden"})
				return
			}
		}
		var row controlplane.WebIDEManagedInstance
		instances, listErr := svc.WebIDEApplicationInstances(ctx, parts[1])
		err = listErr
		if err != nil {
			break
		}
		err = errors.New("IDE instance unavailable")
		for _, instance := range instances {
			if instance.ID != parts[3] {
				continue
			}
			err = web.ideGateway.accessOperation(ctx, instance.AccessID, func() error {
				var actionErr error
				row, actionErr = svc.WebIDEApplicationInstanceAction(ctx, parts[1], parts[3], in.Action)
				data = row
				if actionErr == nil && in.Action == "stop" {
					return web.ideGateway.releaseEndpoints(row.AccessID, row.ID)
				}
				return actionErr
			})
			break
		}
	case len(parts) == 2 && parts[0] == "applications" && r.Method == http.MethodDelete:
		err = svc.DeleteWebIDEApplication(ctx, parts[1])
	case len(parts) == 2 && parts[0] == "accesses" && r.Method == http.MethodDelete:
		err = web.ideGateway.accessOperation(ctx, parts[1], func() error {
			if deleteErr := svc.DeleteWebIDEAccess(ctx, parts[1]); deleteErr != nil {
				return deleteErr
			}
			return web.ideGateway.releaseEndpoints(parts[1], "")
		})
	case len(parts) == 3 && parts[0] == "accesses" && parts[2] == "runtime" && r.Method == http.MethodPost:
		var in struct {
			Action     string `json:"action"`
			Project    string `json:"project"`
			InstanceID string `json:"instance_id"`
		}
		if !decodeWebIDE(w, r, &in) {
			return
		}
		err = web.ideGateway.accessOperation(ctx, parts[1], func() error {
			result, runtimeErr := svc.WebIDERuntime(ctx, parts[1], proto.WebIDERequest{Action: in.Action, Project: in.Project, InstanceID: in.InstanceID})
			data = result
			if runtimeErr != nil {
				return runtimeErr
			}
			if in.Action == "stop" && result.Status == "ok" {
				return web.ideGateway.releaseEndpoints(parts[1], in.InstanceID)
			}
			return nil
		})
	case len(parts) == 3 && parts[0] == "accesses" && parts[2] == "launch" && r.Method == http.MethodPost:
		var in struct {
			InstanceID string `json:"instance_id"`
			Project    string `json:"project"`
			Theme      string `json:"theme"`
		}
		if !decodeWebIDE(w, r, &in) {
			return
		}
		if len(in.InstanceID) > 64 || len(in.Project) > 4096 || (in.Theme != "" && in.Theme != "dark" && in.Theme != "light") {
			writeJSON(w, 400, map[string]any{"code": 400, "message": "Invalid instance"})
			return
		}
		_, _, err = svc.WebIDETarget(ctx, parts[1])
		if err != nil {
			break
		}
		token, valid := bearerToken(r)
		if !valid {
			writeUnauthorized(w)
			return
		}
		var target string
		target, err = web.ideGateway.launch(ctx, token, parts[1], in.InstanceID, in.Project, in.Theme)
		data = map[string]string{"url": target}
	default:
		http.NotFound(w, r)
		return
	}
	if err != nil {
		status := 500
		if errors.Is(err, iam.ErrForbidden) {
			status = 403
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			status = 404
		} else if mapped := kerrors.FromError(err); mapped != nil && mapped.Code >= 400 && mapped.Code < 500 {
			status = int(mapped.Code)
		}
		writeJSON(w, status, map[string]any{"code": status, "message": "WebIDE operation unavailable", "request_id": requestID})
		return
	}
	writeJSON(w, 200, map[string]any{"code": 200, "message": "success", "data": data})
}
