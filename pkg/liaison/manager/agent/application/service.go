package application

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/modelsettings"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

var (
	ErrInvalid     = errors.New("invalid agent request")
	ErrNotFound    = errors.New("agent resource not found")
	ErrUnavailable = errors.New("agent runtime unavailable")
)

const accessResourceType = "access"
const policyRevision = "agent-medium-risk-v1"

type Store interface {
	CreateSession(ctx context.Context, session runtime.Session) error
	CreateSessionWithAttachment(ctx context.Context, session runtime.Session, attachment runtime.Attachment) (runtime.Session, runtime.Attachment, error)
	GetSession(ctx context.Context, sessionID string) (runtime.Session, error)
	ListSessions(ctx context.Context, createdBy uint) ([]runtime.Session, error)
	ListAttachments(ctx context.Context, sessionID string) ([]runtime.Attachment, error)
	ListTurns(ctx context.Context, sessionID string) ([]runtime.Turn, error)
	ListSessionSteps(ctx context.Context, sessionID string) ([]runtime.Step, error)
	ListSessionMessages(ctx context.Context, sessionID string) ([]runtime.Message, error)
	ArchiveSession(ctx context.Context, sessionID string, expectedVersion uint64) (runtime.Session, error)
}

type AttachmentBinder interface {
	Bind(ctx context.Context, principal tool.Principal, handleID string) (tool.AttachmentSnapshot, error)
}

type ResourceRelations interface {
	ListIAMResourceRelations(resourceType string, resourceID uint64) ([]*model.IAMResourceRelation, error)
}

type Authorizer interface {
	RequireResourcePermission(actor *model.User, resource, action string) error
	RequireOrganizationResourcePermission(actor *model.User, organizationID uint, resource, action string) error
}

type IDGenerator interface {
	NewID(prefix string) (string, error)
}

type TurnRunner interface {
	Run(ctx context.Context, request runtime.RunRequest) (runtime.RunResult, error)
}

type ApprovalResolver interface {
	ResolveApproval(ctx context.Context, request runtime.RunRequest, approvalID string, approve bool, note string) (runtime.RunResult, error)
}

type ApprovalLister interface {
	ListApprovals(ctx context.Context, sessionID string) ([]runtime.ApprovalView, error)
}

type Option func(*Service)

func WithTurnRunner(runner TurnRunner) Option {
	return func(service *Service) {
		service.runner = runner
		service.approvals, _ = runner.(ApprovalResolver)
		service.approvalList, _ = runner.(ApprovalLister)
	}
}

type Service struct {
	references          ResourceReferenceResolver
	models              *modelsettings.Manager
	assistanceMu        sync.Mutex
	assistanceGenerator assistance.Generator
	assistants          map[assistanceKey]assistanceEntry
	store               Store
	binder              AttachmentBinder
	relations           ResourceRelations
	authorizer          Authorizer
	ids                 IDGenerator
	runner              TurnRunner
	approvals           ApprovalResolver
	approvalList        ApprovalLister
}

type assistanceKey struct {
	owner          uint
	handle, editor string
}
type assistanceEntry struct {
	session *assistance.Session
	used    time.Time
}

func WithAssistanceGenerator(generator assistance.Generator) Option {
	return func(s *Service) { s.assistanceGenerator = generator }
}

type CreateSessionRequest struct {
	Kind     tool.SessionKind
	Actor    *model.User
	HandleID string
	Title    string
}

// Available reports whether this process has a configured turn runner.
func (service *Service) Available() bool {
	return service.runner != nil && (service.models == nil || service.models.Enabled(context.Background()))
}

func WithModelSettings(models *modelsettings.Manager) Option {
	return func(s *Service) { s.models = models }
}

func (service *Service) ModelSettings() *modelsettings.Manager { return service.models }

