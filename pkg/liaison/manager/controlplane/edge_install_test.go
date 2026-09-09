package controlplane

import (
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
)

func TestConnectorInstallCommands(t *testing.T) {
	for _, tc := range []struct {
		name, serverURL, baseURL, download, edge string
		port                                     int
	}{
		{"lan", "https://10.11.1.110", "https://10.11.1.110", "10.11.1.110", "10.11.1.110:30012", 0},
		{"domain_custom_ports", "https://liaison.example:8443/", "https://liaison.example:8443", "liaison.example:8443", "liaison.example:31012", 31012},
		{"http", "http://10.11.1.110:8080", "http://10.11.1.110:8080", "http://10.11.1.110:8080", "10.11.1.110:30012", 0},
		{"ipv6", "https://[2001:db8::1]:8443", "https://[2001:db8::1]:8443", "[2001:db8::1]:8443", "[2001:db8::1]:31012", 31012},
		{"path_prefix", "https://liaison.example/liaison/", "https://liaison.example/liaison", "https://liaison.example/liaison", "liaison.example:30012", 0},
		{"legacy_host", "liaison.example:8443", "https://liaison.example:8443", "liaison.example:8443", "liaison.example:30012", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			unix, windows, err := connectorInstallCommands(config.Manager{ServerURL: tc.serverURL, FrontierEdgePort: tc.port}, "test-ak", "test-sk")
			if err != nil {
				t.Fatal(err)
			}
			for _, part := range []string{tc.baseURL + "/install.sh", "--server-http-addr='" + tc.download + "'", "--server-edge-addr='" + tc.edge + "'"} {
				if !strings.Contains(unix, part) {
					t.Errorf("Unix command missing %q: %s", part, unix)
				}
			}
			for _, part := range []string{tc.baseURL + "/install.ps1", "-ServerHttpAddr '" + tc.download + "'", "-ServerEdgeAddr '" + tc.edge + "'", "$LASTEXITCODE -eq 0"} {
				if !strings.Contains(windows, part) {
					t.Errorf("Windows command missing %q: %s", part, windows)
				}
			}
		})
	}
}

func TestConnectorInstallCommandsListenFallback(t *testing.T) {
	for _, tls := range []bool{false, true} {
		manager := config.Manager{}
		manager.Listen.Addr = "10.11.1.110:8443"
		manager.Listen.TLS.Enable = tls
		unix, windows, err := connectorInstallCommands(manager, "ak", "sk")
		if err != nil {
			t.Fatal(err)
		}
		scheme := "http://"
		if tls {
			scheme = "https://"
		}
		for _, command := range []string{unix, windows} {
			if !strings.Contains(command, scheme+manager.Listen.Addr+"/install.") {
				t.Errorf("incorrect fallback: %s", command)
			}
		}
	}
}

func TestConnectorInstallCommandsRejectInvalidConfig(t *testing.T) {
	for _, serverURL := range []string{"ftp://example.com", "https://", "https://user:pass@example.com", "https://example.com?x=1", "https://example.com/#x", "https://[broken"} {
		if _, _, err := connectorInstallCommands(config.Manager{ServerURL: serverURL}, "ak", "sk"); err == nil {
			t.Errorf("accepted %q", serverURL)
		}
	}
	for _, port := range []int{-1, 65536} {
		if _, _, err := connectorInstallCommands(config.Manager{ServerURL: "https://example.com", FrontierEdgePort: port}, "ak", "sk"); err == nil {
			t.Errorf("accepted port %d", port)
		}
	}
}
