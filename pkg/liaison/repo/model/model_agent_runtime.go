package model

import (
	"time"

	"gorm.io/gorm"
)

type AgentSession struct {
	Kind               string         `gorm:"column:kind;type:varchar(32);not null;default:access" json:"kind"`
	ID                 string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt          time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	OrganizationID     uint           `gorm:"column:organization_id;not null;index" json:"organization_id"`
	CreatedBy          uint           `gorm:"column:created_by;not null;index" json:"created_by"`
	Title              string         `gorm:"column:title;type:varchar(255);not null;default:''" json:"title"`
	Status             uint8          `gorm:"column:status;type:tinyint;not null;default:0;index" json:"status"`
	ActiveTurnID       string         `gorm:"column:active_turn_id;type:varchar(64);not null;default:'';index" json:"active_turn_id"`
	ActiveAttachmentID string         `gorm:"column:active_attachment_id;type:varchar(64);not null;default:''" json:"active_attachment_id"`
	Summary            string         `gorm:"column:summary;type:text;not null;default:''" json:"summary"`
	Version            uint64         `gorm:"column:version;not null;default:1" json:"version"`
}

func (AgentSession) TableName() string { return "agent_sessions" }

type AgentAttachment struct {
	AccessHandleID string         `gorm:"column:access_handle_id;type:varchar(64);not null;default:''" json:"-"`
	ID             string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	AgentSessionID string         `gorm:"column:agent_session_id;type:varchar(64);not null;index" json:"agent_session_id"`
	AccessID       uint           `gorm:"column:access_id;not null;index" json:"access_id"`
	ApplicationID  uint           `gorm:"column:application_id;not null;index" json:"application_id"`
	Protocol       string         `gorm:"column:protocol;type:varchar(32);not null;default:'';index" json:"protocol"`
	Capabilities   string         `gorm:"column:capabilities_json;type:text;not null;default:'[]'" json:"capabilities_json"`
	Generation     uint64         `gorm:"column:generation;not null;default:1" json:"generation"`
	State          uint8          `gorm:"column:state;type:tinyint;not null;default:0;index" json:"state"`
}

func (AgentAttachment) TableName() string { return "agent_attachments" }

type AgentTurn struct {
	ID             string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	AgentSessionID string         `gorm:"column:agent_session_id;type:varchar(64);not null;index" json:"agent_session_id"`
	Status         uint8          `gorm:"column:status;type:tinyint;not null;default:0;index" json:"status"`
	ActiveStepID   string         `gorm:"column:active_step_id;type:varchar(64);not null;default:''" json:"active_step_id"`
	ErrorCode      string         `gorm:"column:error_code;type:varchar(64);not null;default:''" json:"error_code"`
	ErrorMessage   string         `gorm:"column:error_message;type:varchar(1024);not null;default:''" json:"error_message"`
	Version        uint64         `gorm:"column:version;not null;default:1" json:"version"`
	StartedAt      *time.Time     `gorm:"column:started_at" json:"started_at,omitempty"`
	CompletedAt    *time.Time     `gorm:"column:completed_at;index" json:"completed_at,omitempty"`
}

func (AgentTurn) TableName() string { return "agent_turns" }

type AgentMessage struct {
	ID             string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime;index" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	AgentSessionID string         `gorm:"column:agent_session_id;type:varchar(64);not null;index" json:"agent_session_id"`
	TurnID         string         `gorm:"column:turn_id;type:varchar(64);not null;uniqueIndex:idx_agent_message_order,priority:1" json:"turn_id"`
	Sequence       uint32         `gorm:"column:sequence;not null;uniqueIndex:idx_agent_message_order,priority:2" json:"sequence"`
	Role           string         `gorm:"column:role;type:varchar(16);not null;default:''" json:"role"`
	Content        string         `gorm:"column:content_json;type:text;not null;default:'{}'" json:"content_json"`
	ModelProvider  string         `gorm:"column:model_provider;type:varchar(64);not null;default:''" json:"model_provider"`
	ModelName      string         `gorm:"column:model_name;type:varchar(128);not null;default:''" json:"model_name"`
	Usage          string         `gorm:"column:usage_json;type:text;not null;default:'{}'" json:"usage_json"`
}

func (AgentMessage) TableName() string { return "agent_messages" }

