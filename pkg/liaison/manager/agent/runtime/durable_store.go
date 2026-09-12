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

// DurableStore persists the runtime state machine with optimistic concurrency.
// Multi-row state changes are committed in one database transaction.
type DurableStore struct {
	dao   dao.Dao
	clock func() time.Time
}

func NewDurableStore(repository dao.Dao) (*DurableStore, error) {
	if repository == nil {
		return nil, errors.New("agent durable store requires dao")
	}
	return &DurableStore{dao: repository, clock: time.Now}, nil
}

func (store *DurableStore) CreateSession(ctx context.Context, session Session) error {
	if err := validateSession(session); err != nil {
		return err
	}
	now := store.clock().UTC()
	session.Version = 1
	session.CreatedAt = now
	session.UpdatedAt = now
	if err := store.dao.CreateAgentSession(ctx, sessionModel(session)); err != nil {
		return fmt.Errorf("create agent session: %w", err)
	}
	return nil
}

func (store *DurableStore) CreateSessionWithAttachment(ctx context.Context, session Session, attachment Attachment) (Session, Attachment, error) {
	if session.Kind == tool.SessionManagement {
		return Session{}, Attachment{}, errors.New("management sessions cannot have access attachments")
	}
	if err := validateSession(session); err != nil {
		return Session{}, Attachment{}, err
	}
	if attachment.AgentSessionID != session.ID {
		return Session{}, Attachment{}, errors.New("attachment session does not match session")
	}
	if err := validateAttachment(attachment); err != nil {
		return Session{}, Attachment{}, err
	}
	now := store.clock().UTC()
	session.Version = 1
	session.CreatedAt = now
	session.UpdatedAt = now
	attachment.CreatedAt = now
	attachment.UpdatedAt = now
	if attachment.Generation == 0 {
		attachment.Generation = 1
	}
	session.ActiveAttachmentID = attachment.ID
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		if err := repository.CreateAgentSession(ctx, sessionModel(session)); err != nil {
			return fmt.Errorf("create agent session: %w", err)
		}
		record, err := attachmentModel(attachment)
		if err != nil {
			return err
		}
		if err := repository.CreateAgentAttachment(ctx, record); err != nil {
			return fmt.Errorf("create agent attachment: %w", err)
		}
		return nil
	})
	if err != nil {
		return Session{}, Attachment{}, err
	}
	return session, cloneAttachment(attachment), nil
}

func (store *DurableStore) GetSession(ctx context.Context, sessionID string) (Session, error) {
	record, err := store.dao.GetAgentSession(ctx, sessionID)
	if err != nil {
		return Session{}, mapNotFound(err, ErrSessionNotFound, sessionID)
	}
	return sessionFromModel(record), nil
}

func (store *DurableStore) ListSessions(ctx context.Context, createdBy uint) ([]Session, error) {
	records, err := store.dao.ListAgentSessions(ctx, createdBy)
	if err != nil {
		return nil, fmt.Errorf("list agent sessions: %w", err)
	}
	sessions := make([]Session, 0, len(records))
	for _, record := range records {
		sessions = append(sessions, sessionFromModel(record))
	}
	return sessions, nil
}

func (store *DurableStore) ArchiveSession(ctx context.Context, sessionID string, expectedVersion uint64) (Session, error) {
	session, err := store.GetSession(ctx, sessionID)
	if err != nil {
		return Session{}, err
	}
	if session.Version != expectedVersion {
		return Session{}, versionConflict("session", sessionID)
	}
	if session.ActiveTurnID != "" {
		return Session{}, fmt.Errorf("%w: session %s has active turn %s", ErrTurnAlreadyActive, sessionID, session.ActiveTurnID)
	}
	session.Status = SessionArchived
	session.Version++
	session.UpdatedAt = store.clock().UTC()
	updated, err := store.dao.UpdateAgentSessionCAS(ctx, sessionModel(session), expectedVersion)
	if err != nil {
		return Session{}, fmt.Errorf("archive agent session: %w", err)
	}
	if !updated {
		return Session{}, versionConflict("session", sessionID)
	}
	return session, nil
}

