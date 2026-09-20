package runtime

import "context"

func validateModelUpdate(session Session, version uint64) error {
	if session.Version != version {
		return ErrVersionConflict
	}
	if session.Status != SessionActive {
		return ErrSessionArchived
	}
	if session.ActiveTurnID != "" {
		return ErrTurnAlreadyActive
	}
	return nil
}

func (store *MemoryStore) SetSessionModel(ctx context.Context, id string, version uint64, selection ModelSelection) (Session, error) {
	if err := ctx.Err(); err != nil {
		return Session{}, err
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[id]
	if !ok {
		return Session{}, ErrSessionNotFound
	}
	if err := validateModelUpdate(session, version); err != nil {
		return Session{}, err
	}
	session.ModelSelection = selection
	session.Version++
	session.UpdatedAt = store.clock().UTC()
	store.sessions[id] = session
	return session, nil
}

func (store *DurableStore) SetSessionModel(ctx context.Context, id string, version uint64, selection ModelSelection) (Session, error) {
	session, err := store.GetSession(ctx, id)
	if err != nil {
		return Session{}, err
	}
	if err := validateModelUpdate(session, version); err != nil {
		return Session{}, err
	}
	session.ModelSelection = selection
	session.Version++
	session.UpdatedAt = store.clock().UTC()
	updated, err := store.dao.UpdateAgentSessionCAS(ctx, sessionModel(session), version)
	if err != nil {
		return Session{}, err
	}
	if !updated {
		return Session{}, ErrVersionConflict
	}
	return session, nil
}
