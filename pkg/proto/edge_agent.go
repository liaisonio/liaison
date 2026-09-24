package proto

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const RPCEdgeAgent = "edge_agent_v1"

type EdgeAgentRequest struct {
	HistoryBefore    string             `json:"history_before,omitempty"`
	EdgeID           uint64             `json:"edge_id"`
	Action           string             `json:"action"`
	InstallationID   string             `json:"installation_id,omitempty"`
	Project          string             `json:"project,omitempty"`
	SessionID        string             `json:"session_id,omitempty"`
	Text             string             `json:"text,omitempty"`
	SkillID          string             `json:"skill_id,omitempty"`
	AccessID         string             `json:"access_id,omitempty"`
	Directory        string             `json:"directory,omitempty"`
	WorkingDirectory string             `json:"working_directory,omitempty"`
	Cursor           string             `json:"cursor,omitempty"`
	Model            string             `json:"model,omitempty"`
	Title            string             `json:"title,omitempty"`
	HistoryPage      string             `json:"history_page,omitempty"`
	ApprovalID       string             `json:"approval_id,omitempty"`
	Decision         string             `json:"decision,omitempty"`
	PermissionMode   string             `json:"permission_mode,omitempty"`
	InputID          string             `json:"input_id,omitempty"`
	Answers          []AgentInputAnswer `json:"answers,omitempty"`
}
type EdgeAgentRPCRequest struct {
	ResetRevision uint64           `json:"reset_revision,omitempty"`
	Version       int              `json:"version"`
	ActorID       string           `json:"actor_id"`
	Request       EdgeAgentRequest `json:"request"`
	ProjectRoot   string           `json:"project_root,omitempty"`
	Resume        *EdgeAgentResume `json:"resume,omitempty"`
}
type EdgeAgentResume struct {
	Truncated  bool               `json:"truncated,omitempty"`
	Window     uint64             `json:"window,omitempty"`
	SessionID  string             `json:"session_id"`
	Messages   []EdgeAgentMessage `json:"messages,omitempty"`
	Activities []AgentActivity    `json:"activities,omitempty"`
	Title      string             `json:"title,omitempty"`
	Model      string             `json:"model,omitempty"`
	StartedAt  string             `json:"started_at,omitempty"`
	Revision   uint64             `json:"revision"`
}
type AgentInstallation struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"`
	Path   string `json:"path"`
	Source string `json:"source"`
}
type EdgeAgentMessage struct {
	Role   string `json:"role"`
	Text   string `json:"text"`
	ItemID string `json:"-"`
}
type AgentDirectory struct {
	Name string `json:"name"`
	Path string `json:"path"`
}
type AgentActivity struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Status       string            `json:"status"`
	MessageIndex int               `json:"message_index"`
	DurationMS   int64             `json:"duration_ms"`
	Command      string            `json:"command,omitempty"`
	Directory    string            `json:"directory,omitempty"`
	Output       string            `json:"output,omitempty"`
	ExitCode     *int              `json:"exit_code,omitempty"`
	Changes      []AgentFileChange `json:"changes,omitempty"`
	Truncated    bool              `json:"truncated,omitempty"`
}
type AgentFileChange struct {
	Path     string `json:"path"`
	Kind     string `json:"kind"`
	Diff     string `json:"diff"`
	MovePath string `json:"move_path,omitempty"`
}
type AgentInputOption struct {
	Label       string `json:"label"`
	Description string `json:"description"`
}
type AgentInputQuestion struct {
	ID       string             `json:"id"`
	Header   string             `json:"header"`
	Question string             `json:"question"`
	IsOther  bool               `json:"is_other"`
	IsSecret bool               `json:"is_secret"`
	Options  []AgentInputOption `json:"options,omitempty"`
}
type AgentInputRequest struct {
	ID        string               `json:"id"`
	Questions []AgentInputQuestion `json:"questions"`
	Blocking  bool                 `json:"blocking"`
}
type AgentInputAnswer struct {
	QuestionID string   `json:"question_id"`
	Answers    []string `json:"answers"`
}
type EdgeAgentResult struct {
	Window               uint64                `json:"window,omitempty"`
	HistoryWindowing     bool                  `json:"history_windowing,omitempty"`
	InputRequests        []AgentInputRequest   `json:"input_requests,omitempty"`
	Approvals            []AgentApproval       `json:"approvals,omitempty"`
	PermissionsAvailable bool                  `json:"permissions_available,omitempty"`
	PermissionMode       string                `json:"permission_mode,omitempty"`
	HistoryPersistent    bool                  `json:"history_persistent,omitempty"`
	Archived             bool                  `json:"archived,omitempty"`
	HistoryTotal         int64                 `json:"history_total,omitempty"`
	Models               []AgentModel          `json:"models,omitempty"`
	ModelsAvailable      bool                  `json:"models_available,omitempty"`
	Title                string                `json:"title,omitempty"`
	SessionManagement    bool                  `json:"session_management,omitempty"`
	Sessions             []AgentSessionSummary `json:"sessions,omitempty"`
	SessionsAvailable    bool                  `json:"sessions_available,omitempty"`
	Revision             uint64                `json:"revision,omitempty"`
	Activities           []AgentActivity       `json:"activities,omitempty"`
	Directory            string                `json:"directory,omitempty"`
	ParentDirectory      string                `json:"parent_directory,omitempty"`
	Directories          []AgentDirectory      `json:"directories,omitempty"`
	DirectoryRoots       []string              `json:"directory_roots,omitempty"`
	AgentVersion         string                `json:"agent_version,omitempty"`
	ThreadID             string                `json:"thread_id,omitempty"`
	Model                string                `json:"model,omitempty"`
	Project              string                `json:"project,omitempty"`
	StartedAt            string                `json:"started_at,omitempty"`
	Skills               []AgentSkill          `json:"skills,omitempty"`
	SkillsAvailable      bool                  `json:"skills_available"`
	Version              int                   `json:"version"`
	Status               string                `json:"status"`
	Installations        []AgentInstallation   `json:"installations,omitempty"`
	DefaultProject       string                `json:"default_project,omitempty"`
	SessionID            string                `json:"session_id,omitempty"`
	Running              bool                  `json:"running"`
	Closed               bool                  `json:"closed"`
	Messages             []EdgeAgentMessage    `json:"messages,omitempty"`
	Truncated            bool                  `json:"truncated,omitempty"`
}

