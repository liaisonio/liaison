// Package tcpwindow 管理一个访问路由代际的临时准入授权，不执行 IO 或身份认证。
// 调用方必须先校验资源权限；Admit 的结果仍须与静态 ACL 取交集。
package tcpwindow

import (
	"errors"
	"sync"
	"time"
)

const (
	DefaultDuration = 10 * time.Second
	MinDuration     = 5 * time.Second
	MaxDuration     = 300 * time.Second
	ReplayRetention = 10 * time.Minute
	DefaultCapacity = 256
)

var (
	ErrInvalid  = errors.New("invalid temporary TCP opening")
	ErrConflict = errors.New("temporary TCP opening request conflicts")
	ErrExpired  = errors.New("temporary TCP opening expired or revoked")
	ErrCapacity = errors.New("temporary TCP opening capacity reached")
	ErrClosed   = errors.New("temporary TCP opening registry closed")
	ErrNotFound = errors.New("temporary TCP opening not found")
)

// Lease 是只读快照；ID 是幂等标识，不是 bearer credential。
type Lease struct {
	ID        string
	OwnerID   int64
	OpenedAt  time.Time
	ExpiresAt time.Time
}

type record struct {
	lease    Lease
	duration time.Duration
	revoked  bool
}

// Registry 不可复制。一个实例只服务一个访问的一个路由代际。
// 不存储全局访问 ID，从结构上避免不同访问的同名请求共享授权。
type Registry struct {
	mu       sync.Mutex
	items    map[string]record
	capacity int
	closed   bool
}

func New(capacity int) (*Registry, error) {
	if capacity < 1 || capacity > DefaultCapacity {
		return nil, ErrInvalid
	}
	return &Registry{items: make(map[string]record), capacity: capacity}, nil
}

func validID(id string) bool {
	if len(id) < 16 || len(id) > 128 {
		return false
	}
	for _, c := range id {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

// Grant 的 now 必须由服务端时钟给出，不接受来自客户端的时间。
// 相同标识、操作者和时长重试不会延长原窗口。
func (r *Registry) Grant(now time.Time, id string, ownerID int64, duration time.Duration) (Lease, error) {
	if now.IsZero() || !validID(id) || ownerID <= 0 || duration < MinDuration || duration > MaxDuration || duration%time.Second != 0 {
		return Lease{}, ErrInvalid
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Lease{}, ErrClosed
	}
	for key, item := range r.items {
		if !now.Before(item.lease.ExpiresAt.Add(ReplayRetention)) {
			delete(r.items, key)
		}
	}
	if item, ok := r.items[id]; ok {
		if item.lease.OwnerID != ownerID || item.duration != duration {
			return Lease{}, ErrConflict
		}
		if item.revoked || !now.Before(item.lease.ExpiresAt) {
			return Lease{}, ErrExpired
		}
		return item.lease, nil
	}
	if len(r.items) >= r.capacity {
		return Lease{}, ErrCapacity
	}
	lease := Lease{ID: id, OwnerID: ownerID, OpenedAt: now, ExpiresAt: now.Add(duration)}
	r.items[id] = record{lease: lease, duration: duration}
	return lease, nil
}

// Admit 在准入点重新校验截止时间；不依赖清理任务，恰好到期即拒绝。
// 多授权匹配时稳定选择最晚截止的授权，供连接审计关联。
func (r *Registry) Admit(now time.Time) (Lease, bool) {
	if now.IsZero() {
		return Lease{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Lease{}, false
	}
	var selected Lease
	for _, item := range r.items {
		lease := item.lease
		if item.revoked || now.Before(lease.OpenedAt) || !now.Before(lease.ExpiresAt) {
			continue
		}
		if selected.ID == "" || lease.ExpiresAt.After(selected.ExpiresAt) || lease.ExpiresAt.Equal(selected.ExpiresAt) && lease.ID < selected.ID {
			selected = lease
		}
	}
	return selected, selected.ID != ""
}

// Get 只返回操作者自己的授权；过期或撤销不伪装成仍开放。
func (r *Registry) Get(now time.Time, id string, ownerID int64) (Lease, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return Lease{}, ErrClosed
	}
	item, ok := r.items[id]
	if !ok || ownerID <= 0 || item.lease.OwnerID != ownerID {
		return Lease{}, ErrNotFound
	}
	if now.IsZero() || now.Before(item.lease.OpenedAt) || item.revoked || !now.Before(item.lease.ExpiresAt) {
		return Lease{}, ErrExpired
	}
	return item.lease, nil
}

// Revoke 幂等撤销本人的授权，不影响其他授权或已准入的连接。
func (r *Registry) Revoke(id string, ownerID int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return ErrClosed
	}
	item, ok := r.items[id]
	if !ok || ownerID <= 0 || item.lease.OwnerID != ownerID {
		return ErrNotFound
	}
	item.revoked = true
	r.items[id] = item
	return nil
}

// NextExpiry 返回下一次需复核的截止时间；调度者届时必须重新检查，不能盲关新 listener。
func (r *Registry) NextExpiry(now time.Time) (time.Time, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed || now.IsZero() {
		return time.Time{}, false
	}
	var next time.Time
	for _, item := range r.items {
		if item.revoked || now.Before(item.lease.OpenedAt) || !now.Before(item.lease.ExpiresAt) {
			continue
		}
		if next.IsZero() || item.lease.ExpiresAt.Before(next) {
			next = item.lease.ExpiresAt
		}
	}
	return next, !next.IsZero()
}

// Close 销毁整个路由代际；旧回调无法再次授予访问。
func (r *Registry) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.closed = true
	clear(r.items)
}
