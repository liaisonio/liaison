package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

type Store interface {
	AttachmentStore
	CreateSession(ctx context.Context, session Session) error
	CreateSessionWithAttachment(ctx context.Context, session Session, attachment Attachment) (Session, Attachment, error)
	GetSession(ctx context.Context, sessionID string) (Session, error)
	ListSessions(ctx context.Context, createdBy uint) ([]Session, error)
	ArchiveSession(ctx context.Context, sessionID string, expectedVersion uint64) (Session, error)
	StartTurn(ctx context.Context, sessionID string, turn Turn) (Session, Turn, error)
	GetTurn(ctx context.Context, turnID string) (Turn, error)
	ListTurns(ctx context.Context, sessionID string) ([]Turn, error)
	TransitionTurn(ctx context.Context, turnID string, expectedVersion uint64, to TurnStatus, code, message string) (Turn, error)
	AppendStep(ctx context.Context, turnID string, expectedTurnVersion uint64, step Step) (Turn, Step, error)
	GetStep(ctx context.Context, stepID string) (Step, error)
	ListSessionSteps(ctx context.Context, sessionID string) ([]Step, error)
	CompleteStep(ctx context.Context, stepID string, output json.RawMessage) (Step, error)
	WaitStepApproval(ctx context.Context, stepID string) (Step, error)
	ResumeStep(ctx context.Context, stepID string) (Step, error)
	CancelStep(ctx context.Context, stepID, code, message string) (Step, error)
	FailStep(ctx context.Context, stepID, code, message string) (Step, error)
	AppendMessage(ctx context.Context, message Message) (Message, error)
	ListMessages(ctx context.Context, turnID string) ([]Message, error)
	ListSessionMessages(ctx context.Context, sessionID string) ([]Message, error)
	SaveToolSnapshot(ctx context.Context, turnID, modelStepID string, snapshot tool.ToolSetSnapshot) error
	GetToolSnapshot(ctx context.Context, snapshotID string) (tool.ToolSetSnapshot, error)
}

type AttachmentStore interface {
	CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error)
	GetAttachment(ctx context.Context, attachmentID string) (Attachment, error)
	ListAttachments(ctx context.Context, sessionID string) ([]Attachment, error)
	UpdateAttachment(ctx context.Context, attachment Attachment, expectedGeneration uint64) (Attachment, error)
	SetActiveAttachment(ctx context.Context, sessionID, attachmentID string, expectedVersion uint64) (Session, error)
}

// MemoryStore is useful for tests and local runtime bring-up. Production wiring
// must use a durable Store before Agent HTTP APIs are enabled.
type MemoryStore struct {
	mu          sync.RWMutex
	sessions    map[string]Session
	turns       map[string]Turn
	steps       map[string]Step
	messages    map[string][]Message
	snapshots   map[string]tool.ToolSetSnapshot
	attachments map[string]Attachment
	clock       func() time.Time
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		sessions:    make(map[string]Session),
		turns:       make(map[string]Turn),
		steps:       make(map[string]Step),
		messages:    make(map[string][]Message),
		snapshots:   make(map[string]tool.ToolSetSnapshot),
		attachments: make(map[string]Attachment),
		clock:       time.Now,
	}
}