// AgentApproval is a display-only projection. Native RPC IDs remain on Edge.
type AgentApproval struct {
	ID        string `json:"id"`
	Command   string `json:"command"`
	Directory string `json:"directory"`
	Reason    string `json:"reason"`
}
type AgentModel struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type AgentSessionSummary struct {
	SessionID string `json:"session_id"`
	ThreadID  string `json:"thread_id"`
	Title     string `json:"title"`
	Project   string `json:"project"`
	UpdatedAt string `json:"updated_at"`
	Running   bool   `json:"running"`
	Closed    bool   `json:"closed"`
	Status    string `json:"status"`
}

type AgentSkill struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

func (r EdgeAgentRequest) Valid() bool {
	if r.Action == "transcript" {
		before, err := strconv.ParseUint(r.HistoryBefore, 10, 63)
		if err != nil || before == 0 || len(r.AccessID) != 32 {
			return false
		}
	} else if r.HistoryBefore != "" {
		return false
	}
	if r.Action == "answer" {
		if len(r.InputID) != 32 || len(r.Answers) == 0 || len(r.Answers) > 3 {
			return false
		}
		seen := map[string]bool{}
		for _, a := range r.Answers {
			if a.QuestionID == "" || len(a.QuestionID) > 128 || seen[a.QuestionID] || len(a.Answers) != 1 || !utf8.ValidString(a.Answers[0]) || len(a.Answers[0]) > 4096 || strings.TrimSpace(a.Answers[0]) == "" {
				return false
			}
			seen[a.QuestionID] = true
		}
	} else if r.InputID != "" || len(r.Answers) > 0 {
		return false
	}
	if r.Action == "approve" {
		if len(r.ApprovalID) != 32 || (r.Decision != "accept" && r.Decision != "decline") {
			return false
		}
	} else if r.ApprovalID != "" || r.Decision != "" {
		return false
	}
	if r.Action == "permissions" {
		if r.PermissionMode != "read-only" && r.PermissionMode != "workspace-write" {
			return false
		}
	} else if r.PermissionMode != "" {
		return false
	}
	if r.HistoryPage != "" {
		page, err := strconv.Atoi(r.HistoryPage)
		if r.Action != "sessions" || err != nil || page < 1 || page > 100000 {
			return false
		}
	}
	if len(r.Model) > 128 || (r.Model != "" && r.Action != "model") || (r.Title != "" && r.Action != "rename") {
		return false
	}
	if r.Action == "model" && (r.Model == "" || strings.ContainsFunc(r.Model, unicode.IsControl)) {
		return false
	}
	if r.Action == "rename" && (!utf8.ValidString(r.Title) || utf8.RuneCountInString(r.Title) > 120 || strings.TrimSpace(r.Title) == "" || strings.ContainsFunc(r.Title, unicode.IsControl)) {
		return false
	}
	if r.Action == "watch" && r.Cursor == "" {
		return false
	}
	if len(r.Directory) > 4096 || len(r.WorkingDirectory) > 4096 || (r.Directory != "" && r.Action != "directories") || (r.WorkingDirectory != "" && r.Action != "start") {
		return false
	}
	if r.Cursor != "" {
		if r.Action != "watch" {
			return false
		}
		if _, err := strconv.ParseUint(r.Cursor, 10, 64); err != nil {
			return false
		}
	}
	if (r.AccessID != "" && len(r.AccessID) != 32) || (r.SkillID != "" && (r.Action != "send" || len(r.SkillID) != 32)) {
		return false
	}
	if len(r.Project) > 4096 || len(r.InstallationID) > 64 || len(r.SessionID) > 64 || len(r.Text) > 16384 {
		return false
	}
	switch r.Action {
	case "sessions":
		return len(r.AccessID) == 32 && r.SessionID == "" && r.Text == "" && r.Project == "" && r.InstallationID == ""
	case "directories":
		return r.SessionID == "" && r.Text == "" && r.Project == "" && r.InstallationID == ""
	case "discover":
		return r.SessionID == "" && r.Text == "" && r.Project == "" && r.InstallationID == ""
	case "start":
		return len(r.InstallationID) == 32 && r.Project != "" && r.SessionID == "" && r.Text == ""
	case "resume":
		return len(r.SessionID) == 32 && len(r.AccessID) == 32 && r.Text == "" && r.Project == "" && r.InstallationID == ""
	case "poll", "snapshot", "watch", "stop", "interrupt", "models", "model", "rename", "delete", "discard", "approve", "permissions", "answer", "transcript":
		return len(r.SessionID) == 32 && r.Text == "" && r.Project == "" && r.InstallationID == ""
	case "send":
		return len(r.SessionID) == 32 && r.Text != "" && r.Project == "" && r.InstallationID == ""
	}
	return false
}