type SessionDetail struct {
	Session     runtime.Session        `json:"session"`
	Attachments []runtime.Attachment   `json:"attachments"`
	Turns       []runtime.Turn         `json:"turns"`
	Steps       []runtime.Step         `json:"steps"`
	Messages    []runtime.Message      `json:"messages"`
	Approvals   []runtime.ApprovalView `json:"approvals"`
}

type RunTurnRequest struct {
	References     []runtime.ResourceReference
	ModelSelection runtime.ModelSelection
	Actor          *model.User
	SessionID      string
	Prompt         string
}

type ResolveApprovalRequest struct {
	Actor      *model.User
	SessionID  string
	ApprovalID string
	Approve    bool
	Note       string
}

func NewService(store Store, binder AttachmentBinder, relations ResourceRelations, authorizer Authorizer, ids IDGenerator, options ...Option) (*Service, error) {
	if store == nil || binder == nil || relations == nil || authorizer == nil {
		return nil, errors.New("agent application service requires store, binder, resource relations and authorizer")
	}
	if ids == nil {
		ids = randomIDGenerator{}
	}
	service := &Service{store: store, binder: binder, relations: relations, authorizer: authorizer, ids: ids}
	for _, option := range options {
		if option != nil {
			option(service)
		}
	}
	return service, nil
}

func (service *Service) RunTurn(ctx context.Context, request RunTurnRequest) (runtime.RunResult, error) {
	prompt := strings.TrimSpace(request.Prompt)
	if request.Actor == nil || request.Actor.ID == 0 || strings.TrimSpace(request.SessionID) == "" || prompt == "" {
		return runtime.RunResult{}, fmt.Errorf("%w: actor, session and prompt are required", ErrInvalid)
	}
	if len([]byte(prompt)) > 64*1024 {
		return runtime.RunResult{}, fmt.Errorf("%w: prompt exceeds 64 KiB", ErrInvalid)
	}
	if service.runner == nil {
		return runtime.RunResult{}, ErrUnavailable
	}
	session, err := service.ownedSession(ctx, request.Actor, request.SessionID, "use")
	if err != nil {
		return runtime.RunResult{}, err
	}
	selection := request.ModelSelection
	refs, err := service.resolveReferences(ctx, request.Actor, session, request.References)
	if err != nil {
		return runtime.RunResult{}, err
	}
	if service.models != nil {
		selection, err = service.models.ResolveSelection(ctx, selection)
		if err != nil {
			return runtime.RunResult{}, fmt.Errorf("%w: selected model unavailable", ErrInvalid)
		}
	} else if selection.ProviderID != "" || selection.Model != "" {
		return runtime.RunResult{}, ErrInvalid
	}
	result, err := service.runner.Run(ctx, runtime.RunRequest{
		References:     refs,
		Selection:      selection,
		SessionID:      session.ID,
		Prompt:         prompt,
		Principal:      tool.Principal{UserID: request.Actor.ID, OrganizationID: session.OrganizationID},
		PolicyRevision: policyRevision,
		Budget:         tool.DefaultDisclosureBudget(),
	})
	if err != nil {
		return result, fmt.Errorf("run agent turn: %w", err)
	}
	return result, nil
}

func (service *Service) ResolveApproval(ctx context.Context, request ResolveApprovalRequest) (runtime.RunResult, error) {
	if request.Actor == nil || request.Actor.ID == 0 || strings.TrimSpace(request.SessionID) == "" || strings.TrimSpace(request.ApprovalID) == "" {
		return runtime.RunResult{}, fmt.Errorf("%w: actor, session and approval are required", ErrInvalid)
	}
	if len([]byte(request.Note)) > 1024 {
		return runtime.RunResult{}, fmt.Errorf("%w: approval note exceeds 1 KiB", ErrInvalid)
	}
	if service.approvals == nil {
		return runtime.RunResult{}, ErrUnavailable
	}
	session, err := service.ownedSession(ctx, request.Actor, request.SessionID, "use")
	if err != nil {
		return runtime.RunResult{}, err
	}
	return service.approvals.ResolveApproval(ctx, runtime.RunRequest{
		SessionID:      session.ID,
		Principal:      tool.Principal{UserID: request.Actor.ID, OrganizationID: session.OrganizationID},
		PolicyRevision: policyRevision,
		Budget:         tool.DefaultDisclosureBudget(),
	}, strings.TrimSpace(request.ApprovalID), request.Approve, strings.TrimSpace(request.Note))
}

