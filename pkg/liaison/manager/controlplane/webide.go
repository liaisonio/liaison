package controlplane

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

type WebIDEApplicationInput struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	EdgeID         uint64 `json:"edge_id"`
	InstallationID string `json:"installation_id"`
	Mode           string `json:"mode"`
	Port           int    `json:"port"`
}
type WebIDEAccessInput struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	ApplicationID string `json:"application_id"`
	Enabled       bool   `json:"enabled"`
	Project       string `json:"project"`
}
type WebIDEList[T any] struct {
	Items []T   `json:"items"`
	Total int64 `json:"total"`
}
type webIDECaller interface {
	WebIDE(context.Context, uint64, proto.WebIDERequest) (proto.WebIDEResult, error)
}

type WebIDEManagedInstance struct {
	proto.WebIDEInstance
	AccessName string `json:"access_name"`
	Linked     bool   `json:"linked"`
}

func (cp *controlPlane) WebIDEApplicationInstances(ctx context.Context, id string) ([]WebIDEManagedInstance, error) {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return nil, err
	}
	app, err := cp.repo.GetWebIDEApplication(ctx, owner, id)
	if err != nil {
		return nil, err
	}
	if app.Mode != "managed" {
		return nil, badRequest("EXTERNAL_SERVICE", "External service is not managed")
	}
	result, err := cp.WebIDEControl(ctx, app.EdgeID, proto.WebIDERequest{Action: "instances"})
	if err != nil {
		return nil, err
	}
	if result.Status != "ok" {
		return nil, errors.New("IDE instances unavailable")
	}
	rows := []WebIDEManagedInstance{}
	for _, instance := range result.Instances {
		if instance.InstallationID != app.InstallationID {
			continue
		}
		if instance.ApplicationID != "" && instance.ApplicationID != app.ID {
			continue
		}
		access, err := cp.repo.GetWebIDEAccess(ctx, owner, instance.AccessID)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, err
		}
		if err == nil && access.ApplicationID != app.ID {
			continue
		}
		row := WebIDEManagedInstance{WebIDEInstance: instance, Linked: err == nil}
		if row.Linked {
			row.AccessName = access.Name
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (cp *controlPlane) WebIDEApplicationInstanceAction(ctx context.Context, appID, instanceID, action string) (WebIDEManagedInstance, error) {
	if action != "recover" && action != "stop" {
		return WebIDEManagedInstance{}, badRequest("INVALID_ACTION", "Invalid instance action")
	}
	rows, err := cp.WebIDEApplicationInstances(ctx, appID)
	if err != nil {
		return WebIDEManagedInstance{}, err
	}
	for _, row := range rows {
		if row.ID != instanceID {
			continue
		}
		owner, err := cp.webIDEOwner(ctx, 0)
		if err != nil {
			return row, err
		}
		app, err := cp.repo.GetWebIDEApplication(ctx, owner, appID)
		if err != nil {
			return row, err
		}
		if action == "stop" {
			result, err := cp.WebIDEControl(ctx, app.EdgeID, proto.WebIDERequest{Action: "stop", AccessID: row.AccessID, InstanceID: row.ID})
			if err != nil {
				return row, err
			}
			if result.Status != "ok" {
				return row, errors.New("IDE instance stop failed")
			}
			row.Status = "stopped"
			return row, nil
		}
		if row.Linked {
			return row, nil
		}
		_, err = cp.SaveWebIDEAccess(ctx, "", WebIDEAccessInput{ID: row.AccessID, Name: app.Name, ApplicationID: app.ID, Enabled: true, Project: row.Project})
		if err != nil {
			return row, err
		}
		row.Linked = true
		row.AccessName = app.Name
		return row, nil
	}
	return WebIDEManagedInstance{}, gorm.ErrRecordNotFound
}

func webIDEID(id string) bool {
	if len(id) != 32 {
		return false
	}
	_, err := hex.DecodeString(id)
	return err == nil
}
func webIDEName(name string) bool {
	return strings.TrimSpace(name) != "" && len(name) <= 120 && !strings.ContainsRune(name, 0)
}
func (cp *controlPlane) webIDEOwner(ctx context.Context, edge uint64) (uint, error) {
	actor, ok := actorUserID(ctx)
	if !ok {
		return 0, iam.ErrForbidden
	}
	if edge != 0 && !cp.ownsAgentConnector(actor, edge) {
		return 0, iam.ErrForbidden
	}
	return actor, nil
}

func (cp *controlPlane) WebIDEConnectors(ctx context.Context) ([]AgentConnector, error) {
	actor, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return nil, err
	}
	ids, err := cp.repo.ListIAMResourceIDsForSubject(resourceConnector, model.IAMSubjectUser, actor)
	if err != nil {
		return nil, err
	}
	rows := []AgentConnector{}
	for _, id := range ids {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if !cp.ownsAgentConnector(actor, id) {
			continue
		}
		if len(rows) >= 100 {
			break
		}
		edge, err := cp.repo.GetEdge(id)
		if err != nil {
			continue
		}
		rows = append(rows, AgentConnector{ID: id, Name: edge.Name, Online: edge.Online == model.EdgeOnlineStatusOnline && edge.Status == model.EdgeStatusRunning})
	}
	return rows, nil
}
func (cp *controlPlane) WebIDEControl(ctx context.Context, edge uint64, q proto.WebIDERequest) (proto.WebIDEResult, error) {
	owner, err := cp.webIDEOwner(ctx, edge)
	if err != nil {
		return proto.WebIDEResult{}, err
	}
	if edge == 0 {
		return proto.WebIDEResult{}, iam.ErrForbidden
	}
	q.Version = 1
	q.OwnerID = strconv.FormatUint(uint64(owner), 10)
	q.RequestID = proto.WebIDERequestID(ctx)
	if !q.Valid() {
		return proto.WebIDEResult{}, badRequest("INVALID_WEBIDE_REQUEST", "Invalid IDE request")
	}
	caller, ok := cp.frontierBound.(webIDECaller)
	if !ok {
		return proto.WebIDEResult{}, errors.New("IDE connector capability unavailable")
	}
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()
	return caller.WebIDE(ctx, edge, q)
}
func (cp *controlPlane) WebIDEApplications(ctx context.Context, page, size int) (WebIDEList[model.WebIDEApplication], error) {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return WebIDEList[model.WebIDEApplication]{}, err
	}
	rows, total, err := cp.repo.WebIDEApplications(ctx, owner, page, size)
	return WebIDEList[model.WebIDEApplication]{rows, total}, err
}
func (cp *controlPlane) SaveWebIDEApplication(ctx context.Context, id string, in WebIDEApplicationInput) (*model.WebIDEApplication, error) {
	if !webIDEName(in.Name) {
		return nil, badRequest("INVALID_APPLICATION", "Invalid name")
	}
	owner, err := cp.webIDEOwner(ctx, in.EdgeID)
	if err != nil {
		return nil, err
	}
	if id != "" {
		row, err := cp.repo.GetWebIDEApplication(ctx, owner, id)
		if err != nil {
			return nil, err
		}
		if _, err = cp.webIDEOwner(ctx, row.EdgeID); err != nil {
			return nil, err
		}
		row.Name = strings.TrimSpace(in.Name)
		return row, cp.repo.SaveWebIDEApplication(ctx, row, false)
	}
	if !webIDEID(in.ID) || in.EdgeID == 0 {
		return nil, badRequest("INVALID_APPLICATION", "Invalid application")
	}
	if in.Mode == "external" {
		in.InstallationID = "external-" + strconv.Itoa(in.Port)
	}
	if previous, e := cp.repo.GetWebIDEApplication(ctx, owner, in.ID); e == nil {
		if previous.EdgeID != in.EdgeID || previous.InstallationID != in.InstallationID || previous.Mode != in.Mode || previous.Port != in.Port {
			return nil, conflict("APPLICATION_CONFLICT", "Application identity changed")
		}
		return previous, nil
	}
	switch in.Mode {
	case "managed":
		if in.Port != 0 {
			return nil, badRequest("INVALID_APPLICATION", "Invalid managed port")
		}
		result, err := cp.WebIDEControl(ctx, in.EdgeID, proto.WebIDERequest{Action: "discover"})
		if err != nil {
			return nil, err
		}
		found := false
		for _, i := range result.Installations {
			if i.ID == in.InstallationID {
				found = true
			}
		}
		if !found {
			return nil, badRequest("INSTALLATION_UNAVAILABLE", "Discover an available installation first")
		}
	case "external":
		if in.Port < 1 || in.Port > 65535 {
			return nil, badRequest("INVALID_PORT", "Invalid port")
		}
		in.InstallationID = "external-" + strconv.Itoa(in.Port)
	default:
		return nil, badRequest("INVALID_APPLICATION", "Invalid application mode")
	}
	row := &model.WebIDEApplication{ID: in.ID, OwnerID: owner, EdgeID: in.EdgeID, Name: strings.TrimSpace(in.Name), InstallationID: in.InstallationID, Mode: in.Mode, Port: in.Port}
	return row, cp.repo.SaveWebIDEApplication(ctx, row, true)
}
func (cp *controlPlane) DeleteWebIDEApplication(ctx context.Context, id string) error {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return err
	}
	row, err := cp.repo.GetWebIDEApplication(ctx, owner, id)
	if err != nil {
		return err
	}
	if _, err = cp.webIDEOwner(ctx, row.EdgeID); err != nil {
		return err
	}
	if row.Mode == "managed" {
		instances, listErr := cp.WebIDEApplicationInstances(ctx, id)
		if listErr != nil {
			return listErr
		}
		for _, instance := range instances {
			if instance.Status != "stopped" {
				return conflict("APPLICATION_IN_USE", "Stop managed instances before removing the application")
			}
		}
	}
	if err = cp.repo.DeleteWebIDEApplication(ctx, owner, id); errors.Is(err, dao.ErrWebIDEInUse) {
		return conflict("APPLICATION_IN_USE", "Remove linked accesses first")
	}
	return err
}

