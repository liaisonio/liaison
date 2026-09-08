package tool

import (
	"context"
	"fmt"
	"sync"
)

type managedRegistration struct {
	registration ToolRegistration
	generation   uint64
	status       ToolStatus
	inFlight     map[uint64]context.CancelFunc
	nextLeaseID  uint64
}

type Registry struct {
	mu            sync.Mutex
	generation    uint64
	registrations map[ToolID]*managedRegistration
}

func NewRegistry() *Registry {
	return &Registry{registrations: make(map[ToolID]*managedRegistration)}
}

func (registry *Registry) register(registration ToolRegistration, replace bool) (uint64, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if existing, ok := registry.registrations[registration.Descriptor.ID]; ok && !replace && existing.status != ToolStatusUnloaded {
		return 0, fmt.Errorf("%w: %s", ErrAlreadyRegistered, registration.Descriptor.ID.String())
	}

	registry.generation++
	registry.registrations[registration.Descriptor.ID] = &managedRegistration{
		registration: cloneRegistration(registration),
		generation:   registry.generation,
		status:       ToolStatusActive,
		inFlight:     make(map[uint64]context.CancelFunc),
	}
	return registry.generation, nil
}

func (registry *Registry) setStatus(id ToolID, status ToolStatus) (uint64, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	entry, ok := registry.registrations[id]
	if !ok || entry.status == ToolStatusUnloaded {
		return 0, fmt.Errorf("%w: %s", ErrToolNotFound, id.String())
	}
	if !validStatusTransition(entry.status, status) {
		return 0, fmt.Errorf("%w: cannot transition %s from %s to %s", ErrToolUnavailable, id.String(), entry.status.String(), status.String())
	}
	registry.generation++
	entry.status = status
	return registry.generation, nil
}

func validStatusTransition(from, to ToolStatus) bool {
	if from == to {
		return true
	}
	switch from {
	case ToolStatusActive:
		return to == ToolStatusDisabled || to == ToolStatusDraining || to == ToolStatusFailed
	case ToolStatusDisabled:
		return to == ToolStatusActive || to == ToolStatusDraining || to == ToolStatusFailed
	case ToolStatusFailed:
		return to == ToolStatusActive || to == ToolStatusDisabled || to == ToolStatusDraining
	case ToolStatusDraining, ToolStatusUnloaded:
		return false
	default:
		return false
	}
}

func (registry *Registry) unload(id ToolID, force bool) (uint64, bool, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	entry, ok := registry.registrations[id]
	if !ok || entry.status == ToolStatusUnloaded {
		return 0, false, fmt.Errorf("%w: %s", ErrToolNotFound, id.String())
	}
	registry.generation++
	entry.status = ToolStatusDraining
	if force {
		for _, cancel := range entry.inFlight {
			cancel()
		}
	}
	if len(entry.inFlight) == 0 {
		entry.status = ToolStatusUnloaded
		delete(registry.registrations, id)
		return registry.generation, true, nil
	}
	return registry.generation, false, nil
}

func (registry *Registry) acquire(ctx context.Context, id ToolID, registrationGeneration uint64) (*ToolLease, error) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	entry, ok := registry.registrations[id]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrToolNotFound, id.String())
	}
	if entry.status != ToolStatusActive {
		return nil, fmt.Errorf("%w: %s is %s", ErrToolUnavailable, id.String(), entry.status.String())
	}
	if entry.generation != registrationGeneration {
		return nil, fmt.Errorf("%w: %s generation %d is now %d", ErrStaleSnapshot, id.String(), registrationGeneration, entry.generation)
	}
	entry.nextLeaseID++
	leaseID := entry.nextLeaseID
	leaseContext, cancel := context.WithCancel(ctx)
	entry.inFlight[leaseID] = cancel
	return &ToolLease{
		registry:     registry,
		entry:        entry,
		leaseID:      leaseID,
		ctx:          leaseContext,
		registration: cloneRegistration(entry.registration),
	}, nil
}

func (registry *Registry) release(entry *managedRegistration, leaseID uint64) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	cancel, ok := entry.inFlight[leaseID]
	if !ok {
		return
	}
	cancel()
	delete(entry.inFlight, leaseID)
	if entry.status == ToolStatusDraining && len(entry.inFlight) == 0 {
		entry.status = ToolStatusUnloaded
		current, exists := registry.registrations[entry.registration.Descriptor.ID]
		if exists && current == entry {
			delete(registry.registrations, entry.registration.Descriptor.ID)
			registry.generation++
		}
	}
}

func (registry *Registry) status(id ToolID) (ToolStatus, uint64, int, bool) {
	registry.mu.Lock()
	defer registry.mu.Unlock()
	entry, ok := registry.registrations[id]
	if !ok {
		return ToolStatusUnloaded, 0, 0, false
	}
	return entry.status, entry.generation, len(entry.inFlight), true
}

type ToolLease struct {
	registry     *Registry
	entry        *managedRegistration
	leaseID      uint64
	ctx          context.Context
	registration ToolRegistration
	once         sync.Once
}

func (lease *ToolLease) Context() context.Context {
	return lease.ctx
}

func (lease *ToolLease) Registration() ToolRegistration {
	return cloneRegistration(lease.registration)
}

func (lease *ToolLease) Release() {
	lease.once.Do(func() {
		lease.registry.release(lease.entry, lease.leaseID)
	})
}

func cloneRegistration(registration ToolRegistration) ToolRegistration {
	return ToolRegistration{Descriptor: cloneDescriptor(registration.Descriptor), Factory: registration.Factory}
}

func (status ToolStatus) String() string {
	switch status {
	case ToolStatusActive:
		return "active"
	case ToolStatusDisabled:
		return "disabled"
	case ToolStatusDraining:
		return "draining"
	case ToolStatusFailed:
		return "failed"
	case ToolStatusUnloaded:
		return "unloaded"
	default:
		return "unknown"
	}
}