func (service *Service) CreateSession(ctx context.Context, request CreateSessionRequest) (SessionDetail, error) {
	if !request.Kind.Valid() {
		return SessionDetail{}, fmt.Errorf("%w: unknown session kind", ErrInvalid)
	}
	if request.Kind == tool.SessionManagement {
		return service.createManagementSession(ctx, request)
	}
	if request.Actor == nil || request.Actor.ID == 0 || strings.TrimSpace(request.HandleID) == "" {
		return SessionDetail{}, fmt.Errorf("%w: actor and handle are required", ErrInvalid)
	}
	snapshot, err := service.binder.Bind(ctx, tool.Principal{UserID: request.Actor.ID}, strings.TrimSpace(request.HandleID))
	if err != nil {
		return SessionDetail{}, fmt.Errorf("bind live access session: %w", err)
	}
	organizationID, err := service.resourceOrganization(snapshot.AccessID)
	if err != nil {
		return SessionDetail{}, err
	}
	if err := service.authorizer.RequireOrganizationResourcePermission(request.Actor, organizationID, "agent_sessions", "create"); err != nil {
		return SessionDetail{}, err
	}
	sessionID, err := service.ids.NewID("session")
	if err != nil {
		return SessionDetail{}, fmt.Errorf("create agent session ID: %w", err)
	}
	session := runtime.Session{
		Kind:           tool.SessionAccess,
		ID:             sessionID,
		OrganizationID: organizationID,
		CreatedBy:      request.Actor.ID,
		Title:          normalizedTitle(request.Title, snapshot.Protocol),
		Status:         runtime.SessionActive,
	}
	attachment := runtime.Attachment{
		ID:             snapshot.AccessHandleID,
		AgentSessionID: sessionID,
		AccessID:       snapshot.AccessID,
		ApplicationID:  snapshot.ApplicationID,
		Protocol:       snapshot.Protocol,
		Capabilities:   append([]tool.Capability(nil), snapshot.Capabilities...),
		Generation:     snapshot.Generation,
		State:          runtime.AttachmentConnected,
	}
	session, attachment, err = service.store.CreateSessionWithAttachment(ctx, session, attachment)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("create agent session: %w", err)
	}
	return SessionDetail{Session: session, Attachments: []runtime.Attachment{attachment}}, nil
}

func (service *Service) ListSessions(ctx context.Context, actor *model.User) ([]runtime.Session, error) {
	if actor == nil || actor.ID == 0 {
		return nil, fmt.Errorf("%w: actor is required", ErrInvalid)
	}
	accessErr := service.authorizer.RequireResourcePermission(actor, "agent_sessions", "read")
	homeErr := service.authorizer.RequireResourcePermission(actor, "management_agent_sessions", "read")
	if accessErr != nil && homeErr != nil {
		return nil, accessErr
	}
	sessions, err := service.store.ListSessions(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	visible := make([]runtime.Session, 0, len(sessions))
	for _, session := range sessions {
		if (session.Kind == tool.SessionManagement && homeErr == nil) || (session.Kind != tool.SessionManagement && accessErr == nil) {
			visible = append(visible, session)
		}
	}
	return visible, nil
}

func (service *Service) GetSession(ctx context.Context, actor *model.User, sessionID string) (SessionDetail, error) {
	session, err := service.ownedSession(ctx, actor, sessionID, "read")
	if err != nil {
		return SessionDetail{}, err
	}
	attachments, err := service.store.ListAttachments(ctx, session.ID)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("list agent attachments: %w", err)
	}
	turns, err := service.store.ListTurns(ctx, session.ID)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("list agent turns: %w", err)
	}
	steps, err := service.store.ListSessionSteps(ctx, session.ID)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("list agent steps: %w", err)
	}
	messages, err := service.store.ListSessionMessages(ctx, session.ID)
	if err != nil {
		return SessionDetail{}, fmt.Errorf("list agent messages: %w", err)
	}
	var approvals []runtime.ApprovalView
	if service.approvalList != nil {
		approvals, err = service.approvalList.ListApprovals(ctx, session.ID)
		if err != nil {
			return SessionDetail{}, fmt.Errorf("list agent approvals: %w", err)
		}
	}
	return SessionDetail{Session: session, Attachments: attachments, Turns: turns, Steps: steps, Messages: messages, Approvals: approvals}, nil
}