func (store *DurableStore) CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error) {
	if err := validateAttachment(attachment); err != nil {
		return Attachment{}, err
	}
	var created Attachment
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		sessionRecord, getErr := repository.GetAgentSession(ctx, attachment.AgentSessionID)
		if getErr != nil {
			return mapNotFound(getErr, ErrSessionNotFound, attachment.AgentSessionID)
		}
		if tool.SessionKind(sessionRecord.Kind) == tool.SessionManagement {
			return errors.New("management sessions cannot have access attachments")
		}
		if SessionStatus(sessionRecord.Status) == SessionArchived {
			return fmt.Errorf("%w: %s", ErrSessionArchived, attachment.AgentSessionID)
		}
		now := store.clock().UTC()
		if attachment.Generation == 0 {
			attachment.Generation = 1
		}
		attachment.CreatedAt = now
		attachment.UpdatedAt = now
		record, encodeErr := attachmentModel(attachment)
		if encodeErr != nil {
			return encodeErr
		}
		if createErr := repository.CreateAgentAttachment(ctx, record); createErr != nil {
			return fmt.Errorf("create agent attachment: %w", createErr)
		}
		created = cloneAttachment(attachment)
		return nil
	})
	if err != nil {
		return Attachment{}, err
	}
	return created, nil
}

func (store *DurableStore) GetAttachment(ctx context.Context, attachmentID string) (Attachment, error) {
	record, err := store.dao.GetAgentAttachment(ctx, attachmentID)
	if err != nil {
		return Attachment{}, mapNotFound(err, ErrAttachmentNotFound, attachmentID)
	}
	return attachmentFromModel(record)
}

func (store *DurableStore) ListAttachments(ctx context.Context, sessionID string) ([]Attachment, error) {
	if _, err := store.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}
	records, err := store.dao.ListAgentAttachments(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list agent attachments: %w", err)
	}
	attachments := make([]Attachment, 0, len(records))
	for _, record := range records {
		attachment, decodeErr := attachmentFromModel(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		attachments = append(attachments, attachment)
	}
	return attachments, nil
}

func (store *DurableStore) UpdateAttachment(ctx context.Context, attachment Attachment, expectedGeneration uint64) (Attachment, error) {
	if err := validateAttachment(attachment); err != nil {
		return Attachment{}, err
	}
	current, err := store.GetAttachment(ctx, attachment.ID)
	if err != nil {
		return Attachment{}, err
	}
	if current.Generation != expectedGeneration {
		return Attachment{}, versionConflict("attachment", attachment.ID)
	}
	if current.AgentSessionID != attachment.AgentSessionID || current.AccessID != attachment.AccessID ||
		current.ApplicationID != attachment.ApplicationID || current.Protocol != attachment.Protocol {
		return Attachment{}, errors.New("attachment resource binding is immutable")
	}
	if current.State == AttachmentRemoved {
		return Attachment{}, fmt.Errorf("%w: removed attachment %s", ErrInvalidTransition, attachment.ID)
	}
	attachment.Generation = expectedGeneration + 1
	attachment.CreatedAt = current.CreatedAt
	attachment.UpdatedAt = store.clock().UTC()
	record, err := attachmentModel(attachment)
	if err != nil {
		return Attachment{}, err
	}
	updated, err := store.dao.UpdateAgentAttachmentCAS(ctx, record, expectedGeneration)
	if err != nil {
		return Attachment{}, fmt.Errorf("update agent attachment: %w", err)
	}
	if !updated {
		return Attachment{}, versionConflict("attachment", attachment.ID)
	}
	return cloneAttachment(attachment), nil
}

