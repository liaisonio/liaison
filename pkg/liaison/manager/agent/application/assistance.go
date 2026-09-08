package application

import (
	"context"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// NewAssistanceSession 不创建 Agent Session，也不调用聊天 Store。
// 返回的实例只能交给所属用户的辅助会话容器，连接断开时必须 Close。
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
	return assistance.NewSession(assistance.Binding{OwnerID: owner.ID, HandleID: snapshot.AccessHandleID, Protocol: protocol}, generator, &assistanceGuard{
		service: service, actor: owner, snapshot: snapshot, organizationID: organizationID,
	})
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
