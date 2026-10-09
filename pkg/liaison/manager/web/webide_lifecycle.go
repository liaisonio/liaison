package web

import (
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"
)

// A bounded set of cancellable operation gates orders launch against stop,
// disable and delete for the same access. No registry mutex spans device RPCs.
func (g *webIDEGateway) acquireAccess(ctx context.Context, access string) (func(), error) {
	g.gateOnce.Do(func() {
		for i := range g.gates {
			g.gates[i] = make(chan struct{}, 1)
		}
	})
	hash := sha256.Sum256([]byte(access))
	gate := g.gates[int(hash[0])%len(g.gates)]
	var done <-chan struct{}
	if g.ctx != nil {
		done = g.ctx.Done()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-done:
		return nil, errors.New("IDE gateway closed")
	case gate <- struct{}{}:
		if err := ctx.Err(); err != nil {
			<-gate
			return nil, err
		}
		if g.ctx != nil && g.ctx.Err() != nil {
			<-gate
			return nil, errors.New("IDE gateway closed")
		}
		return func() { <-gate }, nil
	}
}

func (g *webIDEGateway) accessOperation(ctx context.Context, access string, operation func() error) error {
	release, err := g.acquireAccess(ctx, access)
	if err != nil {
		return err
	}
	defer release()
	return operation()
}

func (e *ideEndpoint) close() error {
	e.closeOnce.Do(func() {
		e.mu.Lock()
		e.closed = true
		clear(e.grants)
		e.mu.Unlock()
		if e.cancel != nil {
			e.cancel() // Also tears down hijacked WebSockets through their streams.
		}
		var errs []error
		if e.listener != nil {
			if err := e.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				errs = append(errs, err)
			}
		}
		if e.server != nil {
			if err := e.server.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				errs = append(errs, err)
			}
		}
		e.closeErr = errors.Join(errs...)
	})
	return e.closeErr
}

// releaseEndpoints is called only after the control plane has authorized and
// completed the operation. An empty instance revokes every endpoint of an access.
func (g *webIDEGateway) releaseEndpoints(access, instance string) error {
	g.mu.Lock()
	var entries []*ideEndpoint
	for key, e := range g.endpoints {
		if e.access == access && (instance == "" || e.instance == instance) {
			e.mu.Lock()
			e.closed = true
			clear(e.grants)
			e.mu.Unlock()
			delete(g.endpoints, key)
			entries = append(entries, e)
		}
	}
	g.mu.Unlock()
	var errs []error
	for _, e := range entries {
		errs = append(errs, e.close())
	}
	return errors.Join(errs...)
}

func (g *webIDEGateway) serveEndpoint(key string, entry *ideEndpoint) {
	defer g.workers.Done()
	defer func() {
		if recover() != nil {
			slog.Error("WebIDE listener panic")
		}
		g.mu.Lock()
		// A retired listener must never remove a replacement using the same key.
		if g.endpoints[key] == entry {
			delete(g.endpoints, key)
		}
		g.mu.Unlock()
		if err := entry.close(); err != nil {
			slog.Error("close WebIDE endpoint", "error", err)
		}
	}()
	if err := entry.server.Serve(entry.listener); err != nil && !errors.Is(err, http.ErrServerClosed) && !errors.Is(err, net.ErrClosed) {
		slog.Error("WebIDE listener stopped", "error", err)
	}
}

// Expiring a browser grant releases only its gateway endpoint, never project
// data, settings, extensions or the device process. Reopen issues a fresh grant.
func (g *webIDEGateway) reapExpired(now time.Time) error {
	g.mu.Lock()
	var expired []*ideEndpoint
	for key, entry := range g.endpoints {
		entry.mu.Lock()
		for k, grant := range entry.grants {
			if !now.Before(grant.expires) {
				delete(entry.grants, k)
			}
		}
		if len(entry.grants) == 0 && entry.pendingHandoffs == 0 {
			entry.closed = true
			delete(g.endpoints, key)
			expired = append(expired, entry)
		}
		entry.mu.Unlock()
	}
	g.mu.Unlock()
	var errs []error
	for _, entry := range expired {
		errs = append(errs, entry.close())
	}
	return errors.Join(errs...)
}

func (g *webIDEGateway) reapLoop() {
	defer g.workers.Done()
	defer func() {
		if recover() != nil {
			slog.Error("WebIDE endpoint collector panic")
		}
	}()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-g.ctx.Done():
			return
		case now := <-ticker.C:
			if err := g.reapExpired(now); err != nil {
				slog.Error("collect expired WebIDE endpoints", "error", err)
			}
		}
	}
}
