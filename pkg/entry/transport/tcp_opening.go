package transport

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
)

// tcpOpeningListener 只关闭新连接入口，不能取消已准入连接的路由 context。
type tcpOpeningListener interface {
	CloseListener() error
}

type tcpOpeningRequest struct {
	ctx      context.Context
	id       string
	owner    int64
	duration time.Duration
	revoke   bool
	result   chan tcpOpeningResult
}

type tcpOpeningResult struct {
	lease tcpwindow.Lease
	err   error
}

// tcpOpeningRuntime 属于一个路由代际。唯一的工作协程串行操作 listener，
// 不持 Gatekeeper 锁进行 IO；到期关闭不会取消调用方拥有的路由 context。
type tcpOpeningRuntime struct {
	registry *tcpwindow.Registry
	requests chan tcpOpeningRequest
	done     chan struct{}
}

func newTCPOpeningRuntime(ctx context.Context, open func(context.Context) (tcpOpeningListener, error)) (*tcpOpeningRuntime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if open == nil {
		return nil, tcpwindow.ErrInvalid
	}
	registry, err := tcpwindow.New(tcpwindow.DefaultCapacity)
	if err != nil {
		return nil, err
	}
	r := &tcpOpeningRuntime{registry: registry, requests: make(chan tcpOpeningRequest), done: make(chan struct{})}
	go r.run(ctx, open)
	return r, nil
}

func (r *tcpOpeningRuntime) grant(ctx context.Context, id string, owner int64, duration time.Duration) (tcpwindow.Lease, error) {
	return r.request(tcpOpeningRequest{ctx: ctx, id: id, owner: owner, duration: duration})
}

func (r *tcpOpeningRuntime) revoke(ctx context.Context, id string, owner int64) error {
	_, err := r.request(tcpOpeningRequest{ctx: ctx, id: id, owner: owner, revoke: true})
	return err
}

func (r *tcpOpeningRuntime) request(req tcpOpeningRequest) (tcpwindow.Lease, error) {
	if err := req.ctx.Err(); err != nil {
		return tcpwindow.Lease{}, err
	}
	// 调用方取消后，工作协程仍可提交结果，不阻塞到期处理。
	req.result = make(chan tcpOpeningResult, 1)
	select {
	case <-req.ctx.Done():
		return tcpwindow.Lease{}, req.ctx.Err()
	case <-r.done:
		return tcpwindow.Lease{}, tcpwindow.ErrClosed
	case r.requests <- req:
	}
	select {
	case result := <-req.result:
		return result.lease, result.err
	case <-req.ctx.Done():
		// 结果可能已经生效；调用方必须使用相同 ID 查询/重试，不能自动续期。
		return tcpwindow.Lease{}, req.ctx.Err()
	case <-r.done:
		return tcpwindow.Lease{}, tcpwindow.ErrClosed
	}
}

func closeTCPOpeningListener(listener tcpOpeningListener) error {
	if listener == nil {
		return nil
	}
	if err := listener.CloseListener(); err != nil {
		return fmt.Errorf("close temporary TCP listener: %w", err)
	}
	return nil
}

func (r *tcpOpeningRuntime) run(ctx context.Context, open func(context.Context) (tcpOpeningListener, error)) {
	var listener tcpOpeningListener
	defer func() {
		r.registry.Close()
		if err := closeTCPOpeningListener(listener); err != nil {
			slog.ErrorContext(ctx, "temporary TCP listener cleanup failed", "error", err)
		}
		if recovered := recover(); recovered != nil {
			slog.ErrorContext(ctx, "temporary TCP lifecycle panic", "panic", recovered)
		}
		close(r.done)
	}()
	for {
		// 每轮重新计算，旧 timer 不持有 listener 引用，也不能关闭新代际。
		var timer *time.Timer
		var expiry <-chan time.Time
		var closingErr error
		if deadline, active := r.registry.NextExpiry(time.Now()); active {
			timer = time.NewTimer(time.Until(deadline))
		} else if listener != nil {
			if err := closeTCPOpeningListener(listener); err != nil {
				closingErr = err
				slog.ErrorContext(ctx, "temporary TCP listener close failed; admission denied", "error", err)
				// 保留句柄并重试；窗口校验独立拒绝新连接，不假装关闭成功。
				timer = time.NewTimer(100 * time.Millisecond)
			} else {
				listener = nil
			}
		}
		if timer != nil {
			expiry = timer.C
		}
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case <-expiry:
			continue
		case req := <-r.requests:
			if timer != nil {
				timer.Stop()
			}
			result := tcpOpeningResult{}
			if err := req.ctx.Err(); err != nil {
				result.err = err
			} else if req.revoke {
				result.err = r.registry.Revoke(req.id, req.owner)
				if _, active := r.registry.NextExpiry(time.Now()); result.err == nil && !active {
					result.err = closeTCPOpeningListener(listener)
					if result.err == nil {
						listener = nil
					}
				}
			} else if closingErr != nil {
				result.err = closingErr
			} else {
				result = r.applyGrant(ctx, req, open, &listener)
			}
			req.result <- result
		}
	}
}

func (r *tcpOpeningRuntime) applyGrant(ctx context.Context, req tcpOpeningRequest, open func(context.Context) (tcpOpeningListener, error), listener *tcpOpeningListener) tcpOpeningResult {
	// 先在临时状态中验证参数，避免无效请求触发绑定端口。
	validation, err := tcpwindow.New(1)
	if err != nil {
		return tcpOpeningResult{err: err}
	}
	if _, err := validation.Grant(time.Now(), req.id, req.owner, req.duration); err != nil {
		return tcpOpeningResult{err: err}
	}
	if _, err := r.registry.Get(time.Now(), req.id, req.owner); err != nil && !errors.Is(err, tcpwindow.ErrNotFound) {
		return tcpOpeningResult{err: err}
	}
	if *listener == nil {
		// 使用路由生命周期，而不是 HTTP 请求生命周期；响应断开不能杀死会话。
		opened, err := open(ctx)
		if err != nil {
			return tcpOpeningResult{err: fmt.Errorf("open temporary TCP listener: %w", err)}
		}
		if opened == nil {
			return tcpOpeningResult{err: fmt.Errorf("open temporary TCP listener: %w", tcpwindow.ErrInvalid)}
		}
		*listener = opened
	}
	if err := ctx.Err(); err != nil {
		return tcpOpeningResult{err: err}
	}
	if err := req.ctx.Err(); err != nil {
		return tcpOpeningResult{err: err}
	}
	// Listen 完成后才确定窗口时间；绑定期间没有授权，准入仍拒绝。
	lease, err := r.registry.Grant(time.Now(), req.id, req.owner, req.duration)
	return tcpOpeningResult{lease: lease, err: err}
}
