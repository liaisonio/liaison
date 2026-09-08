package application

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type organizationMemberships interface {
	ListUserOrganizations(uint) ([]*model.OrganizationMembership, error)
}

func (s *Service) createManagementSession(ctx context.Context, request CreateSessionRequest) (SessionDetail, error) {
	if request.Actor == nil || request.Actor.ID == 0 || request.Actor.Status != model.UserStatusActive || strings.TrimSpace(request.HandleID) != "" {
		return SessionDetail{}, fmt.Errorf("%w: active actor required; management sessions cannot bind a handle", ErrInvalid)
	}
	memberships, ok := s.relations.(organizationMemberships)
	if !ok {
		return SessionDetail{}, ErrUnavailable
	}
	items, err := memberships.ListUserOrganizations(request.Actor.ID)
	if err != nil {
		return SessionDetail{}, err
	}
	ids := []uint{}
	for _, item := range items {
		if item != nil && item.UserID == request.Actor.ID && item.OrganizationID != 0 {
			ids = append(ids, item.OrganizationID)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	// Stable server-derived domain. Membership does not grant visibility of
	// other members' resources; management executors still scope to the actor.
	if len(ids) == 0 {
		return SessionDetail{}, ErrNotFound
	}
	organizationID := ids[0]
	if err := s.authorizer.RequireOrganizationResourcePermission(request.Actor, organizationID, "management_agent_sessions", "create"); err != nil {
		return SessionDetail{}, err
	}
	id, err := s.ids.NewID("session")
	if err != nil {
		return SessionDetail{}, err
	}
	title := strings.TrimSpace(request.Title)
	if title == "" {
		title = "New management session"
	}
	if len([]rune(title)) > 255 {
		title = string([]rune(title)[:255])
	}
	session := runtime.Session{ID: id, Kind: tool.SessionManagement, CreatedBy: request.Actor.ID, OrganizationID: organizationID, Title: title, Status: runtime.SessionActive}
	if err := s.store.CreateSession(ctx, session); err != nil {
		return SessionDetail{}, err
	}
	session, err = s.store.GetSession(ctx, id)
	if err != nil {
		return SessionDetail{}, err
	}
	return SessionDetail{Session: session, Attachments: []runtime.Attachment{}}, nil
}