type AgentStep struct {
	ID             string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt      time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	TurnID         string         `gorm:"column:turn_id;type:varchar(64);not null;uniqueIndex:idx_agent_step_sequence,priority:1" json:"turn_id"`
	Sequence       uint32         `gorm:"column:sequence;not null;uniqueIndex:idx_agent_step_sequence,priority:2" json:"sequence"`
	Kind           uint8          `gorm:"column:kind;type:tinyint;not null" json:"kind"`
	Status         uint8          `gorm:"column:status;type:tinyint;not null;default:0;index" json:"status"`
	ToolID         string         `gorm:"column:tool_id;type:varchar(192);not null;default:''" json:"tool_id"`
	ToolSnapshotID string         `gorm:"column:tool_snapshot_id;type:varchar(64);not null;default:'';index" json:"tool_snapshot_id"`
	Input          string         `gorm:"column:input_json;type:text;not null;default:'{}'" json:"input_json"`
	Output         string         `gorm:"column:output_json;type:text;not null;default:'{}'" json:"output_json"`
	ErrorCode      string         `gorm:"column:error_code;type:varchar(64);not null;default:''" json:"error_code"`
	ErrorMessage   string         `gorm:"column:error_message;type:varchar(1024);not null;default:''" json:"error_message"`
	Version        uint64         `gorm:"column:version;not null;default:1" json:"version"`
	StartedAt      *time.Time     `gorm:"column:started_at" json:"started_at,omitempty"`
	CompletedAt    *time.Time     `gorm:"column:completed_at" json:"completed_at,omitempty"`
}

func (AgentStep) TableName() string { return "agent_steps" }

type AgentApproval struct {
	ID                   string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt            time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt            time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	AgentSessionID       string         `gorm:"column:agent_session_id;type:varchar(64);not null;index" json:"agent_session_id"`
	TurnID               string         `gorm:"column:turn_id;type:varchar(64);not null;index" json:"turn_id"`
	StepID               string         `gorm:"column:step_id;type:varchar(64);not null;uniqueIndex" json:"step_id"`
	RequestedBy          uint           `gorm:"column:requested_by;not null;index" json:"requested_by"`
	DecidedBy            *uint          `gorm:"column:decided_by;index" json:"decided_by,omitempty"`
	Status               uint8          `gorm:"column:status;type:tinyint;not null;default:0;index" json:"status"`
	Risk                 uint8          `gorm:"column:risk;type:tinyint;not null;default:0" json:"risk"`
	InputSHA256          string         `gorm:"column:input_sha256;type:char(64);not null;default:''" json:"input_sha256"`
	AttachmentGeneration uint64         `gorm:"column:attachment_generation;not null;default:0" json:"attachment_generation"`
	Reason               string         `gorm:"column:reason;type:varchar(1024);not null;default:''" json:"reason"`
	DecisionNote         string         `gorm:"column:decision_note;type:varchar(1024);not null;default:''" json:"decision_note"`
	ExpiresAt            time.Time      `gorm:"column:expires_at;not null;index" json:"expires_at"`
	DecidedAt            *time.Time     `gorm:"column:decided_at" json:"decided_at,omitempty"`
	Invocation           string         `gorm:"column:invocation_json;type:text;not null;default:'{}'" json:"invocation_json"`
	Snapshot             string         `gorm:"column:snapshot_json;type:text;not null;default:'{}'" json:"snapshot_json"`
}

func (AgentApproval) TableName() string { return "agent_approvals" }

type AgentToolsetSnapshot struct {
	ID                string         `gorm:"column:id;type:varchar(64);primaryKey" json:"id"`
	CreatedAt         time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time      `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	DeletedAt         gorm.DeletedAt `gorm:"column:deleted_at;index" json:"-"`
	TurnID            string         `gorm:"column:turn_id;type:varchar(64);not null;index" json:"turn_id"`
	ModelStepID       string         `gorm:"column:model_step_id;type:varchar(64);not null;uniqueIndex" json:"model_step_id"`
	CatalogGeneration uint64         `gorm:"column:catalog_generation;not null" json:"catalog_generation"`
	PolicyRevision    string         `gorm:"column:policy_revision;type:varchar(128);not null;default:''" json:"policy_revision"`
	Attachments       string         `gorm:"column:attachments_json;type:text;not null;default:'{}'" json:"attachments_json"`
	Tools             string         `gorm:"column:tools_json;type:text;not null;default:'[]'" json:"tools_json"`
}

func (AgentToolsetSnapshot) TableName() string { return "agent_toolset_snapshots" }
