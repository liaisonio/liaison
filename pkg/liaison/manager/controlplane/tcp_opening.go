package controlplane

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sync"
	"time"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type tcpOpeningManager interface {
	GrantTCPOpening(context.Context, int, string, int64, time.Duration) (tcpwindow.Lease, error)
	GetTCPOpening(context.Context, int, string, int64) (tcpwindow.Lease, error)
	RevokeTCPOpening(context.Context, int, string, int64) error
}

type TCPOpeningData struct {
	RequestID  string `json:"request_id"`
	State      string `json:"state"`
	ServerTime string `json:"server_time"`
	ExpiresAt  string `json:"expires_at,omitempty"`
}

func normalizeTCPOpeningPolicy(p *v1.TCPOpeningPolicy, protocol model.AccessProtocol) (*v1.TCPOpeningPolicy, error) {
	mode, seconds := "continuous", int32(10)
	if p != nil {
		if p.Mode != "" {
			mode = p.Mode
		}
		if p.DurationSeconds != 0 {
			seconds = p.DurationSeconds
		}
	}
	if mode != "continuous" && mode != "on_demand" || seconds < 5 || seconds > 300 ||
		mode == "on_demand" && protocol != model.AccessProtocolTCP {
		return nil, badRequest("TCP_OPENING_POLICY_INVALID", "按需开放仅支持 TCP，时长为 5～300 秒")
	}
	return &v1.TCPOpeningPolicy{Mode: mode, DurationSeconds: seconds}, nil
}

func proxyTCPOpeningPolicy(p *model.Proxy) *v1.TCPOpeningPolicy {
	mode, seconds := p.TCPOpeningMode, p.TCPOpeningDurationSeconds
	if mode == "" {
		mode = "continuous"
	}
	if seconds == 0 {
		seconds = 10
	}
	return &v1.TCPOpeningPolicy{Mode: mode, DurationSeconds: int32(seconds)}
}

var openingRequestID = regexp.MustCompile(`^[a-zA-Z0-9_-]{16,128}$`)

// Authentication is required even for in-process callers. Resource scope is
// checked again here; transport-level "accesses/use" permission is not enough.
func (cp *controlPlane) TCPOpening(ctx context.Context, proxyID uint, requestID, operation string) (*TCPOpeningData, error) {
	owner, ok := actorUserID(ctx)
	if !ok {
		return nil, &HTTPError{status: 401, reason: "UNAUTHORIZED", message: "请先登录"}
	}
	if !openingRequestID.MatchString(requestID) || proxyID == 0 ||
		(operation != "open" && operation != "get" && operation != "revoke") {
		return nil, badRequest("TCP_OPENING_INVALID", "开放请求无效")
	}
	cp.proxyOperations[proxyID%64].Lock()
	defer cp.proxyOperations[proxyID%64].Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p, err := cp.getProxyForFirewall(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	if p.Status != model.ProxyStatusRunning || p.AccessProtocol != model.AccessProtocolTCP || p.TCPOpeningMode != "on_demand" {
		return nil, badRequest("TCP_OPENING_NOT_AVAILABLE", "此访问未启用按需开放")
	}
	runtime, ok := cp.proxyManager.(tcpOpeningManager)
	if !ok {
		return nil, errors.New("TCP opening runtime unavailable")
	}
	if operation == "open" && !cp.openingLimiter.allow(owner, time.Now()) {
		return nil, &HTTPError{status: 429, reason: "TCP_OPENING_RATE_LIMIT", message: "操作过于频繁，请稍后重试"}
	}
	resource := fmt.Sprintf("access/%d/tcp_openings/%s", proxyID, requestID)
	if operation != "get" {
		if err := cp.RecordManagementAudit(ctx, &ManagementAudit{UserID: owner, Module: "accesses", Action: "tcp_opening_" + operation + "_requested", Resource: resource, StatusCode: 202}); err != nil {
			return nil, fmt.Errorf("record TCP opening intent: %w", err)
		}
	}
	var lease tcpwindow.Lease
	switch operation {
	case "open":
		lease, err = runtime.GrantTCPOpening(ctx, int(proxyID), requestID, int64(owner), time.Duration(proxyTCPOpeningPolicy(p).DurationSeconds)*time.Second)
	case "get":
		lease, err = runtime.GetTCPOpening(ctx, int(proxyID), requestID, int64(owner))
	case "revoke":
		err = runtime.RevokeTCPOpening(ctx, int(proxyID), requestID, int64(owner))
	}
	closed := errors.Is(err, tcpwindow.ErrExpired) || errors.Is(err, tcpwindow.ErrNotFound) || errors.Is(err, tcpwindow.ErrClosed)
	if operation != "get" {
		auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		auditErr := cp.RecordManagementAudit(auditCtx, &ManagementAudit{UserID: owner, Module: "accesses", Action: "tcp_opening_" + operation + "_result", Resource: resource, Success: err == nil, StatusCode: func() int {
			if err == nil {
				return 200
			}
			return 503
		}()})
		cancel()
		// Do not report confirmed success if the audit result could not be saved.
		// Clients retain the request ID and query the actual runtime state.
		if auditErr != nil {
			return nil, fmt.Errorf("record TCP opening result: %w", auditErr)
		}
	}
	if err != nil && !closed {
		return nil, err
	}
	now := time.Now()
	result := &TCPOpeningData{RequestID: requestID, State: "closed", ServerTime: now.UTC().Format(time.RFC3339Nano)}
	if err == nil && operation != "revoke" && now.Before(lease.ExpiresAt) {
		result.State = "open"
		result.ExpiresAt = lease.ExpiresAt.UTC().Format(time.RFC3339Nano)
	}
	return result, nil
}

type tcpOpeningRateLimit struct {
	mu      sync.Mutex
	buckets map[uint]openingBucket
}
type openingBucket struct {
	since time.Time
	count int
}

func (l *tcpOpeningRateLimit) allow(owner uint, now time.Time) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.buckets == nil {
		l.buckets = make(map[uint]openingBucket)
	}
	for key, b := range l.buckets {
		if now.Sub(b.since) >= time.Minute {
			delete(l.buckets, key)
		}
	}
	b, exists := l.buckets[owner]
	if !exists {
		if len(l.buckets) >= 4096 {
			return false
		}
		b.since = now
	}
	if b.count >= 30 {
		return false
	}
	b.count++
	l.buckets[owner] = b
	return true
}