func (store *DurableStore) SetActiveAttachment(ctx context.Context, sessionID, attachmentID string, expectedVersion uint64) (Session, error) {
	var updatedSession Session
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		sessionRecord, getErr := repository.GetAgentSession(ctx, sessionID)
		if getErr != nil {
			return mapNotFound(getErr, ErrSessionNotFound, sessionID)
		}
		session := sessionFromModel(sessionRecord)
		if session.Version != expectedVersion {
			return versionConflict("session", sessionID)
		}
		if session.Kind == tool.SessionManagement && attachmentID != "" {
			return errors.New("management sessions cannot activate access attachments")
		}
		if session.Status == SessionArchived {
			return fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
		}
		if attachmentID != "" {
			attachmentRecord, attachmentErr := repository.GetAgentAttachment(ctx, attachmentID)
			if attachmentErr != nil {
				return mapNotFound(attachmentErr, ErrAttachmentNotFound, attachmentID)
			}
			if attachmentRecord.AgentSessionID != sessionID || AttachmentState(attachmentRecord.State) == AttachmentRemoved {
				return fmt.Errorf("%w: %s", ErrAttachmentNotFound, attachmentID)
			}
		}
		session.ActiveAttachmentID = attachmentID
		session.Version++
		session.UpdatedAt = store.clock().UTC()
		updated, updateErr := repository.UpdateAgentSessionCAS(ctx, sessionModel(session), expectedVersion)
		if updateErr != nil {
			return fmt.Errorf("set active agent attachment: %w", updateErr)
		}
		if !updated {
			return versionConflict("session", sessionID)
		}
		updatedSession = session
		return nil
	})
	if err != nil {
		return Session{}, err
	}
	return updatedSession, nil
}

func (store *DurableStore) StartTurn(ctx context.Context, sessionID string, turn Turn) (Session, Turn, error) {
	if turn.ID == "" {
		return Session{}, Turn{}, errors.New("turn ID is required")
	}
	var updatedSession Session
	var createdTurn Turn
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		record, getErr := repository.GetAgentSession(ctx, sessionID)
		if getErr != nil {
			return mapNotFound(getErr, ErrSessionNotFound, sessionID)
		}
		session := sessionFromModel(record)
		if session.Status == SessionArchived {
			return fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
		}
		if session.ActiveTurnID != "" {
			return fmt.Errorf("%w: %s", ErrTurnAlreadyActive, session.ActiveTurnID)
		}
		now := store.clock().UTC()
		turn.AgentSessionID = sessionID
		turn.Status = TurnQueued
		turn.Version = 1
		turn.CreatedAt = now
		turn.UpdatedAt = now
		if createErr := repository.CreateAgentTurn(ctx, turnModel(turn)); createErr != nil {
			return fmt.Errorf("create agent turn: %w", createErr)
		}
		expectedVersion := session.Version
		session.ActiveTurnID = turn.ID
		session.Version++
		session.UpdatedAt = now
		updated, updateErr := repository.UpdateAgentSessionCAS(ctx, sessionModel(session), expectedVersion)
		if updateErr != nil {
			return fmt.Errorf("activate agent turn: %w", updateErr)
		}
		if !updated {
			return versionConflict("session", sessionID)
		}
		updatedSession = session
		createdTurn = turn
		return nil
	})
	if err != nil {
		return Session{}, Turn{}, err
	}
	return updatedSession, createdTurn, nil
}

func (store *DurableStore) GetTurn(ctx context.Context, turnID string) (Turn, error) {
	record, err := store.dao.GetAgentTurn(ctx, turnID)
	if err != nil {
		return Turn{}, mapNotFound(err, ErrTurnNotFound, turnID)
	}
	return turnFromModel(record), nil
}

func (store *DurableStore) ListTurns(ctx context.Context, sessionID string) ([]Turn, error) {
	if _, err := store.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}
	records, err := store.dao.ListAgentTurns(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list agent turns: %w", err)
	}
	turns := make([]Turn, 0, len(records))
	for _, record := range records {
		turns = append(turns, turnFromModel(record))
	}
	return turns, nil
}

