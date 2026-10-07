package proto

import (
	"context"
	"regexp"
)

const RPCWebIDE = "webide_v1"

// WebIDEStream contains identity, not a caller-controlled socket or address.
type WebIDEStream struct {
	OwnerID    string `json:"owner_id"`
	AccessID   string `json:"access_id"`
	InstanceID string `json:"instance_id"`
}

var webIDEID = regexp.MustCompile(`^[a-zA-Z0-9_-]{1,64}$`)

type WebIDERequest struct {
	Version        int    `json:"version"`
	Action         string `json:"action"`
	OwnerID        string `json:"owner_id"`
	AccessID       string `json:"access_id,omitempty"`
	Project        string `json:"project,omitempty"`
	InstallationID string `json:"installation_id,omitempty"`
	InstanceID     string `json:"instance_id,omitempty"`
	Directory      string `json:"directory,omitempty"`
	ApplicationID  string `json:"application_id,omitempty"`
	RequestID      string `json:"request_id,omitempty"`
}

func (r WebIDERequest) Valid() bool {
	if r.Version != 1 || !webIDEID.MatchString(r.OwnerID) || len(r.Project) > 4096 || len(r.Directory) > 4096 || (r.ApplicationID != "" && !webIDEID.MatchString(r.ApplicationID)) || (r.RequestID != "" && !webIDEID.MatchString(r.RequestID)) {
		return false
	}
	switch r.Action {
	case "discover", "directories", "install", "instances":
		return r.AccessID == "" && r.InstanceID == "" && r.Project == "" && r.InstallationID == ""
	case "start":
		return webIDEID.MatchString(r.AccessID) && webIDEID.MatchString(r.InstallationID) && r.InstanceID == ""
	case "stop":
		return webIDEID.MatchString(r.AccessID) && webIDEID.MatchString(r.InstanceID) && r.Project == "" && r.InstallationID == ""
	default:
		return false
	}
}

type WebIDEInstallation struct {
	ID      string `json:"id"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Path    string `json:"path"`
}
type WebIDEInstance struct {
	ID             string `json:"id"`
	AccessID       string `json:"access_id"`
	InstallationID string `json:"installation_id"`
	Project        string `json:"project"`
	Status         string `json:"status"`
	StartedAt      string `json:"started_at"`
	ApplicationID  string `json:"application_id,omitempty"`
}

type webIDERequestIDKey struct{}

func WithWebIDERequestID(ctx context.Context, id string) context.Context {
	if !webIDEID.MatchString(id) {
		return ctx
	}
	return context.WithValue(ctx, webIDERequestIDKey{}, id)
}
func WebIDERequestID(ctx context.Context) string {
	id, _ := ctx.Value(webIDERequestIDKey{}).(string)
	return id
}

type WebIDEDirectory struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type WebIDEResult struct {
	Version       int                  `json:"version"`
	Status        string               `json:"status"`
	Installations []WebIDEInstallation `json:"installations,omitempty"`
	Instances     []WebIDEInstance     `json:"instances,omitempty"`
	Directories   []WebIDEDirectory    `json:"directories,omitempty"`
	Directory     string               `json:"directory,omitempty"`
	Truncated     bool                 `json:"truncated,omitempty"`
	Platform      string               `json:"platform,omitempty"`
	CanLaunch     bool                 `json:"can_launch"`
}
