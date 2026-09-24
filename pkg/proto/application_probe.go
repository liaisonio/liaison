package proto

import (
	"net"
	"regexp"
	"strings"
)

const RPCTCPProbe = "application_tcp_probe"

type TCPProbeRequest struct {
	Host string `json:"host"`
	Port int    `json:"port"`
}
type TCPProbeResult struct {
	Version    int    `json:"version"`
	Status     string `json:"status"`
	DurationMS int64  `json:"duration_ms"`
}

var probeLabel = regexp.MustCompile(`^[a-zA-Z0-9](?:[a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)

// Valid rejects URLs, credentials, ranges and multi-target inputs. Private and
// loopback destinations are intentional: they refer to the selected Edge host.
func (r TCPProbeRequest) Valid() bool {
	if r.Port < 1 || r.Port > 65535 || len(r.Host) == 0 || len(r.Host) > 253 {
		return false
	}
	if ip := net.ParseIP(r.Host); ip != nil {
		return !ip.IsUnspecified() && !ip.IsMulticast()
	}
	for _, label := range strings.Split(strings.TrimSuffix(r.Host, "."), ".") {
		if !probeLabel.MatchString(label) {
			return false
		}
	}
	return true
}
