package transport

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
	"github.com/stretchr/testify/require"
)

type openingTestListener struct {
	closes atomic.Int32
	err    error
}

func (l *openingTestListener) CloseListener() error {
	l.closes.Add(1)
	return l.err
}

func TestTCPOpening_IdempotentRevokeAndReopen(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var opens atomic.Int32
	first := &openingTestListener{}
	r, err := newTCPOpeningRuntime(ctx, func(context.Context) (tcpOpeningListener, error) {
		if opens.Add(1) == 1 {
			return first, nil
		}
		return &openingTestListener{}, nil
	})
	require.NoError(t, err)
	lease, err := r.grant(ctx, "request-first-0001", 1, tcpwindow.MinDuration)
	require.NoError(t, err)
	retry, err := r.grant(ctx, lease.ID, 1, tcpwindow.MinDuration)
	require.NoError(t, err)
	require.Equal(t, lease, retry)
	require.EqualValues(t, 1, opens.Load())
	require.ErrorIs(t, r.revoke(ctx, lease.ID, 2), tcpwindow.ErrNotFound)
	require.NoError(t, r.revoke(ctx, lease.ID, 1))
	require.EqualValues(t, 1, first.closes.Load())
	_, allowed := r.registry.Admit(time.Now())
	require.False(t, allowed)
	_, err = r.grant(ctx, lease.ID, 1, tcpwindow.MinDuration)
	require.ErrorIs(t, err, tcpwindow.ErrExpired)
	_, err = r.grant(ctx, "request-second-002", 1, tcpwindow.MinDuration)
	require.NoError(t, err)
	require.EqualValues(t, 2, opens.Load())
	cancel()
	select {
	case <-r.done:
	case <-time.After(time.Second):
		t.Fatal("runtime did not exit")
	}
	_, err = r.grant(context.Background(), "request-third-003", 1, tcpwindow.MinDuration)
	require.ErrorIs(t, err, tcpwindow.ErrClosed)
}

func TestTCPOpening_BindFailureDoesNotAuthorize(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	bindErr := errors.New("address in use")
	r, err := newTCPOpeningRuntime(ctx, func(context.Context) (tcpOpeningListener, error) { return nil, bindErr })
	require.NoError(t, err)
	_, err = r.grant(ctx, "request-first-0001", 1, tcpwindow.MinDuration)
	require.ErrorIs(t, err, bindErr)
	_, allowed := r.registry.Admit(time.Now())
	require.False(t, allowed)
	_, err = r.grant(ctx, "invalid", 1, tcpwindow.MinDuration)
	require.ErrorIs(t, err, tcpwindow.ErrInvalid)
}

func TestTCPOpening_RevokeDoesNotCloseOtherActiveGrant(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := &openingTestListener{}
	r, err := newTCPOpeningRuntime(ctx, func(context.Context) (tcpOpeningListener, error) { return listener, nil })
	require.NoError(t, err)
	_, err = r.grant(ctx, "request-first-0001", 1, tcpwindow.MinDuration)
	require.NoError(t, err)
	_, err = r.grant(ctx, "request-second-002", 1, tcpwindow.DefaultDuration)
	require.NoError(t, err)
	require.NoError(t, r.revoke(ctx, "request-first-0001", 1))
	require.Zero(t, listener.closes.Load())
	lease, allowed := r.registry.Admit(time.Now())
	require.True(t, allowed)
	require.Equal(t, "request-second-002", lease.ID)
	require.NoError(t, r.revoke(ctx, "request-second-002", 1))
	require.EqualValues(t, 1, listener.closes.Load())
}

func TestTCPOpening_CloseFailureDoesNotReportReopened(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	closeErr := errors.New("close failed")
	listener := &openingTestListener{err: closeErr}
	r, err := newTCPOpeningRuntime(ctx, func(context.Context) (tcpOpeningListener, error) { return listener, nil })
	require.NoError(t, err)
	_, err = r.grant(ctx, "request-first-0001", 1, tcpwindow.MinDuration)
	require.NoError(t, err)
	require.ErrorIs(t, r.revoke(ctx, "request-first-0001", 1), closeErr)
	_, err = r.grant(ctx, "request-second-002", 1, tcpwindow.MinDuration)
	require.ErrorIs(t, err, closeErr)
	_, allowed := r.registry.Admit(time.Now())
	require.False(t, allowed)
}

type openingSocketListener struct{ net.Listener }

func (l openingSocketListener) CloseListener() error { return l.Close() }

func TestTCPOpening_ExpiryClosesSocketButKeepsAcceptedConnection(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listeners := make(chan net.Listener, 1)
	r, err := newTCPOpeningRuntime(ctx, func(ctx context.Context) (tcpOpeningListener, error) {
		var lc net.ListenConfig
		listener, err := lc.Listen(ctx, "tcp", "127.0.0.1:0")
		if err != nil {
			return nil, err
		}
		listeners <- listener
		return openingSocketListener{listener}, nil
	})
	require.NoError(t, err)
	lease, err := r.grant(ctx, "request-first-0001", 1, tcpwindow.MinDuration)
	require.NoError(t, err)
	listener := <-listeners
	client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	require.NoError(t, err)
	defer client.Close()
	server, err := listener.Accept()
	require.NoError(t, err)
	defer server.Close()
	// 等真实的 5 秒窗口，不修改生产时长限制来使测试通过。
	timer := time.NewTimer(time.Until(lease.ExpiresAt))
	defer timer.Stop()
	<-timer.C
	require.Eventually(t, func() bool {
		conn, err := net.DialTimeout("tcp", listener.Addr().String(), 50*time.Millisecond)
		if err != nil {
			return true
		}
		require.NoError(t, conn.Close())
		return false
	}, time.Second, 10*time.Millisecond)
	require.NoError(t, client.SetDeadline(time.Now().Add(time.Second)))
	require.NoError(t, server.SetDeadline(time.Now().Add(time.Second)))
	_, err = client.Write([]byte("x"))
	require.NoError(t, err)
	buf := make([]byte, 1)
	_, err = server.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "x", string(buf))
	_, err = server.Write([]byte("y"))
	require.NoError(t, err)
	_, err = client.Read(buf)
	require.NoError(t, err)
	require.Equal(t, "y", string(buf))
}
