package probe

import (
	"context"
	"errors"
	"io"
	"net"
	"syscall"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func TestProbeConnectsWithoutSendingData(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer listener.Close()
	result := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			result <- err
			return
		}
		defer conn.Close()
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, err := conn.Read(make([]byte, 1))
		if n != 0 {
			result <- errors.New("unexpected probe payload")
			return
		}
		result <- err
	}()
	target := proto.TCPProbeRequest{Host: "127.0.0.1", Port: listener.Addr().(*net.TCPAddr).Port}
	got := run(context.Background(), target, (&net.Dialer{}).DialContext)
	require.Equal(t, "reachable", got.Status)
	require.ErrorIs(t, <-result, io.EOF)
}

func TestProbeErrors(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status string
	}{
		{syscall.ECONNREFUSED, "refused"}, {context.DeadlineExceeded, "timeout"},
		{context.Canceled, "probe_unavailable"}, {&net.DNSError{Err: "secret detail"}, "dns_error"},
		{errors.New("private detail"), "unreachable"},
	} {
		t.Run(tc.status, func(t *testing.T) {
			got := run(context.Background(), proto.TCPProbeRequest{Host: "::1", Port: 443}, func(ctx context.Context, network, address string) (net.Conn, error) {
				require.Equal(t, "tcp", network)
				require.Equal(t, "[::1]:443", address)
				_, ok := ctx.Deadline()
				require.True(t, ok)
				return nil, tc.err
			})
			require.Equal(t, tc.status, got.Status)
		})
	}
}
