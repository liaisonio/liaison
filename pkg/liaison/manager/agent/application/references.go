package application

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

var ErrReferenceUnavailable = errors.New("referenced resource unavailable")

type ResourceReferenceResolver interface {
	ResolveAgentResource(context.Context, uint, string, uint64) (string, error)
}

func WithResourceReferences(resolver ResourceReferenceResolver) Option {
	return func(s *Service) { s.references = resolver }
}

func (s *Service) resolveReferences(ctx context.Context, actor *model.User, session runtime.Session, refs []runtime.ResourceReference) ([]runtime.ResourceReference, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	if len(refs) > 8 || session.Kind != tool.SessionManagement {
		return nil, ErrInvalid
	}
	if s.references == nil {
		return nil, ErrUnavailable
	}
	result := make([]runtime.ResourceReference, 0, len(refs))
	seen := make(map[string]bool)
	for _, ref := range refs {
		if ref.Type != "connector" && ref.Type != "device" && ref.Type != "application" {
			return nil, ErrInvalid
		}
		id, err := strconv.ParseUint(ref.ID, 10, 64)
		if err != nil || id == 0 || strconv.FormatUint(id, 10) != ref.ID {
			return nil, ErrInvalid
		}
		if err := s.authorizer.RequireOrganizationResourcePermission(actor, session.OrganizationID, ref.Type+"s", "read"); err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNotFound, ErrReferenceUnavailable)
		}
		name, err := s.references.ResolveAgentResource(ctx, actor.ID, ref.Type, id)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", ErrNotFound, ErrReferenceUnavailable)
		}
		key := ref.Type + ":" + ref.ID
		if !seen[key] {
			result = append(result, runtime.ResourceReference{Type: ref.Type, ID: ref.ID, Name: name})
			seen[key] = true
		}
	}
	return result, nil
}
