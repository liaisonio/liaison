package proto

const RPCInstallationStatus = "edge_installation_status"
const RPCUninstall = "edge_uninstall"

type UninstallCommand struct {
	RuntimeID     string `json:"runtime_id"`
	TaskID        string `json:"task_id"`
	InstanceID    string `json:"instance_id"`
	ExpiresAt     int64  `json:"expires_at"`
	CallbackURL   string `json:"callback_url"`
	CallbackToken string `json:"callback_token"`
}

// InstallationStatus mirrors api/edge_lifecycle.proto for JSON Edge RPCs.
// OwnershipVerified means read-only preflight succeeded. CanUninstall is a
// separate capability: it requires the local allow_remote_uninstall opt-in.
type InstallationStatus struct {
	RuntimeID         string `json:"runtime_id,omitempty"`
	Version           int    `json:"version"`
	Platform          string `json:"platform"`
	InstanceID        string `json:"instance_id,omitempty"`
	Service           string `json:"service,omitempty"`
	InstallationKind  string `json:"installation_kind,omitempty"`
	Legacy            bool   `json:"legacy"`
	OwnershipVerified bool   `json:"ownership_verified"`
	CanUninstall      bool   `json:"can_uninstall"`
	Reason            string `json:"reason"`
}
