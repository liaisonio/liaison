package transport

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/transport/tcpwindow"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
	"github.com/stretchr/testify/require"
)

type openingStream struct {
	geminio.Stream
	conn net.Conn
}

func (s openingStream) Read(p []byte) (int, error)         { return s.conn.Read(p) }
func (s openingStream) Write(p []byte) (int, error)        { return s.conn.Write(p) }
func (s openingStream) Close() error                       { return s.conn.Close() }
func (s openingStream) LocalAddr() net.Addr                { return s.conn.LocalAddr() }
func (s openingStream) RemoteAddr() net.Addr               { return s.conn.RemoteAddr() }
func (s openingStream) SetDeadline(t time.Time) error      { return s.conn.SetDeadline(t) }
func (s openingStream) SetReadDeadline(t time.Time) error  { return s.conn.SetReadDeadline(t) }
func (s openingStream) SetWriteDeadline(t time.Time) error { return s.conn.SetWriteDeadline(t) }

type openingFrontier struct{ done chan struct{} }

func (f openingFrontier) Close() error { return nil }
func (f openingFrontier) OpenStream(ctx context.Context, _ uint64) (geminio.Stream, error) {
	client, server := net.Pipe()
	go func() {
		defer close(f.done)
		defer server.Close()
		var size uint32
		if binary.Read(server, binary.BigEndian, &size) != nil {
			return
		}
		if _, err := io.CopyN(io.Discard, server, int64(size)); err != nil {
			return
		}
		_, _ = io.Copy(server, server)
	}()
	return openingStream{conn: client}, nil
}

func openingTarget(t *testing.T, id int) *proto.Proxy {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return &proto.Proxy{ID: id, ProxyPort: port, AccessProtocol: "tcp", TCPOpeningMode: "on_demand", Dst: "127.0.0.1:22"}
}

func TestOnDemandRealForwardingSurvivesExpiry(t *testing.T) {
	done := make(chan struct{})
	m := NewGatekeeper(openingFrontier{done})
	defer m.Close()
	target := openingTarget(t, 1)
	require.NoError(t, m.CreateProxy(context.Background(), target))
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(target.ProxyPort))
	probe, err := net.Listen("tcp", address)
	require.NoError(t, err, "idle route must not bind port")
	require.NoError(t, probe.Close())
	lease, err := m.GrantTCPOpening(context.Background(), 1, "real-forward-0001", 1, 5*time.Second)
	require.NoError(t, err)
	client, err := net.DialTimeout("tcp", address, time.Second)
	require.NoError(t, err)
	defer client.Close()
	ping := func() {
		require.NoError(t, client.SetDeadline(time.Now().Add(time.Second)))
		_, err := client.Write([]byte("ping"))
		require.NoError(t, err)
		b := make([]byte, 4)
		_, err = io.ReadFull(client, b)
		require.NoError(t, err)
		require.Equal(t, "ping", string(b))
	}
	ping()
	timer := time.NewTimer(time.Until(lease.ExpiresAt.Add(100 * time.Millisecond)))
	defer timer.Stop()
	<-timer.C
	fresh, err := net.DialTimeout("tcp", address, time.Second)
	if fresh != nil {
		fresh.Close()
	}
	require.Error(t, err, "expired listener must be closed")
	ping()
	require.NoError(t, m.DeleteProxy(context.Background(), 1))
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("deleted route kept connection alive")
	}
	require.NoError(t, m.CreateProxy(context.Background(), target))
	_, err = m.GetTCPOpening(context.Background(), 1, lease.ID, 1)
	require.ErrorIs(t, err, tcpwindow.ErrNotFound, "rebuild must not restore windows")
}

type denyOpeningFirewall struct{}

func (denyOpeningFirewall) CheckAddr(int, net.Addr) bool { return false }

func TestOnDemandFirewallAndBindFailure(t *testing.T) {
	m := NewGatekeeper(nil)
	defer m.Close()
	target := openingTarget(t, 2)
	require.NoError(t, m.CreateProxy(context.Background(), target))
	address := net.JoinHostPort("127.0.0.1", strconv.Itoa(target.ProxyPort))
	occupied, err := net.Listen("tcp", ":"+strconv.Itoa(target.ProxyPort))
	require.NoError(t, err)
	_, err = m.GrantTCPOpening(context.Background(), 2, "bind-failure-0001", 1, 5*time.Second)
	require.Error(t, err)
	_, err = m.GetTCPOpening(context.Background(), 2, "bind-failure-0001", 1)
	require.ErrorIs(t, err, tcpwindow.ErrNotFound)
	require.NoError(t, occupied.Close())
	m.SetFirewall(denyOpeningFirewall{})
	_, err = m.GrantTCPOpening(context.Background(), 2, "bind-success-0001", 1, 5*time.Second)
	require.NoError(t, err)
	client, err := net.DialTimeout("tcp", address, time.Second)
	require.NoError(t, err)
	defer client.Close()
	require.NoError(t, client.SetReadDeadline(time.Now().Add(time.Second)))
	_, err = client.Read(make([]byte, 1))
	require.Error(t, err, "firewall must still reject during an open window")
	require.NoError(t, m.RevokeTCPOpening(context.Background(), 2, "bind-success-0001", 1))
}
