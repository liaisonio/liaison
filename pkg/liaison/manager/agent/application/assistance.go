package application

import (
	"context"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// NewAssistanceSession creates an editor cancellation lane. SSH generation
// reads the owning Shell Agent session; this lane owns no conversation history.
func (service *Service) NewAssistanceSession(ctx context.Context, actor *model.User, handleID string, generator assistance.Generator) (*assistance.Session, error) {
	if actor == nil || actor.ID == 0 || strings.TrimSpace(handleID) == "" || generator == nil {
		return nil, ErrInvalid
	}
	snapshot, err := service.binder.Bind(ctx, tool.Principal{UserID: actor.ID}, handleID)
	if err != nil {
		return nil, err
	}
	organizationID, err := service.resourceOrganization(snapshot.AccessID)
	if err != nil {
		return nil, err
	}
	if err := service.authorizer.RequireOrganizationResourcePermission(actor, organizationID, "agent_sessions", "use"); err != nil {
		return nil, err
	}
	protocol := string(snapshot.Protocol)
	if snapshot.Protocol == tool.ProtocolWebSSH {
		protocol = "ssh"
	}
	// 保存用户副本，避免请求调用方之后修改身份字段。
	owner := *actor
	if protocol == "ssh" {
		generator = &shellGenerator{service: service, actor: owner, inner: generator}
	}
	return assistance.NewSession(assistance.Binding{OwnerID: owner.ID, HandleID: snapshot.AccessHandleID, Protocol: protocol}, generator, &assistanceGuard{
		service: service, actor: owner, snapshot: snapshot, organizationID: organizationID,
	})
}

type shellContextReader interface {
	ShellCompletionContext(context.Context, tool.Principal, string) ([]runtime.ModelMessage, error)
}

type shellGenerator struct {
	service *Service
	actor   model.User
	inner   assistance.Generator
}

func (g *shellGenerator) validate(ctx context.Context, binding assistance.Binding, sessionID string) (runtime.Session, error) {
	if sessionID == "" || len(sessionID) > 256 {
		return runtime.Session{}, assistance.ErrInvalid
	}
	session, err := g.service.ownedSession(ctx, &g.actor, sessionID, "use")
	if err != nil {
		return runtime.Session{}, err
	}
	if session.Kind != tool.SessionShell || session.Status != runtime.SessionActive {
		return runtime.Session{}, assistance.ErrClosed
	}
	attachments, err := g.service.store.ListAttachments(ctx, session.ID)
	if err != nil {
		return runtime.Session{}, err
	}
	if len(attachments) != 1 || attachments[0].ID != session.ActiveAttachmentID || attachments[0].Protocol != tool.ProtocolWebSSH || attachments[0].HandleID() != binding.HandleID {
		return runtime.Session{}, assistance.ErrClosed
	}
	if err := g.service.checkLiveAttachments(ctx, &g.actor, session); err != nil {
		return runtime.Session{}, err
	}
	return session, nil
}

func (g *shellGenerator) Suggest(ctx context.Context, binding assistance.Binding, input assistance.Input) (string, error) {
	session, err := g.validate(ctx, binding, input.AgentSessionID)
	if err != nil {
		return "", err
	}
	reader, ok := g.service.runner.(shellContextReader)
	if !ok {
		return "", ErrUnavailable
	}
	input.AgentSessionID = session.ID
	input.AgentContext, err = reader.ShellCompletionContext(ctx, tool.Principal{UserID: g.actor.ID, OrganizationID: session.OrganizationID}, session.ID)
	if err != nil {
		return "", err
	}
	text, err := g.inner.Suggest(ctx, binding, input)
	if err != nil {
		return "", err
	}
	if _, err := g.validate(ctx, binding, session.ID); err != nil {
		return "", err
	}
	return text, nil
}

type assistanceGuard struct {
	service        *Service
	actor          model.User
	snapshot       tool.AttachmentSnapshot
	organizationID uint
}

func (guard *assistanceGuard) Check(ctx context.Context, binding assistance.Binding) error {
	if binding.OwnerID != guard.actor.ID || binding.HandleID != guard.snapshot.AccessHandleID {
		return assistance.ErrClosed
	}
	live, err := guard.service.binder.Bind(ctx, tool.Principal{UserID: guard.actor.ID}, binding.HandleID)
	if err != nil {
		return err
	}
	if live.Generation != guard.snapshot.Generation || live.AccessID != guard.snapshot.AccessID || live.ApplicationID != guard.snapshot.ApplicationID || live.Protocol != guard.snapshot.Protocol || live.AccessHandleID != guard.snapshot.AccessHandleID {
		return assistance.ErrClosed
	}
	organizationID, err := guard.service.resourceOrganization(live.AccessID)
	if err != nil {
		return err
	}
	if organizationID != guard.organizationID {
		return assistance.ErrClosed
	}
	return guard.service.authorizer.RequireOrganizationResourcePermission(&guard.actor, organizationID, "agent_sessions", "use")
}
