package accesssession

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
)

func PublicReference(id string) string { return fmt.Sprintf("conn_%x", sha256.Sum256([]byte(id)))[:29] }

// DescribeReference resolves a public digest without exposing a bearer handle.
func (registry *Registry) DescribeReference(ctx context.Context, reference string, userID uint) (Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return Descriptor{}, err
	}
	if registry == nil || userID == 0 {
		return Descriptor{}, ErrHandleNotFound
	}
	registry.mu.RLock()
	defer registry.mu.RUnlock()
	for _, handle := range registry.handles {
		if handle.Descriptor.UserID == userID && PublicReference(handle.Descriptor.ID) == reference {
			d := cloneDescriptor(handle.Descriptor)
			d.ID = ""
			return d, nil
		}
	}
	return Descriptor{}, ErrHandleNotFound
}

var (
	ErrHandleNotFound = errors.New("access session handle not found")
	ErrHandleMismatch = errors.New("access session handle does not match attachment")
)

type Protocol string

const (
	ProtocolWebSSH     Protocol = "web_ssh"
	ProtocolMySQL      Protocol = "mysql"
	ProtocolPostgreSQL Protocol = "postgresql"
	ProtocolRedis      Protocol = "redis"
	ProtocolMongoDB    Protocol = "mongodb"
	ProtocolRDP        Protocol = "rdp"
	ProtocolVNC        Protocol = "vnc"
)

type Descriptor struct {
	ID            string
	UserID        uint
	AccessID      uint
	ApplicationID uint
	Protocol      Protocol
	Capabilities  []string
	Generation    uint64
}

type ResolveRequest struct {
	ID            string
	UserID        uint
	AccessID      uint
	ApplicationID uint
	Protocol      Protocol
	Generation    uint64
}

type Terminal interface {
	Read(ctx context.Context, maxLines int) (json.RawMessage, error)
	Execute(ctx context.Context, command string) (json.RawMessage, error)
}

type Data interface {
	Schema(ctx context.Context, path []string) (json.RawMessage, error)
	Query(ctx context.Context, statement string) (json.RawMessage, error)
}

type Desktop interface {
	SessionInfo(ctx context.Context) (json.RawMessage, error)
}

type Handle struct {
	Descriptor Descriptor
	Terminal   Terminal
	Data       Data
	Desktop    Desktop
}

type Registry struct {
	mu          sync.RWMutex
	handles     map[string]Handle
	generations map[string]uint64
}

func NewRegistry() *Registry {
	return &Registry{
		handles:     make(map[string]Handle),
		generations: make(map[string]uint64),
	}
}

// Register publishes a live protocol session. Reusing an ID advances its
// generation so a stale Agent attachment can never target a replacement
// connection accidentally.
func (registry *Registry) Register(handle Handle) (Descriptor, func(), error) {
	if registry == nil {
		return Descriptor{}, nil, errors.New("access session registry is required")
	}
	if handle.Descriptor.ID == "" || handle.Descriptor.UserID == 0 || handle.Descriptor.AccessID == 0 || handle.Descriptor.Protocol == "" {
		return Descriptor{}, nil, errors.New("access session descriptor is incomplete")
	}
	if handle.Terminal == nil && handle.Data == nil && handle.Desktop == nil {
		return Descriptor{}, nil, errors.New("access session requires a protocol handler")
	}

	registry.mu.Lock()
	generation := registry.generations[handle.Descriptor.ID] + 1
	registry.generations[handle.Descriptor.ID] = generation
	handle.Descriptor.Generation = generation
	handle.Descriptor = cloneDescriptor(handle.Descriptor)
	registry.handles[handle.Descriptor.ID] = handle
	registry.mu.Unlock()

	unregister := func() {
		registry.mu.Lock()
		defer registry.mu.Unlock()
		current, ok := registry.handles[handle.Descriptor.ID]
		if ok && current.Descriptor.Generation == generation {
			delete(registry.handles, handle.Descriptor.ID)
		}
	}
	return handle.Descriptor, unregister, nil
}

func (registry *Registry) Resolve(ctx context.Context, request ResolveRequest) (Handle, error) {
	if err := ctx.Err(); err != nil {
		return Handle{}, err
	}
	if registry == nil {
		return Handle{}, ErrHandleNotFound
	}
	registry.mu.RLock()
	handle, ok := registry.handles[request.ID]
	registry.mu.RUnlock()
	if !ok {
		return Handle{}, fmt.Errorf("%w: %s", ErrHandleNotFound, request.ID)
	}
	descriptor := handle.Descriptor
	if descriptor.UserID != request.UserID ||
		descriptor.AccessID != request.AccessID ||
		descriptor.ApplicationID != request.ApplicationID ||
		descriptor.Protocol != request.Protocol ||
		descriptor.Generation != request.Generation {
		return Handle{}, ErrHandleMismatch
	}
	handle.Descriptor = cloneDescriptor(handle.Descriptor)
	return handle, nil
}

// Describe returns server-authored attachment facts for a live handle. The
// caller supplies only the authenticated user and opaque handle ID; resource
// IDs, protocol and generation are never accepted from a model or browser.
func (registry *Registry) Describe(ctx context.Context, id string, userID uint) (Descriptor, error) {
	if err := ctx.Err(); err != nil {
		return Descriptor{}, err
	}
	if registry == nil {
		return Descriptor{}, ErrHandleNotFound
	}
	registry.mu.RLock()
	handle, ok := registry.handles[id]
	registry.mu.RUnlock()
	if !ok {
		return Descriptor{}, fmt.Errorf("%w: %s", ErrHandleNotFound, id)
	}
	if userID == 0 || handle.Descriptor.UserID != userID {
		return Descriptor{}, ErrHandleMismatch
	}
	return cloneDescriptor(handle.Descriptor), nil
}

func cloneDescriptor(descriptor Descriptor) Descriptor {
	descriptor.Capabilities = append([]string(nil), descriptor.Capabilities...)
	return descriptor
}
