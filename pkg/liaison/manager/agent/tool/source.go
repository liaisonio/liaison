package tool

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type SourceEventKind uint8

const (
	SourceEventUpsert SourceEventKind = iota
	SourceEventEnable
	SourceEventDisable
	SourceEventUnload
)

type ToolSourceEvent struct {
	Kind         SourceEventKind
	Registration ToolRegistration
	ToolID       ToolID
	Force        bool
}

type ToolSource interface {
	ID() string
	TrustLevel() TrustLevel
	Snapshot(ctx context.Context) ([]ToolRegistration, error)
	Watch(ctx context.Context) (<-chan ToolSourceEvent, error)
}

type SourceManager struct {
	engine *Engine
	mu     sync.Mutex
	tools  map[string]map[ToolID]struct{}
}

func NewSourceManager(engine *Engine) *SourceManager {
	return &SourceManager{engine: engine, tools: make(map[string]map[ToolID]struct{})}
}

func (manager *SourceManager) Load(ctx context.Context, source ToolSource) error {
	if source == nil {
		return errors.New("tool source is required")
	}
	sourceID := source.ID()
	if sourceID == "" {
		return errors.New("tool source ID is required")
	}
	registrations, err := source.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("read tool source %q snapshot: %w", sourceID, err)
	}
	if err := validateSourceRegistrations(sourceID, source.TrustLevel(), registrations); err != nil {
		return err
	}

	manager.mu.Lock()
	defer manager.mu.Unlock()
	if _, loaded := manager.tools[sourceID]; loaded {
		return fmt.Errorf("tool source %q is already loaded", sourceID)
	}
	loaded := make(map[ToolID]struct{}, len(registrations))
	for _, registration := range registrations {
		if err := manager.engine.Register(ctx, registration); err != nil {
			for id := range loaded {
				if _, unloadErr := manager.engine.Unload(ctx, id, false); unloadErr != nil {
					return fmt.Errorf("register %s: %w; rollback %s: %v", registration.Descriptor.ID.String(), err, id.String(), unloadErr)
				}
			}
			return fmt.Errorf("register source %q tool %s: %w", sourceID, registration.Descriptor.ID.String(), err)
		}
		loaded[registration.Descriptor.ID] = struct{}{}
	}
	manager.tools[sourceID] = loaded
	return nil
}

// Run applies a source's event stream in the caller's goroutine. Its owner is
// responsible for lifecycle and cancellation, which avoids hidden goroutines.
func (manager *SourceManager) Run(ctx context.Context, source ToolSource) error {
	if err := manager.Load(ctx, source); err != nil {
		return err
	}
	events, err := source.Watch(ctx)
	if err != nil {
		return fmt.Errorf("watch tool source %q: %w", source.ID(), err)
	}
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-events:
			if !ok {
				return nil
			}
			if err := manager.Apply(ctx, source, event); err != nil {
				return err
			}
		}
	}
}

func (manager *SourceManager) Apply(ctx context.Context, source ToolSource, event ToolSourceEvent) error {
	if source == nil || source.ID() == "" {
		return errors.New("tool source is required")
	}
	manager.mu.Lock()
	defer manager.mu.Unlock()
	loaded, ok := manager.tools[source.ID()]
	if !ok {
		return fmt.Errorf("tool source %q is not loaded", source.ID())
	}
	switch event.Kind {
	case SourceEventUpsert:
		if err := validateSourceRegistrations(source.ID(), source.TrustLevel(), []ToolRegistration{event.Registration}); err != nil {
			return err
		}
		if err := manager.engine.Replace(ctx, event.Registration); err != nil {
			return fmt.Errorf("replace source %q tool: %w", source.ID(), err)
		}
		loaded[event.Registration.Descriptor.ID] = struct{}{}
	case SourceEventEnable:
		if _, exists := loaded[event.ToolID]; !exists {
			return fmt.Errorf("source %q does not own %s", source.ID(), event.ToolID.String())
		}
		if err := manager.engine.Enable(ctx, event.ToolID); err != nil {
			return err
		}
	case SourceEventDisable:
		if _, exists := loaded[event.ToolID]; !exists {
			return fmt.Errorf("source %q does not own %s", source.ID(), event.ToolID.String())
		}
		if err := manager.engine.Disable(ctx, event.ToolID); err != nil {
			return err
		}
	case SourceEventUnload:
		if _, exists := loaded[event.ToolID]; !exists {
			return fmt.Errorf("source %q does not own %s", source.ID(), event.ToolID.String())
		}
		complete, err := manager.engine.Unload(ctx, event.ToolID, event.Force)
		if err != nil {
			return err
		}
		if complete {
			delete(loaded, event.ToolID)
		}
	default:
		return fmt.Errorf("unknown source event kind %d", event.Kind)
	}
	return nil
}

func (manager *SourceManager) Unload(ctx context.Context, sourceID string, force bool) (bool, error) {
	manager.mu.Lock()
	defer manager.mu.Unlock()
	loaded, ok := manager.tools[sourceID]
	if !ok {
		return false, fmt.Errorf("tool source %q is not loaded", sourceID)
	}
	complete := true
	for id := range loaded {
		unloaded, err := manager.engine.Unload(ctx, id, force)
		if err != nil && !errors.Is(err, ErrToolNotFound) {
			return false, fmt.Errorf("unload source %q tool %s: %w", sourceID, id.String(), err)
		}
		if unloaded || errors.Is(err, ErrToolNotFound) {
			delete(loaded, id)
			continue
		}
		complete = false
	}
	if len(loaded) == 0 {
		delete(manager.tools, sourceID)
		return true, nil
	}
	return complete, nil
}

func validateSourceRegistrations(sourceID string, trust TrustLevel, registrations []ToolRegistration) error {
	seen := make(map[ToolID]struct{}, len(registrations))
	for _, registration := range registrations {
		if registration.Descriptor.Source.ID != sourceID {
			return fmt.Errorf("tool %s declares source %q, expected %q", registration.Descriptor.ID.String(), registration.Descriptor.Source.ID, sourceID)
		}
		if registration.Descriptor.Source.Trust != trust {
			return fmt.Errorf("tool %s trust does not match source", registration.Descriptor.ID.String())
		}
		if _, duplicate := seen[registration.Descriptor.ID]; duplicate {
			return fmt.Errorf("source %q contains duplicate tool %s", sourceID, registration.Descriptor.ID.String())
		}
		seen[registration.Descriptor.ID] = struct{}{}
		if registration.Factory == nil {
			return fmt.Errorf("tool %s has no factory", registration.Descriptor.ID.String())
		}
		if err := ValidateDescriptor(registration.Descriptor); err != nil {
			return err
		}
	}
	return nil
}
