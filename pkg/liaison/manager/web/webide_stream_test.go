//go:build integration

package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/singchia/geminio"
	"github.com/singchia/geminio/client"
	"github.com/singchia/geminio/server"
	"github.com/stretchr/testify/require"
)

type ideSlowStream struct{ net.Conn }

func (c ideSlowStream) Read(b []byte) (int, error) {
	time.Sleep(time.Millisecond)
	return c.Conn.Read(b)
}

type ideStreamControlPlane struct {
	ideGatewayControlPlane
	end geminio.End
}

func (cp *ideStreamControlPlane) OpenWebIDEStream(context.Context, string, string) (net.Conn, error) {
	s, err := cp.end.OpenStream()
	if err != nil {
		return nil, err
	}
	return ideSlowStream{s}, nil
}

// Exercise a real buffered tunnel, not a TCP-only gateway mock. An upstream
// close must not discard the final HTTP chunks before the consumer reads them.
func TestIDEBufferedTunnelCompletesResponseBeforeClose(t *testing.T) {
	payload := bytes.Repeat([]byte("language-resource;"), 64<<10)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		w.WriteHeader(200)
		w.(http.Flusher).Flush()
		_, _ = w.Write(payload)
	}))
	t.Cleanup(upstream.Close)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { ln.Close() })
	ends := make(chan geminio.End, 1)
	errs := make(chan error, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			errs <- err
			return
		}
		opts := server.NewEndOptions()
		opts.SetBufferSize(512, 512)
		end, err := server.NewEndWithConn(conn, opts)
		if err != nil {
			conn.Close()
			errs <- err
			return
		}
		ends <- end
	}()
	conn, err := net.DialTimeout("tcp", ln.Addr().String(), time.Second)
	require.NoError(t, err)
	opts := client.NewEndOptions()
	opts.SetBufferSize(512, 512)
	consumer, err := client.NewEndWithConn(conn, opts)
	require.NoError(t, err)
	t.Cleanup(func() { consumer.Close() })
	var producer geminio.End
	select {
	case producer = <-ends:
	case err := <-errs:
		t.Fatal(err)
	case <-time.After(5 * time.Second):
		t.Fatal("tunnel handshake timed out")
	}
	t.Cleanup(func() { producer.Close() })
	go func() {
		for {
			stream, err := producer.AcceptStream()
			if err != nil {
				return
			}
			go func() {
				backend, err := net.DialTimeout("tcp", strings.TrimPrefix(upstream.URL, "http://"), time.Second)
				if err != nil {
					stream.Close()
					return
				}
				defer backend.Close()
				defer stream.Close()
				done := make(chan struct{})
				go func() { defer close(done); _, _ = io.Copy(backend, stream); backend.Close() }()
				_, _ = io.Copy(stream, backend)
				stream.Close()
				backend.Close()
				<-done
			}()
		}
	}()
	w, admin, _ := newPermissionHTTPTest(t)
	w.controlPlane = &ideStreamControlPlane{end: consumer}
	pat, err := w.iamService.CreatePAT(admin.ID, "stream-test", nil)
	require.NoError(t, err)
	g := &webIDEGateway{web: w}
	e := &ideEndpoint{access: "access", instance: "stream-test", origin: "https://ide.example.test", prefix: "/ide/stream-test/", grants: map[string]ideGrant{ideGrantKey("test-grant"): {token: pat.Token, expires: time.Now().Add(time.Minute)}}}
	for i := 0; i < 5; i++ {
		r := httptest.NewRequest("GET", e.origin+e.prefix+"nls.messages.js", nil)
		r.TLS = &tls.ConnectionState{}
		r.Close = true
		r.AddCookie(&http.Cookie{Name: "liaison_web_ide_" + e.instance, Value: "test-grant"})
		out := httptest.NewRecorder()
		g.serve(e, out, r)
		require.Equal(t, 200, out.Code)
		require.Equal(t, len(payload), out.Body.Len())
		require.Equal(t, sha256.Sum256(payload), sha256.Sum256(out.Body.Bytes()))
	}
}
