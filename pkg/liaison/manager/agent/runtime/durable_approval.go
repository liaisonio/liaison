package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

type DurableApprovalCoordinator struct {
	dao   dao.Dao
	clock func() time.Time
}

func NewDurableApprovalCoordinator(repository dao.Dao) (*DurableApprovalCoordinator, error) {
	if repository == nil {
		return nil, errors.New("agent approval coordinator requires dao")
	}
	return &DurableApprovalCoordinator{dao: repository, clock: time.Now}, nil
}

func (coordinator *DurableApprovalCoordinator) Request(ctx context.Context, request ApprovalRequest) error {
	if request.ID == "" || request.SessionID == "" || request.TurnID == "" || request.StepID == "" || request.RequestedBy == 0 {
		return errors.New("approval ID, session, turn, step and requester are required")
	}
	invocation, err := json.Marshal(request.Invocation)
	if err != nil {
		return fmt.Errorf("encode approval invocation: %w", err)
	}
	snapshot, err := json.Marshal(request.Snapshot)
	if err != nil {
		return fmt.Errorf("encode approval tool snapshot: %w", err)
	}
	now := coordinator.clock().UTC()
	record := &model.AgentApproval{
		ID: request.ID, AgentSessionID: request.SessionID, TurnID: request.TurnID, StepID: request.StepID,
		RequestedBy: request.RequestedBy, Status: uint8(ApprovalPending), Risk: uint8(request.Risk), InputSHA256: request.InputSHA256,
		AttachmentGeneration: request.AttachmentEpoch, Reason: request.Reason, ExpiresAt: request.ExpiresAt.UTC(),
		Invocation: string(invocation), Snapshot: string(snapshot), CreatedAt: now, UpdatedAt: now,
	}
	if err := coordinator.dao.CreateAgentApproval(ctx, record); err != nil {
		return fmt.Errorf("create agent approval: %w", err)
	}
	return nil
}

func (coordinator *DurableApprovalCoordinator) Decide(ctx context.Context, approvalID string, decidedBy uint, approve bool, note string) error {
	if approvalID == "" || decidedBy == 0 {
		return errors.New("approval ID and decider are required")
	}
	record, err := coordinator.dao.GetAgentApproval(ctx, approvalID)
	if err != nil {
		return mapApprovalNotFound(err, approvalID)
	}
	if ApprovalStatus(record.Status) != ApprovalPending {
		return fmt.Errorf("%w: approval %s is %d", ErrApprovalConflict, approvalID, record.Status)
	}
	now := coordinator.clock().UTC()
	status := ApprovalDenied
	if !record.ExpiresAt.After(now) {
		status = ApprovalExpired
	} else if approve {
		status = ApprovalApproved
	}
	record.Status = uint8(status)
	record.DecidedBy = &decidedBy
	record.DecisionNote = note
	record.DecidedAt = &now
	record.UpdatedAt = now
	updated, err := coordinator.dao.UpdateAgentApprovalCAS(ctx, record, uint8(ApprovalPending))
	if err != nil {
		return fmt.Errorf("decide agent approval: %w", err)
	}
	if !updated {
		return fmt.Errorf("%w: approval %s changed concurrently", ErrApprovalConflict, approvalID)
	}
	if status == ApprovalExpired {
		return fmt.Errorf("%w: %s", ErrApprovalExpired, approvalID)
	}
	return nil
}

func (coordinator *DurableApprovalCoordinator) Consume(ctx context.Context, approvalID string, principal tool.Principal) (ApprovalRequest, error) {
	request, err := coordinator.GetApproved(ctx, approvalID, principal)
	if err != nil {
		return ApprovalRequest{}, err
	}
	record, err := coordinator.dao.GetAgentApproval(ctx, approvalID)
	if err != nil {
		return ApprovalRequest{}, mapApprovalNotFound(err, approvalID)
	}
	now := coordinator.clock().UTC()
	record.Status = uint8(ApprovalConsumed)
	record.UpdatedAt = now
	updated, err := coordinator.dao.UpdateAgentApprovalCAS(ctx, record, uint8(ApprovalApproved))
	if err != nil {
		return ApprovalRequest{}, fmt.Errorf("consume agent approval: %w", err)
	}
	if !updated {
		return ApprovalRequest{}, fmt.Errorf("%w: approval %s changed concurrently", ErrApprovalConflict, approvalID)
	}
	return request, nil
}