func (store *DurableStore) TransitionTurn(ctx context.Context, turnID string, expectedVersion uint64, to TurnStatus, code, message string) (Turn, error) {
	var updatedTurn Turn
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		record, getErr := repository.GetAgentTurn(ctx, turnID)
		if getErr != nil {
			return mapNotFound(getErr, ErrTurnNotFound, turnID)
		}
		turn := turnFromModel(record)
		if turn.Version != expectedVersion {
			return versionConflict("turn", turnID)
		}
		if !validTurnTransition(turn.Status, to) {
			return fmt.Errorf("%w: turn %s from %d to %d", ErrInvalidTransition, turnID, turn.Status, to)
		}
		now := store.clock().UTC()
		turn.Status = to
		turn.ErrorCode = code
		turn.ErrorMessage = message
		turn.Version++
		turn.UpdatedAt = now
		if to == TurnRunning && turn.StartedAt.IsZero() {
			turn.StartedAt = now
		}
		if to.Terminal() {
			turn.CompletedAt = &now
		}
		updated, updateErr := repository.UpdateAgentTurnCAS(ctx, turnModel(turn), expectedVersion)
		if updateErr != nil {
			return fmt.Errorf("transition agent turn: %w", updateErr)
		}
		if !updated {
			return versionConflict("turn", turnID)
		}
		if to.Terminal() {
			sessionRecord, sessionErr := repository.GetAgentSession(ctx, turn.AgentSessionID)
			if sessionErr != nil {
				return mapNotFound(sessionErr, ErrSessionNotFound, turn.AgentSessionID)
			}
			session := sessionFromModel(sessionRecord)
			if session.ActiveTurnID == turn.ID {
				expectedSessionVersion := session.Version
				session.ActiveTurnID = ""
				session.Version++
				session.UpdatedAt = now
				sessionUpdated, sessionUpdateErr := repository.UpdateAgentSessionCAS(ctx, sessionModel(session), expectedSessionVersion)
				if sessionUpdateErr != nil {
					return fmt.Errorf("release agent session turn: %w", sessionUpdateErr)
				}
				if !sessionUpdated {
					return versionConflict("session", session.ID)
				}
			}
		}
		updatedTurn = turn
		return nil
	})
	if err != nil {
		return Turn{}, err
	}
	return updatedTurn, nil
}

func (store *DurableStore) AppendStep(ctx context.Context, turnID string, expectedTurnVersion uint64, step Step) (Turn, Step, error) {
	if step.ID == "" {
		return Turn{}, Step{}, errors.New("step ID is required")
	}
	var updatedTurn Turn
	var createdStep Step
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		record, getErr := repository.GetAgentTurn(ctx, turnID)
		if getErr != nil {
			return mapNotFound(getErr, ErrTurnNotFound, turnID)
		}
		turn := turnFromModel(record)
		if turn.Version != expectedTurnVersion {
			return versionConflict("turn", turnID)
		}
		if turn.Status.Terminal() {
			return fmt.Errorf("%w: cannot append to terminal turn %s", ErrInvalidTransition, turnID)
		}
		sequence, sequenceErr := repository.NextAgentStepSequence(ctx, turnID)
		if sequenceErr != nil {
			return fmt.Errorf("allocate agent step sequence: %w", sequenceErr)
		}
		now := store.clock().UTC()
		step.TurnID = turnID
		step.Sequence = sequence
		step.Version = 1
		step.CreatedAt = now
		step.UpdatedAt = now
		if step.Status == StepRunning {
			step.StartedAt = &now
		}
		expectedVersion := turn.Version
		turn.ActiveStepID = step.ID
		turn.Version++
		turn.UpdatedAt = now
		updated, updateErr := repository.UpdateAgentTurnCAS(ctx, turnModel(turn), expectedVersion)
		if updateErr != nil {
			return fmt.Errorf("activate agent step: %w", updateErr)
		}
		if !updated {
			return versionConflict("turn", turnID)
		}
		if createErr := repository.CreateAgentStep(ctx, stepModel(step)); createErr != nil {
			return fmt.Errorf("create agent step: %w", createErr)
		}
		updatedTurn = turn
		createdStep = step
		return nil
	})
	if err != nil {
		return Turn{}, Step{}, err
	}
	return updatedTurn, cloneStep(createdStep), nil
}

func (store *DurableStore) GetStep(ctx context.Context, stepID string) (Step, error) {
	record, err := store.dao.GetAgentStep(ctx, stepID)
	if err != nil {
		return Step{}, mapNotFound(err, ErrStepNotFound, stepID)
	}
	return stepFromModel(record)
}

func (store *DurableStore) ListSessionSteps(ctx context.Context, sessionID string) ([]Step, error) {
	if _, err := store.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}
	records, err := store.dao.ListAgentSessionSteps(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list agent session steps: %w", err)
	}
	steps := make([]Step, 0, len(records))
	for _, record := range records {
		step, decodeErr := stepFromModel(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		steps = append(steps, step)
	}
	return steps, nil
}

