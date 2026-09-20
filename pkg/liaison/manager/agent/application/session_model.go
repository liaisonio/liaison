package application

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// SetSessionModel persists only public identifiers under the existing owner and
// resource permissions. An empty selection means follow the system default.
func (s *Service) SetSessionModel(ctx context.Context, actor *model.User, id string, version uint64, selection runtime.ModelSelection) (runtime.Session, error) {
	session, err := s.ownedSession(ctx, actor, id, "use")
	if err != nil {
		return runtime.Session{}, err
	}
	if err := s.checkLiveAttachments(ctx, actor, session); err != nil {
		return runtime.Session{}, err
	}
	if len(selection.ProviderID) > 128 || len(selection.Model) > 200 || (selection.ProviderID == "") != (selection.Model == "") {
		return runtime.Session{}, ErrInvalid
	}
	if selection.ProviderID != "" {
		if s.models == nil {
			return runtime.Session{}, ErrUnavailable
		}
		if _, err := s.models.ResolveSelection(ctx, selection); err != nil {
			return runtime.Session{}, ErrInvalid
		}
	}
	store, ok := s.store.(interface {
		SetSessionModel(context.Context, string, uint64, runtime.ModelSelection) (runtime.Session, error)
	})
	if !ok {
		return runtime.Session{}, ErrUnavailable
	}
	return store.SetSessionModel(ctx, id, version, selection)
}
