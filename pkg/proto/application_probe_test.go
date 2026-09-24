package proto

import "testing"

func TestTCPProbeTargetValidation(t *testing.T) {
	for _, host := range []string{"localhost", "127.0.0.1", "10.0.0.1", "db.internal", "::1", "example.com."} {
		if !(TCPProbeRequest{Host: host, Port: 443}).Valid() {
			t.Errorf("valid host rejected: %s", host)
		}
	}
	for _, host := range []string{"", "0.0.0.0", "::", "224.0.0.1", "http://host/path", "user@host", "host:443", "a b", "*.internal", "-bad", "a..b"} {
		if (TCPProbeRequest{Host: host, Port: 443}).Valid() {
			t.Errorf("invalid host accepted: %s", host)
		}
	}
	for _, port := range []int{-1, 0, 65536} {
		if (TCPProbeRequest{Host: "localhost", Port: port}).Valid() {
			t.Errorf("invalid port accepted: %d", port)
		}
	}
}