// AuthorizeSession revalidates streaming access without loading message history.
func (service *Service) AuthorizeSession(ctx context.Context, actor *model.User, sessionID string) error {
	_, err := service.ownedSession(ctx, actor, sessionID, "read")
	return err
}

func (service *Service) ArchiveSession(ctx context.Context, actor *model.User, sessionID string, expectedVersion uint64) (runtime.Session, error) {
	session, err := service.ownedSession(ctx, actor, sessionID, "delete")
	if err != nil {
		return runtime.Session{}, err
	}
	if expectedVersion == 0 || expectedVersion != session.Version {
		return runtime.Session{}, fmt.Errorf("%w: expected version is required", ErrInvalid)
	}
	return service.store.ArchiveSession(ctx, session.ID, expectedVersion)
}

func (service *Service) ownedSession(ctx context.Context, actor *model.User, sessionID, action string) (runtime.Session, error) {
	if actor == nil || actor.ID == 0 || strings.TrimSpace(sessionID) == "" {
		return runtime.Session{}, fmt.Errorf("%w: actor and session are required", ErrInvalid)
	}
	session, err := service.store.GetSession(ctx, strings.TrimSpace(sessionID))
	if err != nil {
		if errors.Is(err, runtime.ErrSessionNotFound) {
			return runtime.Session{}, ErrNotFound
		}
		return runtime.Session{}, err
	}
	if session.CreatedBy != actor.ID {
		return runtime.Session{}, ErrNotFound
	}
	resource := "agent_sessions"
	if session.Kind == tool.SessionManagement {
		resource = "management_agent_sessions"
	}
	if err := service.authorizer.RequireOrganizationResourcePermission(actor, session.OrganizationID, resource, action); err != nil {
		return runtime.Session{}, err
	}
	return session, nil
}

func (service *Service) resourceOrganization(accessID uint) (uint, error) {
	relations, err := service.relations.ListIAMResourceRelations(accessResourceType, uint64(accessID))
	if err != nil {
		return 0, fmt.Errorf("list access resource relations: %w", err)
	}
	for _, relation := range relations {
		if relation != nil && relation.Relation == model.IAMRelationBelongsTo && relation.SubjectType == model.IAMSubjectOrganization && relation.SubjectID != 0 {
			return relation.SubjectID, nil
		}
	}
	return 0, fmt.Errorf("%w: access organization relation is missing", ErrNotFound)
}

func normalizedTitle(title string, protocol tool.Protocol) string {
	title = strings.TrimSpace(title)
	runes := []rune(title)
	if len(runes) > 255 {
		title = string(runes[:255])
	}
	if title != "" {
		return title
	}
	return "New " + strings.ToUpper(strings.ReplaceAll(string(protocol), "_", " ")) + " session"
}

type randomIDGenerator struct{}

func (randomIDGenerator) NewID(prefix string) (string, error) {
	buffer := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, buffer); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(buffer), nil
}