func (store *DurableStore) CompleteStep(ctx context.Context, stepID string, output json.RawMessage) (Step, error) {
	return store.transitionStep(ctx, stepID, StepCompleted, output, "", "")
}

func (store *DurableStore) WaitStepApproval(ctx context.Context, stepID string) (Step, error) {
	return store.transitionStep(ctx, stepID, StepWaitingApproval, nil, "", "")
}

func (store *DurableStore) ResumeStep(ctx context.Context, stepID string) (Step, error) {
	return store.transitionStep(ctx, stepID, StepRunning, nil, "", "")
}

func (store *DurableStore) CancelStep(ctx context.Context, stepID, code, message string) (Step, error) {
	return store.transitionStep(ctx, stepID, StepCancelled, nil, code, message)
}

func (store *DurableStore) FailStep(ctx context.Context, stepID, code, message string) (Step, error) {
	return store.transitionStep(ctx, stepID, StepFailed, nil, code, message)
}

func (store *DurableStore) transitionStep(ctx context.Context, stepID string, to StepStatus, output json.RawMessage, code, message string) (Step, error) {
	record, err := store.dao.GetAgentStep(ctx, stepID)
	if err != nil {
		return Step{}, mapNotFound(err, ErrStepNotFound, stepID)
	}
	step, err := stepFromModel(record)
	if err != nil {
		return Step{}, err
	}
	if !validStepTransition(step.Status, to) {
		return Step{}, fmt.Errorf("%w: step %s from %d to %d", ErrInvalidTransition, stepID, step.Status, to)
	}
	expectedVersion := step.Version
	now := store.clock().UTC()
	step.Status = to
	step.Version++
	step.UpdatedAt = now
	step.ErrorCode = code
	step.ErrorMessage = message
	if len(output) > 0 {
		step.Output = append(json.RawMessage(nil), output...)
	}
	if to == StepCompleted || to == StepCancelled || to == StepFailed {
		step.CompletedAt = &now
	}
	updated, updateErr := store.dao.UpdateAgentStepCAS(ctx, stepModel(step), expectedVersion)
	if updateErr != nil {
		return Step{}, fmt.Errorf("transition agent step: %w", updateErr)
	}
	if !updated {
		return Step{}, versionConflict("step", stepID)
	}
	return cloneStep(step), nil
}

func (store *DurableStore) AppendMessage(ctx context.Context, message Message) (Message, error) {
	if message.ID == "" || message.AgentSessionID == "" || message.TurnID == "" {
		return Message{}, errors.New("message ID, session and turn are required")
	}
	var created Message
	err := dao.WithTransactionContext(ctx, store.dao, func(repository dao.Dao) error {
		sequence, sequenceErr := repository.NextAgentMessageSequence(ctx, message.TurnID)
		if sequenceErr != nil {
			return fmt.Errorf("allocate agent message sequence: %w", sequenceErr)
		}
		encoded, encodeErr := json.Marshal(message.Value)
		if encodeErr != nil {
			return fmt.Errorf("encode agent message: %w", encodeErr)
		}
		now := store.clock().UTC()
		message.Sequence = sequence
		message.CreatedAt = now
		record := &model.AgentMessage{ID: message.ID, AgentSessionID: message.AgentSessionID, TurnID: message.TurnID,
			Sequence: sequence, Role: string(message.Value.Role), Content: string(encoded), CreatedAt: now, UpdatedAt: now}
		if createErr := repository.CreateAgentMessage(ctx, record); createErr != nil {
			return fmt.Errorf("create agent message: %w", createErr)
		}
		created = message
		return nil
	})
	if err != nil {
		return Message{}, err
	}
	return created, nil
}

