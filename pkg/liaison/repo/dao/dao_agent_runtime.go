package dao

import (
	"context"
	"errors"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func (d *dao) CreateAgentSession(ctx context.Context, session *model.AgentSession) error {
	if session == nil {
		return errors.New("agent session is nil")
	}
	return d.getDB().WithContext(ctx).Create(session).Error
}

func (d *dao) GetAgentSession(ctx context.Context, id string) (*model.AgentSession, error) {
	var session model.AgentSession
	if err := d.getDB().WithContext(ctx).First(&session, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &session, nil
}

func (d *dao) ListAgentSessions(ctx context.Context, createdBy uint) ([]*model.AgentSession, error) {
	var sessions []*model.AgentSession
	query := d.getDB().WithContext(ctx)
	if createdBy != 0 {
		query = query.Where("created_by = ?", createdBy)
	}
	if err := query.Order("updated_at DESC, id ASC").Find(&sessions).Error; err != nil {
		return nil, err
	}
	return sessions, nil
}

func (d *dao) UpdateAgentSessionCAS(ctx context.Context, session *model.AgentSession, expectedVersion uint64) (bool, error) {
	if session == nil {
		return false, errors.New("agent session is nil")
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentSession{}).
		Where("id = ? AND version = ?", session.ID, expectedVersion).
		Select("title", "status", "active_turn_id", "active_attachment_id", "summary", "version", "updated_at").
		Updates(session)
	return result.RowsAffected == 1, result.Error
}

func (d *dao) CreateAgentAttachment(ctx context.Context, attachment *model.AgentAttachment) error {
	if attachment == nil {
		return errors.New("agent attachment is nil")
	}
	return d.getDB().WithContext(ctx).Create(attachment).Error
}

func (d *dao) GetAgentAttachment(ctx context.Context, id string) (*model.AgentAttachment, error) {
	var attachment model.AgentAttachment
	if err := d.getDB().WithContext(ctx).First(&attachment, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &attachment, nil
}

func (d *dao) ListAgentAttachments(ctx context.Context, sessionID string) ([]*model.AgentAttachment, error) {
	var attachments []*model.AgentAttachment
	if err := d.getDB().WithContext(ctx).Where("agent_session_id = ?", sessionID).Order("created_at ASC, id ASC").Find(&attachments).Error; err != nil {
		return nil, err
	}
	return attachments, nil
}

func (d *dao) UpdateAgentAttachmentCAS(ctx context.Context, attachment *model.AgentAttachment, expectedGeneration uint64) (bool, error) {
	if attachment == nil {
		return false, errors.New("agent attachment is nil")
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentAttachment{}).
		Where("id = ? AND generation = ?", attachment.ID, expectedGeneration).
		Select("capabilities_json", "generation", "state", "updated_at").
		Updates(attachment)
	return result.RowsAffected == 1, result.Error
}

func (d *dao) CreateAgentTurn(ctx context.Context, turn *model.AgentTurn) error {
	if turn == nil {
		return errors.New("agent turn is nil")
	}
	return d.getDB().WithContext(ctx).Create(turn).Error
}

func (d *dao) GetAgentTurn(ctx context.Context, id string) (*model.AgentTurn, error) {
	var turn model.AgentTurn
	if err := d.getDB().WithContext(ctx).First(&turn, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &turn, nil
}

func (d *dao) ListAgentTurns(ctx context.Context, sessionID string) ([]*model.AgentTurn, error) {
	var turns []*model.AgentTurn
	if err := d.getDB().WithContext(ctx).Where("agent_session_id = ?", sessionID).
		Order("created_at ASC, id ASC").Find(&turns).Error; err != nil {
		return nil, err
	}
	return turns, nil
}

func (d *dao) UpdateAgentTurnCAS(ctx context.Context, turn *model.AgentTurn, expectedVersion uint64) (bool, error) {
	if turn == nil {
		return false, errors.New("agent turn is nil")
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentTurn{}).
		Where("id = ? AND version = ?", turn.ID, expectedVersion).
		Select("status", "active_step_id", "error_code", "error_message", "version", "started_at", "completed_at", "updated_at").
		Updates(turn)
	return result.RowsAffected == 1, result.Error
}

func (d *dao) CreateAgentStep(ctx context.Context, step *model.AgentStep) error {
	if step == nil {
		return errors.New("agent step is nil")
	}
	return d.getDB().WithContext(ctx).Create(step).Error
}

func (d *dao) GetAgentStep(ctx context.Context, id string) (*model.AgentStep, error) {
	var step model.AgentStep
	if err := d.getDB().WithContext(ctx).First(&step, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &step, nil
}

func (d *dao) ListAgentSessionSteps(ctx context.Context, sessionID string) ([]*model.AgentStep, error) {
	var steps []*model.AgentStep
	turnIDs := d.getDB().WithContext(ctx).Model(&model.AgentTurn{}).
		Select("id").Where("agent_session_id = ?", sessionID)
	if err := d.getDB().WithContext(ctx).Where("turn_id IN (?)", turnIDs).
		Order("created_at ASC, turn_id ASC, sequence ASC").Find(&steps).Error; err != nil {
		return nil, err
	}
	return steps, nil
}

func (d *dao) NextAgentStepSequence(ctx context.Context, turnID string) (uint32, error) {
	var maximum struct{ Sequence uint32 }
	if err := d.getDB().WithContext(ctx).Model(&model.AgentStep{}).
		Select("COALESCE(MAX(sequence), 0) AS sequence").Where("turn_id = ?", turnID).
		Scan(&maximum).Error; err != nil {
		return 0, err
	}
	return maximum.Sequence + 1, nil
}

func (d *dao) UpdateAgentStepCAS(ctx context.Context, step *model.AgentStep, expectedVersion uint64) (bool, error) {
	if step == nil {
		return false, errors.New("agent step is nil")
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentStep{}).
		Where("id = ? AND version = ?", step.ID, expectedVersion).
		Select("status", "output_json", "error_code", "error_message", "version", "started_at", "completed_at", "updated_at").
		Updates(step)
	return result.RowsAffected == 1, result.Error
}

func (d *dao) CreateAgentMessage(ctx context.Context, message *model.AgentMessage) error {
	if message == nil {
		return errors.New("agent message is nil")
	}
	return d.getDB().WithContext(ctx).Create(message).Error
}

func (d *dao) ListAgentMessages(ctx context.Context, turnID string) ([]*model.AgentMessage, error) {
	var messages []*model.AgentMessage
	if err := d.getDB().WithContext(ctx).Where("turn_id = ?", turnID).Order("sequence ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (d *dao) ListAgentSessionMessages(ctx context.Context, sessionID string) ([]*model.AgentMessage, error) {
	var messages []*model.AgentMessage
	if err := d.getDB().WithContext(ctx).Where("agent_session_id = ?", sessionID).
		Order("created_at ASC, turn_id ASC, sequence ASC").Find(&messages).Error; err != nil {
		return nil, err
	}
	return messages, nil
}

func (d *dao) NextAgentMessageSequence(ctx context.Context, turnID string) (uint32, error) {
	var maximum struct{ Sequence uint32 }
	if err := d.getDB().WithContext(ctx).Model(&model.AgentMessage{}).
		Select("COALESCE(MAX(sequence), 0) AS sequence").Where("turn_id = ?", turnID).
		Scan(&maximum).Error; err != nil {
		return 0, err
	}
	return maximum.Sequence + 1, nil
}

func (d *dao) CreateAgentApproval(ctx context.Context, approval *model.AgentApproval) error {
	if approval == nil {
		return errors.New("agent approval is nil")
	}
	return d.getDB().WithContext(ctx).Create(approval).Error
}

func (d *dao) GetAgentApproval(ctx context.Context, id string) (*model.AgentApproval, error) {
	var approval model.AgentApproval
	if err := d.getDB().WithContext(ctx).First(&approval, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &approval, nil
}

func (d *dao) ListAgentApprovals(ctx context.Context, sessionID string) ([]*model.AgentApproval, error) {
	var approvals []*model.AgentApproval
	if err := d.getDB().WithContext(ctx).Where("agent_session_id = ?", sessionID).
		Order("created_at ASC, id ASC").Find(&approvals).Error; err != nil {
		return nil, err
	}
	return approvals, nil
}

func (d *dao) UpdateAgentApprovalCAS(ctx context.Context, approval *model.AgentApproval, expectedStatus uint8) (bool, error) {
	if approval == nil {
		return false, errors.New("agent approval is nil")
	}
	result := d.getDB().WithContext(ctx).Model(&model.AgentApproval{}).
		Where("id = ? AND status = ?", approval.ID, expectedStatus).
		Select("status", "decided_by", "decision_note", "decided_at", "updated_at").
		Updates(approval)
	return result.RowsAffected == 1, result.Error
}

func (d *dao) CreateAgentToolsetSnapshot(ctx context.Context, snapshot *model.AgentToolsetSnapshot) error {
	if snapshot == nil {
		return errors.New("agent toolset snapshot is nil")
	}
	return d.getDB().WithContext(ctx).Create(snapshot).Error
}

func (d *dao) GetAgentToolsetSnapshot(ctx context.Context, id string) (*model.AgentToolsetSnapshot, error) {
	var snapshot model.AgentToolsetSnapshot
	if err := d.getDB().WithContext(ctx).First(&snapshot, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &snapshot, nil
}