type WebIDEAccessSummary struct {
	model.WebIDEAccess
	ApplicationName    string `json:"application_name"`
	ApplicationMode    string `json:"application_mode"`
	ConnectorName      string `json:"connector_name"`
	ConnectorOnline    bool   `json:"connector_online"`
	ConnectorAvailable bool   `json:"connector_available"`
}

func (cp *controlPlane) WebIDEAccesses(ctx context.Context, page, size int, names ...string) (WebIDEList[WebIDEAccessSummary], error) {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return WebIDEList[WebIDEAccessSummary]{}, err
	}
	rows, total, err := cp.repo.WebIDEAccesses(ctx, owner, page, size, names...)
	if err != nil {
		return WebIDEList[WebIDEAccessSummary]{}, err
	}
	summaries := make([]WebIDEAccessSummary, 0, len(rows))
	apps := map[string]*model.WebIDEApplication{}
	devices := map[uint64]*model.Edge{}
	for _, row := range rows {
		summary := WebIDEAccessSummary{WebIDEAccess: row}
		app, loaded := apps[row.ApplicationID]
		if !loaded {
			app, err = cp.repo.GetWebIDEApplication(ctx, owner, row.ApplicationID)
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return WebIDEList[WebIDEAccessSummary]{}, err
			}
			if err != nil {
				app = nil
			}
			apps[row.ApplicationID] = app
		}
		if app != nil {
			summary.ApplicationName = app.Name
			summary.ApplicationMode = app.Mode
			edge, loaded := devices[app.EdgeID]
			if !loaded {
				if cp.ownsAgentConnector(owner, app.EdgeID) {
					edge, err = cp.repo.GetEdge(app.EdgeID)
					if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
						return WebIDEList[WebIDEAccessSummary]{}, err
					}
					if err != nil {
						edge = nil
					}
				}
				devices[app.EdgeID] = edge
			}
			if edge != nil {
				summary.ConnectorAvailable = true
				summary.ConnectorName = edge.Name
				summary.ConnectorOnline = edge.Online == model.EdgeOnlineStatusOnline && edge.Status == model.EdgeStatusRunning
			}
		}
		summaries = append(summaries, summary)
	}
	return WebIDEList[WebIDEAccessSummary]{summaries, total}, nil
}
func (cp *controlPlane) SaveWebIDEAccess(ctx context.Context, id string, in WebIDEAccessInput) (*model.WebIDEAccess, error) {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return nil, err
	}
	if !webIDEName(in.Name) || !webIDEID(in.ApplicationID) || len(in.Project) > 4096 || strings.ContainsRune(in.Project, 0) {
		return nil, badRequest("INVALID_ACCESS", "Invalid access")
	}
	app, err := cp.repo.GetWebIDEApplication(ctx, owner, in.ApplicationID)
	if err != nil {
		return nil, err
	}
	if _, err = cp.webIDEOwner(ctx, app.EdgeID); err != nil {
		return nil, err
	}
	create := id == ""
	if create {
		id = in.ID
	}
	if !webIDEID(id) {
		return nil, badRequest("INVALID_ACCESS", "Invalid access ID")
	}
	if previous, e := cp.repo.GetWebIDEAccess(ctx, owner, id); e == nil {
		if previous.ApplicationID != in.ApplicationID {
			return nil, conflict("ACCESS_CONFLICT", "Application association is immutable")
		}
		if create {
			return previous, nil
		}
	}
	row := &model.WebIDEAccess{ID: id, OwnerID: owner, ApplicationID: in.ApplicationID, Name: strings.TrimSpace(in.Name), Enabled: in.Enabled, Project: in.Project}
	return row, cp.repo.SaveWebIDEAccess(ctx, row, create)
}
func (cp *controlPlane) DeleteWebIDEAccess(ctx context.Context, id string) error {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return err
	}
	// Removing an entry revokes access only; no device files/processes are deleted.
	return cp.repo.DeleteWebIDEAccess(ctx, owner, id)
}