func (store *MemoryStore) CreateAttachment(ctx context.Context, attachment Attachment) (Attachment, error) {
	if err := ctx.Err(); err != nil {
		return Attachment{}, err
	}
	if err := validateAttachment(attachment); err != nil {
		return Attachment{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[attachment.AgentSessionID]
	if !ok {
		return Attachment{}, fmt.Errorf("%w: %s", ErrSessionNotFound, attachment.AgentSessionID)
	}
	if session.Kind == tool.SessionManagement {
		return Attachment{}, errors.New("management sessions cannot have access attachments")
	}
	if session.Status == SessionArchived {
		return Attachment{}, fmt.Errorf("%w: %s", ErrSessionArchived, session.ID)
	}
	if _, exists := store.attachments[attachment.ID]; exists {
		return Attachment{}, fmt.Errorf("attachment %q already exists", attachment.ID)
	}
	now := store.clock().UTC()
	if attachment.Generation == 0 {
		attachment.Generation = 1
	}
	attachment.CreatedAt = now
	attachment.UpdatedAt = now
	attachment.Capabilities = append([]tool.Capability(nil), attachment.Capabilities...)
	store.attachments[attachment.ID] = attachment
	return cloneAttachment(attachment), nil
}

func (store *MemoryStore) GetAttachment(ctx context.Context, attachmentID string) (Attachment, error) {
	if err := ctx.Err(); err != nil {
		return Attachment{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	attachment, ok := store.attachments[attachmentID]
	if !ok {
		return Attachment{}, fmt.Errorf("%w: %s", ErrAttachmentNotFound, attachmentID)
	}
	return cloneAttachment(attachment), nil
}

func (store *MemoryStore) ListAttachments(ctx context.Context, sessionID string) ([]Attachment, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if _, ok := store.sessions[sessionID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	attachments := make([]Attachment, 0)
	for _, attachment := range store.attachments {
		if attachment.AgentSessionID == sessionID {
			attachments = append(attachments, cloneAttachment(attachment))
		}
	}
	sort.Slice(attachments, func(i, j int) bool {
		if attachments[i].CreatedAt.Equal(attachments[j].CreatedAt) {
			return attachments[i].ID < attachments[j].ID
		}
		return attachments[i].CreatedAt.Before(attachments[j].CreatedAt)
	})
	return attachments, nil
}

func (store *MemoryStore) UpdateAttachment(ctx context.Context, attachment Attachment, expectedGeneration uint64) (Attachment, error) {
	if err := ctx.Err(); err != nil {
		return Attachment{}, err
	}
	if err := validateAttachment(attachment); err != nil {
		return Attachment{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	current, ok := store.attachments[attachment.ID]
	if !ok {
		return Attachment{}, fmt.Errorf("%w: %s", ErrAttachmentNotFound, attachment.ID)
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
	attachment.Capabilities = append([]tool.Capability(nil), attachment.Capabilities...)
	store.attachments[attachment.ID] = attachment
	return cloneAttachment(attachment), nil
}

func (store *MemoryStore) SetActiveAttachment(ctx context.Context, sessionID, attachmentID string, expectedVersion uint64) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[sessionID]
	if !ok {
		return Session{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	if session.Version != expectedVersion {
		return Session{}, versionConflict("session", sessionID)
	}
	if session.Kind == tool.SessionManagement && attachmentID != "" {
		return Session{}, errors.New("management sessions cannot activate access attachments")
	}
	if session.Status == SessionArchived {
		return Session{}, fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
	}
	if attachmentID != "" {
		attachment, exists := store.attachments[attachmentID]
		if !exists || attachment.AgentSessionID != sessionID || attachment.State == AttachmentRemoved {
			return Session{}, fmt.Errorf("%w: %s", ErrAttachmentNotFound, attachmentID)
		}
	}
	session.ActiveAttachmentID = attachmentID
	session.Version++
	session.UpdatedAt = store.clock().UTC()
	store.sessions[sessionID] = session
	return session, nil
}

func cloneAttachment(attachment Attachment) Attachment {
	attachment.Capabilities = append([]tool.Capability(nil), attachment.Capabilities...)
	return attachment
}

func (store *MemoryStore) CreateSession(ctx context.Context, session Session) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateSession(session); err != nil {
		return err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.sessions[session.ID]; exists {
		return fmt.Errorf("session %q already exists", session.ID)
	}
	now := store.clock().UTC()
	session.Version = 1
	session.CreatedAt = now
	session.UpdatedAt = now
	store.sessions[session.ID] = session
	return nil
}

func (store *MemoryStore) CreateSessionWithAttachment(ctx context.Context, session Session, attachment Attachment) (Session, Attachment, error) {
	if session.Kind == tool.SessionManagement {
		return Session{}, Attachment{}, errors.New("management sessions cannot have access attachments")
	}
	if err := ctx.Err(); err != nil {
		return Session{}, Attachment{}, err
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
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.sessions[session.ID]; exists {
		return Session{}, Attachment{}, fmt.Errorf("session %q already exists", session.ID)
	}
	if _, exists := store.attachments[attachment.ID]; exists {
		return Session{}, Attachment{}, fmt.Errorf("attachment %q already exists", attachment.ID)
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
	attachment.Capabilities = append([]tool.Capability(nil), attachment.Capabilities...)
	session.ActiveAttachmentID = attachment.ID
	store.sessions[session.ID] = session
	store.attachments[attachment.ID] = attachment
	return session, cloneAttachment(attachment), nil
}

func (store *MemoryStore) GetSession(ctx context.Context, sessionID string) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	session, ok := store.sessions[sessionID]
	if !ok {
		return Session{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	return session, nil
}

func (store *MemoryStore) ListSessions(ctx context.Context, createdBy uint) ([]Session, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	sessions := make([]Session, 0, len(store.sessions))
	for _, session := range store.sessions {
		if createdBy == 0 || session.CreatedBy == createdBy {
			sessions = append(sessions, session)
		}
	}
	sort.Slice(sessions, func(i, j int) bool {
		if sessions[i].UpdatedAt.Equal(sessions[j].UpdatedAt) {
			return sessions[i].ID < sessions[j].ID
		}
		return sessions[i].UpdatedAt.After(sessions[j].UpdatedAt)
	})
	return sessions, nil
}

func (store *MemoryStore) ArchiveSession(ctx context.Context, sessionID string, expectedVersion uint64) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[sessionID]
	if !ok {
		return Session{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	if session.Version != expectedVersion {
		return Session{}, fmt.Errorf("%w: session %s", ErrVersionConflict, sessionID)
	}
	if session.ActiveTurnID != "" {
		return Session{}, fmt.Errorf("%w: session %s has active turn %s", ErrTurnAlreadyActive, sessionID, session.ActiveTurnID)
	}
	session.Status = SessionArchived
	session.Version++
	session.UpdatedAt = store.clock().UTC()
	store.sessions[sessionID] = session
	return session, nil
}

func (store *MemoryStore) StartTurn(ctx context.Context, sessionID string, turn Turn) (Session, Turn, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, Turn{}, err
	}
	if turn.ID == "" {
		return Session{}, Turn{}, errors.New("turn ID is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[sessionID]
	if !ok {
		return Session{}, Turn{}, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	if session.Status == SessionArchived {
		return Session{}, Turn{}, fmt.Errorf("%w: %s", ErrSessionArchived, sessionID)
	}
	if session.ActiveTurnID != "" {
		return Session{}, Turn{}, fmt.Errorf("%w: %s", ErrTurnAlreadyActive, session.ActiveTurnID)
	}
	if _, exists := store.turns[turn.ID]; exists {
		return Session{}, Turn{}, fmt.Errorf("turn %q already exists", turn.ID)
	}
	now := store.clock().UTC()
	turn.AgentSessionID = sessionID
	turn.Status = TurnQueued
	turn.Version = 1
	turn.CreatedAt = now
	turn.UpdatedAt = now
	session.ActiveTurnID = turn.ID
	session.Version++
	session.UpdatedAt = now
	store.turns[turn.ID] = turn
	store.sessions[sessionID] = session
	return session, turn, nil
}

func (store *MemoryStore) GetTurn(ctx context.Context, turnID string) (Turn, error) {
	if err := ctx.Err(); err != nil {
		return Turn{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	turn, ok := store.turns[turnID]
	if !ok {
		return Turn{}, fmt.Errorf("%w: %s", ErrTurnNotFound, turnID)
	}
	return turn, nil
}

func (store *MemoryStore) ListTurns(ctx context.Context, sessionID string) ([]Turn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if _, ok := store.sessions[sessionID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	turns := make([]Turn, 0)
	for _, turn := range store.turns {
		if turn.AgentSessionID == sessionID {
			turns = append(turns, turn)
		}
	}
	sort.Slice(turns, func(i, j int) bool {
		if turns[i].CreatedAt.Equal(turns[j].CreatedAt) {
			return turns[i].ID < turns[j].ID
		}
		return turns[i].CreatedAt.Before(turns[j].CreatedAt)
	})
	return turns, nil
}

func (store *MemoryStore) TransitionTurn(ctx context.Context, turnID string, expectedVersion uint64, to TurnStatus, code, message string) (Turn, error) {
	if err := ctx.Err(); err != nil {
		return Turn{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	turn, ok := store.turns[turnID]
	if !ok {
		return Turn{}, fmt.Errorf("%w: %s", ErrTurnNotFound, turnID)
	}
	if turn.Version != expectedVersion {
		return Turn{}, fmt.Errorf("%w: turn %s", ErrVersionConflict, turnID)
	}
	if !validTurnTransition(turn.Status, to) {
		return Turn{}, fmt.Errorf("%w: turn %s from %d to %d", ErrInvalidTransition, turnID, turn.Status, to)
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
		session := store.sessions[turn.AgentSessionID]
		if session.ActiveTurnID == turn.ID {
			session.ActiveTurnID = ""
			session.Version++
			session.UpdatedAt = now
			store.sessions[session.ID] = session
		}
	}
	store.turns[turnID] = turn
	return turn, nil
}

func (store *MemoryStore) AppendStep(ctx context.Context, turnID string, expectedTurnVersion uint64, step Step) (Turn, Step, error) {
	if err := ctx.Err(); err != nil {
		return Turn{}, Step{}, err
	}
	if step.ID == "" {
		return Turn{}, Step{}, errors.New("step ID is required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	turn, ok := store.turns[turnID]
	if !ok {
		return Turn{}, Step{}, fmt.Errorf("%w: %s", ErrTurnNotFound, turnID)
	}
	if turn.Version != expectedTurnVersion {
		return Turn{}, Step{}, fmt.Errorf("%w: turn %s", ErrVersionConflict, turnID)
	}
	if turn.Status.Terminal() {
		return Turn{}, Step{}, fmt.Errorf("%w: cannot append to terminal turn %s", ErrInvalidTransition, turnID)
	}
	if _, exists := store.steps[step.ID]; exists {
		return Turn{}, Step{}, fmt.Errorf("step %q already exists", step.ID)
	}
	now := store.clock().UTC()
	step.TurnID = turnID
	step.Sequence = store.nextStepSequenceLocked(turnID)
	step.Version = 1
	step.CreatedAt = now
	step.UpdatedAt = now
	if step.Status == StepRunning {
		step.StartedAt = &now
	}
	turn.ActiveStepID = step.ID
	turn.Version++
	turn.UpdatedAt = now
	store.steps[step.ID] = cloneStep(step)
	store.turns[turnID] = turn
	return turn, cloneStep(step), nil
}

func (store *MemoryStore) CompleteStep(ctx context.Context, stepID string, output json.RawMessage) (Step, error) {
	return store.transitionStep(ctx, stepID, StepCompleted, output)
}

func (store *MemoryStore) WaitStepApproval(ctx context.Context, stepID string) (Step, error) {
	return store.transitionStep(ctx, stepID, StepWaitingApproval, nil)
}

func (store *MemoryStore) ResumeStep(ctx context.Context, stepID string) (Step, error) {
	return store.transitionStep(ctx, stepID, StepRunning, nil)
}

func (store *MemoryStore) CancelStep(ctx context.Context, stepID, code, message string) (Step, error) {
	return store.finishStep(ctx, stepID, StepCancelled, code, message)
}

func (store *MemoryStore) FailStep(ctx context.Context, stepID, code, message string) (Step, error) {
	return store.finishStep(ctx, stepID, StepFailed, code, message)
}

func (store *MemoryStore) finishStep(ctx context.Context, stepID string, status StepStatus, code, message string) (Step, error) {
	_, err := store.transitionStep(ctx, stepID, status, nil)
	if err != nil {
		return Step{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	current := store.steps[stepID]
	current.ErrorCode = code
	current.ErrorMessage = message
	store.steps[stepID] = cloneStep(current)
	return cloneStep(current), nil
}

func (store *MemoryStore) transitionStep(ctx context.Context, stepID string, to StepStatus, output json.RawMessage) (Step, error) {
	if err := ctx.Err(); err != nil {
		return Step{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	step, ok := store.steps[stepID]
	if !ok {
		return Step{}, fmt.Errorf("%w: %s", ErrStepNotFound, stepID)
	}
	if !validStepTransition(step.Status, to) {
		return Step{}, fmt.Errorf("%w: step %s from %d to %d", ErrInvalidTransition, stepID, step.Status, to)
	}
	now := store.clock().UTC()
	step.Status = to
	step.Version++
	step.UpdatedAt = now
	if len(output) > 0 {
		step.Output = append(json.RawMessage(nil), output...)
	}
	if to == StepCompleted || to == StepCancelled || to == StepFailed {
		step.CompletedAt = &now
	}
	store.steps[stepID] = cloneStep(step)
	return cloneStep(step), nil
}

func (store *MemoryStore) GetStep(ctx context.Context, stepID string) (Step, error) {
	if err := ctx.Err(); err != nil {
		return Step{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	step, ok := store.steps[stepID]
	if !ok {
		return Step{}, fmt.Errorf("%w: %s", ErrStepNotFound, stepID)
	}
	return cloneStep(step), nil
}

func (store *MemoryStore) ListSessionSteps(ctx context.Context, sessionID string) ([]Step, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if _, ok := store.sessions[sessionID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	turnIDs := make(map[string]struct{})
	for _, turn := range store.turns {
		if turn.AgentSessionID == sessionID {
			turnIDs[turn.ID] = struct{}{}
		}
	}
	steps := make([]Step, 0)
	for _, step := range store.steps {
		if _, ok := turnIDs[step.TurnID]; ok {
			steps = append(steps, cloneStep(step))
		}
	}
	sort.Slice(steps, func(i, j int) bool {
		if steps[i].CreatedAt.Equal(steps[j].CreatedAt) {
			if steps[i].TurnID == steps[j].TurnID {
				return steps[i].Sequence < steps[j].Sequence
			}
			return steps[i].TurnID < steps[j].TurnID
		}
		return steps[i].CreatedAt.Before(steps[j].CreatedAt)
	})
	return steps, nil
}

func (store *MemoryStore) AppendMessage(ctx context.Context, message Message) (Message, error) {
	if err := ctx.Err(); err != nil {
		return Message{}, err
	}
	if message.ID == "" || message.AgentSessionID == "" || message.TurnID == "" {
		return Message{}, errors.New("message ID, session and turn are required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	messages := store.messages[message.TurnID]
	for _, current := range messages {
		if current.ID == message.ID {
			return Message{}, fmt.Errorf("message %q already exists", message.ID)
		}
	}
	message.Sequence = uint32(len(messages) + 1)
	message.CreatedAt = store.clock().UTC()
	message.Value = cloneMessages([]ModelMessage{message.Value})[0]
	store.messages[message.TurnID] = append(messages, message)
	return message, nil
}

func (store *MemoryStore) ListMessages(ctx context.Context, turnID string) ([]Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	stored := store.messages[turnID]
	result := make([]Message, len(stored))
	for index, message := range stored {
		result[index] = message
		result[index].Value = cloneMessages([]ModelMessage{message.Value})[0]
	}
	return result, nil
}

func (store *MemoryStore) ListSessionMessages(ctx context.Context, sessionID string) ([]Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	if _, ok := store.sessions[sessionID]; !ok {
		return nil, fmt.Errorf("%w: %s", ErrSessionNotFound, sessionID)
	}
	messages := make([]Message, 0)
	for _, turnMessages := range store.messages {
		for _, message := range turnMessages {
			if message.AgentSessionID == sessionID {
				message.Value = cloneMessages([]ModelMessage{message.Value})[0]
				messages = append(messages, message)
			}
		}
	}
	sort.Slice(messages, func(i, j int) bool {
		if messages[i].CreatedAt.Equal(messages[j].CreatedAt) {
			if messages[i].TurnID == messages[j].TurnID {
				return messages[i].Sequence < messages[j].Sequence
			}
			return messages[i].TurnID < messages[j].TurnID
		}
		return messages[i].CreatedAt.Before(messages[j].CreatedAt)
	})
	return messages, nil
}

func (store *MemoryStore) SaveToolSnapshot(ctx context.Context, turnID, modelStepID string, snapshot tool.ToolSetSnapshot) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if turnID == "" || modelStepID == "" || snapshot.ID == "" {
		return errors.New("turn, model step and snapshot IDs are required")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.snapshots[snapshot.ID]; exists {
		return fmt.Errorf("tool snapshot %q already exists", snapshot.ID)
	}
	store.snapshots[snapshot.ID] = cloneToolSnapshot(snapshot)
	return nil
}

func (store *MemoryStore) GetToolSnapshot(ctx context.Context, snapshotID string) (tool.ToolSetSnapshot, error) {
	if err := ctx.Err(); err != nil {
		return tool.ToolSetSnapshot{}, err
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	snapshot, ok := store.snapshots[snapshotID]
	if !ok {
		return tool.ToolSetSnapshot{}, fmt.Errorf("tool snapshot %q not found", snapshotID)
	}
	return cloneToolSnapshot(snapshot), nil
}

func (store *MemoryStore) nextStepSequenceLocked(turnID string) uint32 {
	var next uint32 = 1
	for _, step := range store.steps {
		if step.TurnID == turnID && step.Sequence >= next {
			next = step.Sequence + 1
		}
	}
	return next
}

func cloneStep(step Step) Step {
	cloned := step
	cloned.Input = append([]byte(nil), step.Input...)
	cloned.Output = append([]byte(nil), step.Output...)
	if step.ToolID != nil {
		toolID := *step.ToolID
		cloned.ToolID = &toolID
	}
	return cloned
}

func cloneToolSnapshot(snapshot tool.ToolSetSnapshot) tool.ToolSetSnapshot {
	cloned := snapshot
	cloned.AttachmentGeneration = make(map[string]uint64, len(snapshot.AttachmentGeneration))
	for id, generation := range snapshot.AttachmentGeneration {
		cloned.AttachmentGeneration[id] = generation
	}
	cloned.Tools = append([]tool.ExposedTool(nil), snapshot.Tools...)
	for index := range cloned.Tools {
		cloned.Tools[index].Descriptor.InputSchema = append(json.RawMessage(nil), snapshot.Tools[index].Descriptor.InputSchema...)
	}
	return cloned
}
