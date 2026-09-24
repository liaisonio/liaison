package probe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"strconv"
	"syscall"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
)

type registrar interface {
	RegisterRPCHandler(string, func(context.Context, geminio.Request, geminio.Response)) error
}
type dialFunc func(context.Context, string, string) (net.Conn, error)

// Register enables bounded, payload-free TCP probes on the Edge network.
func Register(fb registrar) error {
	slots := make(chan struct{}, 4)
	return fb.RegisterRPCHandler(proto.RPCTCPProbe, func(ctx context.Context, req geminio.Request, rsp geminio.Response) {
		if len(req.Data()) > 1024 {
			rsp.SetError(errors.New("invalid probe"))
			return
		}
		var target proto.TCPProbeRequest
		decoder := json.NewDecoder(bytes.NewReader(req.Data()))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&target); err != nil || !target.Valid() {
			rsp.SetError(errors.New("invalid probe"))
			return
		}
		if decoder.Decode(new(any)) != io.EOF {
			rsp.SetError(errors.New("invalid probe"))
			return
		}
		result := proto.TCPProbeResult{Version: 1, Status: "busy"}
		select {
		case slots <- struct{}{}:
			defer func() { <-slots }()
			result = run(ctx, target, (&net.Dialer{}).DialContext)
		default:
		}
		data, err := json.Marshal(result)
		if err != nil {
			rsp.SetError(err)
			return
		}
		rsp.SetData(data)
	})
}

func run(ctx context.Context, target proto.TCPProbeRequest, dial dialFunc) proto.TCPProbeResult {
	start := time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	conn, err := dial(ctx, "tcp", net.JoinHostPort(target.Host, strconv.Itoa(target.Port)))
	result := proto.TCPProbeResult{Version: 1, Status: "reachable", DurationMS: time.Since(start).Milliseconds()}
	if err == nil {
		if err = conn.Close(); err != nil {
			result.Status = "probe_unavailable"
		}
		return result
	}
	var dns *net.DNSError
	var network net.Error
	switch {
	case errors.Is(err, context.DeadlineExceeded), errors.As(err, &network) && network.Timeout():
		result.Status = "timeout"
	case errors.As(err, &dns):
		result.Status = "dns_error"
	case errors.Is(err, syscall.ECONNREFUSED):
		result.Status = "refused"
	case errors.Is(err, context.Canceled):
		result.Status = "probe_unavailable"
	default:
		result.Status = "unreachable"
	}
	return result
}