func (cp *controlPlane) WebIDETarget(ctx context.Context, id string) (*model.WebIDEAccess, *model.WebIDEApplication, error) {
	return cp.webIDETarget(ctx, id, true)
}

// Disabling an entry revokes use, not the owner's ability to stop its process.
// All paths still check access, application and connector ownership.
func (cp *controlPlane) webIDETarget(ctx context.Context, id string, requireEnabled bool) (*model.WebIDEAccess, *model.WebIDEApplication, error) {
	owner, err := cp.webIDEOwner(ctx, 0)
	if err != nil {
		return nil, nil, err
	}
	access, err := cp.repo.GetWebIDEAccess(ctx, owner, id)
	if err != nil {
		return nil, nil, err
	}
	if requireEnabled && !access.Enabled {
		return nil, nil, iam.ErrForbidden
	}
	app, err := cp.repo.GetWebIDEApplication(ctx, owner, access.ApplicationID)
	if err != nil {
		return nil, nil, err
	}
	if _, err = cp.webIDEOwner(ctx, app.EdgeID); err != nil {
		return nil, nil, err
	}
	return access, app, nil
}
func (cp *controlPlane) WebIDERuntime(ctx context.Context, id string, q proto.WebIDERequest) (proto.WebIDEResult, error) {
	if q.Action != "start" && q.Action != "stop" {
		return proto.WebIDEResult{}, badRequest("INVALID_ACTION", "Invalid IDE action")
	}
	access, app, err := cp.webIDETarget(ctx, id, q.Action != "stop")
	if err != nil {
		return proto.WebIDEResult{}, err
	}
	if app.Mode == "external" {
		if q.Action == "stop" {
			return proto.WebIDEResult{}, badRequest("EXTERNAL_SERVICE", "External service is not managed")
		}
		return proto.WebIDEResult{Version: 1, Status: "ok", Instances: []proto.WebIDEInstance{{ID: app.ID, AccessID: id, InstallationID: app.InstallationID, Project: q.Project, Status: "external"}}}, nil
	}
	q.AccessID = id
	if q.Action == "start" {
		q.InstallationID = app.InstallationID
		q.ApplicationID = app.ID
		if q.Project == "" {
			q.Project = access.Project
		}
	}
	return cp.WebIDEControl(ctx, app.EdgeID, q)
}
func (cp *controlPlane) OpenWebIDEStream(ctx context.Context, id, instance string) (net.Conn, error) {
	access, app, err := cp.WebIDETarget(ctx, id)
	if err != nil {
		return nil, err
	}
	if cp.frontierBound == nil {
		return nil, errors.New("connector unavailable")
	}
	dst := proto.Dst{}
	if app.Mode == "external" {
		if instance != app.ID {
			return nil, iam.ErrForbidden
		}
		dst.Addr = net.JoinHostPort("127.0.0.1", strconv.Itoa(app.Port))
	} else {
		dst.WebIDE = &proto.WebIDEStream{OwnerID: fmt.Sprint(access.OwnerID), AccessID: id, InstanceID: instance}
	}
	stream, err := cp.frontierBound.OpenStream(ctx, app.EdgeID)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(dst)
	if err != nil {
		stream.Close()
		return nil, err
	}
	frame := make([]byte, 4, len(data)+4)
	binary.BigEndian.PutUint32(frame, uint32(len(data)))
	frame = append(frame, data...)
	if _, err = stream.Write(frame); err != nil {
		stream.Close()
		return nil, err
	}
	return stream, nil
}