func (coordinator *DurableApprovalCoordinator) GetApproved(ctx context.Context, approvalID string, principal tool.Principal) (ApprovalRequest, error) {
	request, status, err := coordinator.Get(ctx, approvalID)
	if err != nil {
		return ApprovalRequest{}, err
	}
	if status != ApprovalApproved {
		return ApprovalRequest{}, fmt.Errorf("%w: approval %s is %d", ErrApprovalConflict, approvalID, status)
	}
	if !request.ExpiresAt.After(coordinator.clock().UTC()) {
		return ApprovalRequest{}, fmt.Errorf("%w: %s", ErrApprovalExpired, approvalID)
	}
	if request.RequestedBy != principal.UserID {
		return ApprovalRequest{}, errors.New("approval does not belong to principal")
	}
	if request.Invocation.Binding.Principal != principal {
		return ApprovalRequest{}, errors.New("approval principal binding changed")
	}
	return request, nil
}

func (coordinator *DurableApprovalCoordinator) Get(ctx context.Context, approvalID string) (ApprovalRequest, ApprovalStatus, error) {
	record, err := coordinator.dao.GetAgentApproval(ctx, approvalID)
	if err != nil {
		return ApprovalRequest{}, ApprovalPending, mapApprovalNotFound(err, approvalID)
	}
	var invocation tool.ToolInvocation
	if err := json.Unmarshal([]byte(record.Invocation), &invocation); err != nil {
		return ApprovalRequest{}, ApprovalStatus(record.Status), fmt.Errorf("decode approval invocation: %w", err)
	}
	var snapshot tool.ToolSetSnapshot
	if err := json.Unmarshal([]byte(record.Snapshot), &snapshot); err != nil {
		return ApprovalRequest{}, ApprovalStatus(record.Status), fmt.Errorf("decode approval tool snapshot: %w", err)
	}
	return ApprovalRequest{ID: record.ID, SessionID: record.AgentSessionID, TurnID: record.TurnID, StepID: record.StepID,
		RequestedBy: record.RequestedBy, Invocation: invocation, Snapshot: snapshot, InputSHA256: record.InputSHA256,
		Reason: record.Reason, Risk: tool.RiskLevel(record.Risk), ExpiresAt: record.ExpiresAt,
		AttachmentEpoch: record.AttachmentGeneration}, ApprovalStatus(record.Status), nil
}

func (coordinator *DurableApprovalCoordinator) List(ctx context.Context, sessionID string) ([]ApprovalView, error) {
	if sessionID == "" {
		return nil, errors.New("agent session ID is required")
	}
	records, err := coordinator.dao.ListAgentApprovals(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list agent approvals: %w", err)
	}
	views := make([]ApprovalView, 0, len(records))
	for _, record := range records {
		var invocation tool.ToolInvocation
		if err := json.Unmarshal([]byte(record.Invocation), &invocation); err != nil {
			return nil, fmt.Errorf("decode approval invocation %s: %w", record.ID, err)
		}
		views = append(views, ApprovalView{
			ID: record.ID, TurnID: record.TurnID, StepID: record.StepID, Status: ApprovalStatus(record.Status),
			ToolID: invocation.Call.ID, Input: append(json.RawMessage(nil), invocation.Call.Input...),
			InputSHA256: record.InputSHA256, Reason: record.Reason, Risk: tool.RiskLevel(record.Risk),
			ExpiresAt: record.ExpiresAt, DecisionNote: record.DecisionNote, DecidedAt: record.DecidedAt,
		})
	}
	return views, nil
}

func mapApprovalNotFound(err error, approvalID string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %s", ErrApprovalNotFound, approvalID)
	}
	return err
}
