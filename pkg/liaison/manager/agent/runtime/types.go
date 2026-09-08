package runtime

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

var (
	ErrSessionNotFound    = errors.New("agent session not found")
	ErrTurnNotFound       = errors.New("agent turn not found")
	ErrStepNotFound       = errors.New("agent step not found")
	ErrSessionArchived    = errors.New("agent session is archived")
	ErrTurnAlreadyActive  = errors.New("agent turn already active")
	ErrInvalidTransition  = errors.New("invalid agent state transition")
	ErrVersionConflict    = errors.New("agent aggregate version conflict")
	ErrAttachmentNotFound = errors.New("agent attachment not found")
)

type SessionStatus uint8

const (
	SessionActive SessionStatus = iota
	SessionArchived
)

type TurnStatus uint8

const (
	TurnQueued TurnStatus = iota
	TurnRunning
	TurnWaitingApproval
	TurnCompleted
	TurnCancelled
	TurnFailed
)

func (status TurnStatus) Terminal() bool {
	return status == TurnCompleted || status == TurnCancelled || status == TurnFailed
}

type StepKind uint8

const (
	StepModel StepKind = iota
	StepTool
	StepApproval
)

type StepStatus uint8

const (
	StepPending StepStatus = iota
	StepRunning
	StepWaitingApproval
	StepCompleted
	StepCancelled
	StepFailed
)

type Session struct {
	Kind               tool.SessionKind `json:"kind"`
	ID                 string           `json:"id"`
	OrganizationID     uint             `json:"organization_id"`
	CreatedBy          uint             `json:"created_by"`
	Title              string           `json:"title"`
	Status             SessionStatus    `json:"status"`
	ActiveTurnID       string           `json:"active_turn_id,omitempty"`
	ActiveAttachmentID string           `json:"active_attachment_id,omitempty"`
	Summary            string           `json:"summary,omitempty"`
	Version            uint64           `json:"version"`
	CreatedAt          time.Time        `json:"created_at"`
	UpdatedAt          time.Time        `json:"updated_at"`
}

type Attachment struct {
	ID             string            `json:"id"`
	AgentSessionID string            `json:"agent_session_id"`
	AccessID       uint              `json:"access_id"`
	ApplicationID  uint              `json:"application_id"`
	Protocol       tool.Protocol     `json:"protocol"`
	Capabilities   []tool.Capability `json:"capabilities"`
	Generation     uint64            `json:"generation"`
	State          AttachmentState   `json:"state"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

func validateAttachment(attachment Attachment) error {
	if attachment.ID == "" || attachment.AgentSessionID == "" || attachment.AccessID == 0 || attachment.ApplicationID == 0 {
		return errors.New("attachment ID, session, access and application are required")
	}
	if attachment.Protocol == "" || len(attachment.Capabilities) == 0 {
		return errors.New("attachment protocol and capabilities are required")
	}
	if attachment.State != AttachmentConnected && attachment.State != AttachmentDisconnected && attachment.State != AttachmentRemoved {
		return fmt.Errorf("unknown attachment state %d", attachment.State)
	}
	return nil
}

type AttachmentState uint8

const (
	AttachmentConnected AttachmentState = iota
	AttachmentDisconnected
	AttachmentRemoved
)

type Turn struct {
	ID             string     `json:"id"`
	AgentSessionID string     `json:"agent_session_id"`
	Status         TurnStatus `json:"status"`
	ActiveStepID   string     `json:"active_step_id,omitempty"`
	ErrorCode      string     `json:"error_code,omitempty"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	Version        uint64     `json:"version"`
	StartedAt      time.Time  `json:"started_at"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

type Step struct {
	ID             string          `json:"id"`
	TurnID         string          `json:"turn_id"`
	Sequence       uint32          `json:"sequence"`
	Kind           StepKind        `json:"kind"`
	Status         StepStatus      `json:"status"`
	ToolID         *tool.ToolID    `json:"tool_id,omitempty"`
	ToolSnapshotID string          `json:"tool_snapshot_id,omitempty"`
	Input          json.RawMessage `json:"input,omitempty"`
	Output         json.RawMessage `json:"output,omitempty"`
	ErrorCode      string          `json:"error_code,omitempty"`
	ErrorMessage   string          `json:"error_message,omitempty"`
	Version        uint64          `json:"version"`
	StartedAt      *time.Time      `json:"started_at,omitempty"`
	CompletedAt    *time.Time      `json:"completed_at,omitempty"`
	CreatedAt      time.Time       `json:"created_at"`
	UpdatedAt      time.Time       `json:"updated_at"`
}

func validStepTransition(from, to StepStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case StepPending:
		return to == StepRunning || to == StepCancelled || to == StepFailed
	case StepRunning:
		return to == StepWaitingApproval || to == StepCompleted || to == StepCancelled || to == StepFailed
	case StepWaitingApproval:
		return to == StepRunning || to == StepCancelled || to == StepFailed
	case StepCompleted, StepCancelled, StepFailed:
		return false
	default:
		return false
	}
}

func validTurnTransition(from, to TurnStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case TurnQueued:
		return to == TurnRunning || to == TurnCancelled || to == TurnFailed
	case TurnRunning:
		return to == TurnWaitingApproval || to == TurnCompleted || to == TurnCancelled || to == TurnFailed
	case TurnWaitingApproval:
		return to == TurnRunning || to == TurnCancelled || to == TurnFailed
	case TurnCompleted, TurnCancelled, TurnFailed:
		return false
	default:
		return false
	}
}

func validateSession(session Session) error {
	if !session.Kind.Valid() {
		return errors.New("invalid session kind")
	}
	if session.Kind == tool.SessionManagement && session.ActiveAttachmentID != "" {
		return errors.New("management sessions cannot have an active access attachment")
	}
	if session.ID == "" || session.OrganizationID == 0 || session.CreatedBy == 0 {
		return errors.New("session ID, organization and creator are required")
	}
	if session.Status != SessionActive && session.Status != SessionArchived {
		return fmt.Errorf("unknown session status %d", session.Status)
	}
	return nil
}
