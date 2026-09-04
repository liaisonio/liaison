package entry

import (
	"testing"

	"github.com/liaisonio/liaison/pkg/proto"
)

func TestRuntimeForProxy(t *testing.T) {
	tests := []struct {
		name  string
		proxy *proto.Proxy
		want  proxyRuntime
	}{
		{name: "nil defaults to tcp", want: proxyRuntimeTCP},
		{name: "raw tcp http application stays tcp", proxy: &proto.Proxy{AccessProtocol: "tcp", ApplicationType: "http"}, want: proxyRuntimeTCP},
		{name: "http access uses http runtime", proxy: &proto.Proxy{AccessProtocol: "http", ApplicationType: "http"}, want: proxyRuntimeHTTP},
		{name: "ssh access uses ssh gateway", proxy: &proto.Proxy{AccessProtocol: "ssh", ApplicationType: "ssh"}, want: proxyRuntimeSSH},
		{name: "native mysql access uses l4 runtime", proxy: &proto.Proxy{AccessProtocol: "mysql", ApplicationType: "mysql"}, want: proxyRuntimeTCP},
		{name: "web access has no listener runtime", proxy: &proto.Proxy{AccessProtocol: "web", ApplicationType: "rdp"}, want: proxyRuntimeTCP},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := runtimeForProxy(test.proxy); got != test.want {
				t.Fatalf("runtimeForProxy() = %q, want %q", got, test.want)
			}
		})
	}
}
