package web

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/gorilla/websocket"
)

func TestClassifyWebSSHDisconnect(t *testing.T) {
	for _, tc := range []struct {
		name, source string
		err          error
		want         webSSHDisconnect
	}{
		{"timeout", "websocket", fmt.Errorf("wrapped: %w", os.ErrDeadlineExceeded), webSSHDisconnect{reason: "client_heartbeat_timeout"}},
		{"normal", "websocket", &websocket.CloseError{Code: 1000, Text: "secret-peer-text"}, webSSHDisconnect{reason: "client_closed", success: true, code: 1000}},
		{"away", "websocket", &websocket.CloseError{Code: 1001}, webSSHDisconnect{reason: "client_closed", success: true, code: 1001}},
		{"abnormal", "websocket", &websocket.CloseError{Code: 1006}, webSSHDisconnect{reason: "websocket_closed", code: 1006}},
		{"browser timeout", "websocket", &websocket.CloseError{Code: 4000}, webSSHDisconnect{reason: "client_reported_server_timeout", code: 4000}},
		{"transport", "websocket", errors.New("secret-error-text"), webSSHDisconnect{reason: "websocket_transport_error"}},
		{"ssh exit", "ssh", nil, webSSHDisconnect{reason: "ssh_exit", success: true}},
		{"ssh error", "ssh", errors.New("secret-error-text"), webSSHDisconnect{reason: "ssh_error"}},
		{"cancel", "context", context.Canceled, webSSHDisconnect{reason: "session_cancelled"}},
		{"setup", "unknown", nil, webSSHDisconnect{reason: "setup_failed"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := classifyWebSSHDisconnect(tc.source, tc.err); got != tc.want {
				t.Fatalf("got %+v, want %+v", got, tc.want)
			}
		})
	}
}
