package web

import (
	"errors"
	"net"

	"github.com/gorilla/websocket"
)

// Never persist peer-provided close text or raw errors: they may contain secrets.
type webSSHDisconnect struct {
	reason  string
	success bool
	code    int
}

func classifyWebSSHDisconnect(source string, err error) webSSHDisconnect {
	switch source {
	case "context":
		return webSSHDisconnect{reason: "session_cancelled"}
	case "ssh":
		if err == nil {
			return webSSHDisconnect{reason: "ssh_exit", success: true}
		}
		return webSSHDisconnect{reason: "ssh_error"}
	case "websocket":
		var timeout net.Error
		if errors.As(err, &timeout) && timeout.Timeout() {
			return webSSHDisconnect{reason: "client_heartbeat_timeout"}
		}
		var closed *websocket.CloseError
		if errors.As(err, &closed) {
			d := webSSHDisconnect{reason: "websocket_closed", code: closed.Code}
			switch closed.Code {
			case websocket.CloseNormalClosure, websocket.CloseGoingAway:
				d.reason, d.success = "client_closed", true
			case 4000:
				d.reason = "client_reported_server_timeout"
			}
			return d
		}
		return webSSHDisconnect{reason: "websocket_transport_error"}
	default:
		return webSSHDisconnect{reason: "setup_failed"}
	}
}