func (store *DurableStore) ListMessages(ctx context.Context, turnID string) ([]Message, error) {
	records, err := store.dao.ListAgentMessages(ctx, turnID)
	if err != nil {
		return nil, fmt.Errorf("list agent messages: %w", err)
	}
	messages := make([]Message, 0, len(records))
	for _, record := range records {
		message, decodeErr := messageFromModel(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (store *DurableStore) ListSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	if _, err := store.GetSession(ctx, sessionID); err != nil {
		return nil, err
	}
	records, err := store.dao.ListAgentSessionMessages(ctx, sessionID)
	if err != nil {
		return nil, fmt.Errorf("list agent session messages: %w", err)
	}
	messages := make([]Message, 0, len(records))
	for _, record := range records {
		message, decodeErr := messageFromModel(record)
		if decodeErr != nil {
			return nil, decodeErr
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (store *DurableStore) SaveToolSnapshot(ctx context.Context, turnID, modelStepID string, snapshot tool.ToolSetSnapshot) error {
	if turnID == "" || modelStepID == "" || snapshot.ID == "" {
		return errors.New("turn, model step and snapshot IDs are required")
	}
	attachments, err := json.Marshal(snapshot.AttachmentGeneration)
	if err != nil {
		return fmt.Errorf("encode tool snapshot attachments: %w", err)
	}
	tools, err := json.Marshal(snapshot.Tools)
	if err != nil {
		return fmt.Errorf("encode tool snapshot tools: %w", err)
	}
	createdAt := snapshot.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = store.clock().UTC()
	}
	record := &model.AgentToolsetSnapshot{ID: snapshot.ID, TurnID: turnID, ModelStepID: modelStepID,
		CatalogGeneration: snapshot.CatalogGeneration, PolicyRevision: snapshot.PolicyRevision,
		Attachments: string(attachments), Tools: string(tools), CreatedAt: createdAt, UpdatedAt: createdAt}
	if err := store.dao.CreateAgentToolsetSnapshot(ctx, record); err != nil {
		return fmt.Errorf("create agent tool snapshot: %w", err)
	}
	return nil
}

func (store *DurableStore) GetToolSnapshot(ctx context.Context, snapshotID string) (tool.ToolSetSnapshot, error) {
	record, err := store.dao.GetAgentToolsetSnapshot(ctx, snapshotID)
	if err != nil {
		return tool.ToolSetSnapshot{}, fmt.Errorf("get agent tool snapshot: %w", err)
	}
	var attachments map[string]uint64
	if err := json.Unmarshal([]byte(record.Attachments), &attachments); err != nil {
		return tool.ToolSetSnapshot{}, fmt.Errorf("decode tool snapshot attachments: %w", err)
	}
	var exposed []tool.ExposedTool
	if err := json.Unmarshal([]byte(record.Tools), &exposed); err != nil {
		return tool.ToolSetSnapshot{}, fmt.Errorf("decode tool snapshot tools: %w", err)
	}
	return tool.ToolSetSnapshot{ID: record.ID, CatalogGeneration: record.CatalogGeneration,
		PolicyRevision: record.PolicyRevision, AttachmentGeneration: attachments, Tools: exposed, CreatedAt: record.CreatedAt}, nil
}

func sessionModel(value Session) *model.AgentSession {
	return &model.AgentSession{Kind: string(value.Kind), ID: value.ID, OrganizationID: value.OrganizationID, CreatedBy: value.CreatedBy, Title: value.Title,
		Status: uint8(value.Status), ActiveTurnID: value.ActiveTurnID, ActiveAttachmentID: value.ActiveAttachmentID,
		Summary: value.Summary, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func sessionFromModel(value *model.AgentSession) Session {
	return Session{Kind: tool.SessionKind(value.Kind), ID: value.ID, OrganizationID: value.OrganizationID, CreatedBy: value.CreatedBy, Title: value.Title,
		Status: SessionStatus(value.Status), ActiveTurnID: value.ActiveTurnID, ActiveAttachmentID: value.ActiveAttachmentID,
		Summary: value.Summary, Version: value.Version, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func attachmentModel(value Attachment) (*model.AgentAttachment, error) {
	capabilities, err := json.Marshal(value.Capabilities)
	if err != nil {
		return nil, fmt.Errorf("encode agent attachment capabilities: %w", err)
	}
	return &model.AgentAttachment{ID: value.ID, AccessHandleID: value.AccessHandleID, AgentSessionID: value.AgentSessionID, AccessID: value.AccessID,
		ApplicationID: value.ApplicationID, Protocol: string(value.Protocol), Capabilities: string(capabilities),
		Generation: value.Generation, State: uint8(value.State), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}, nil
}

func attachmentFromModel(value *model.AgentAttachment) (Attachment, error) {
	var capabilities []tool.Capability
	if err := json.Unmarshal([]byte(value.Capabilities), &capabilities); err != nil {
		return Attachment{}, fmt.Errorf("decode agent attachment %s capabilities: %w", value.ID, err)
	}
	attachment := Attachment{ID: value.ID, AccessHandleID: value.AccessHandleID, AgentSessionID: value.AgentSessionID, AccessID: value.AccessID,
		ApplicationID: value.ApplicationID, Protocol: tool.Protocol(value.Protocol), Capabilities: capabilities,
		Generation: value.Generation, State: AttachmentState(value.State), CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
	if err := validateAttachment(attachment); err != nil {
		return Attachment{}, fmt.Errorf("decode agent attachment %s: %w", value.ID, err)
	}
	return attachment, nil
}

func turnModel(value Turn) *model.AgentTurn {
	var startedAt *time.Time
	if !value.StartedAt.IsZero() {
		startedAt = &value.StartedAt
	}
	return &model.AgentTurn{ID: value.ID, AgentSessionID: value.AgentSessionID, Status: uint8(value.Status), ActiveStepID: value.ActiveStepID,
		ErrorCode: value.ErrorCode, ErrorMessage: value.ErrorMessage, Version: value.Version, StartedAt: startedAt,
		CompletedAt: value.CompletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func turnFromModel(value *model.AgentTurn) Turn {
	var startedAt time.Time
	if value.StartedAt != nil {
		startedAt = *value.StartedAt
	}
	return Turn{ID: value.ID, AgentSessionID: value.AgentSessionID, Status: TurnStatus(value.Status), ActiveStepID: value.ActiveStepID,
		ErrorCode: value.ErrorCode, ErrorMessage: value.ErrorMessage, Version: value.Version, StartedAt: startedAt,
		CompletedAt: value.CompletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func stepModel(value Step) *model.AgentStep {
	toolID := ""
	if value.ToolID != nil {
		toolID = value.ToolID.String()
	}
	return &model.AgentStep{ID: value.ID, TurnID: value.TurnID, Sequence: value.Sequence, Kind: uint8(value.Kind), Status: uint8(value.Status),
		ToolID: toolID, ToolSnapshotID: value.ToolSnapshotID, Input: string(value.Input), Output: string(value.Output),
		ErrorCode: value.ErrorCode, ErrorMessage: value.ErrorMessage, Version: value.Version, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}
}

func stepFromModel(value *model.AgentStep) (Step, error) {
	var toolID *tool.ToolID
	if value.ToolID != "" {
		parsed, err := tool.ParseToolID(value.ToolID)
		if err != nil {
			return Step{}, fmt.Errorf("parse persisted tool ID: %w", err)
		}
		toolID = &parsed
	}
	return Step{ID: value.ID, TurnID: value.TurnID, Sequence: value.Sequence, Kind: StepKind(value.Kind), Status: StepStatus(value.Status),
		ToolID: toolID, ToolSnapshotID: value.ToolSnapshotID, Input: json.RawMessage(value.Input), Output: json.RawMessage(value.Output),
		ErrorCode: value.ErrorCode, ErrorMessage: value.ErrorMessage, Version: value.Version, StartedAt: value.StartedAt,
		CompletedAt: value.CompletedAt, CreatedAt: value.CreatedAt, UpdatedAt: value.UpdatedAt}, nil
}

func messageFromModel(record *model.AgentMessage) (Message, error) {
	var value ModelMessage
	if err := json.Unmarshal([]byte(record.Content), &value); err != nil {
		return Message{}, fmt.Errorf("decode agent message %s: %w", record.ID, err)
	}
	return Message{ID: record.ID, AgentSessionID: record.AgentSessionID, TurnID: record.TurnID,
		Sequence: record.Sequence, Value: value, CreatedAt: record.CreatedAt}, nil
}

func mapNotFound(err, sentinel error, id string) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: %s", sentinel, id)
	}
	return err
}

func versionConflict(kind, id string) error {
	return fmt.Errorf("%w: %s %s", ErrVersionConflict, kind, id)
}
