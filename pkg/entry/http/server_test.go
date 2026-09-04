package http

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
)

func TestDeleteProxyClosesIdleConnectionsWithoutWaitingForReadDeadline(t *testing.T) {
	server := NewServer(nil)
	t.Cleanup(server.Close)

	proxyConfig := &proto.Proxy{ID: 101, ProxyPort: 0}
	if err := server.CreateProxy(context.Background(), proxyConfig, "", ""); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	client, err := net.Dial("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(proxyConfig.ProxyPort)))
	if err != nil {
		t.Fatalf("dial proxy: %v", err)
	}
	defer client.Close()

	proxy := server.proxies[proxyConfig.ID]
	deadline := time.Now().Add(time.Second)
	for {
		proxy.connectionMu.Lock()
		active := len(proxy.connections)
		proxy.connectionMu.Unlock()
		if active == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("connection was not accepted before timeout")
		}
		time.Sleep(time.Millisecond)
	}

	started := time.Now()
	if err := server.DeleteProxy(context.Background(), proxyConfig.ID); err != nil {
		t.Fatalf("delete proxy: %v", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("delete proxy took %s; idle connection should be closed immediately", elapsed)
	}

	if _, exists := server.proxies[proxyConfig.ID]; exists {
		t.Fatal("deleted proxy remains registered")
	}
	_ = client.SetReadDeadline(time.Now().Add(time.Second))
	if _, err := client.Read(make([]byte, 1)); err == nil {
		t.Fatal("idle client connection remains open after proxy deletion")
	}
}
