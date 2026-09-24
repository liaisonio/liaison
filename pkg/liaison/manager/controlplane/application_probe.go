package controlplane

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"sync"
	"time"
)

type ApplicationProbeRequest struct {
	EdgeID        uint64 `json:"edge_id"`
	Host          string `json:"host"`
	Port          int    `json:"port"`
	ApplicationID uint64 `json:"application_id"`
}
type applicationProber interface {
	ProbeApplication(context.Context, uint64, proto.TCPProbeRequest) (proto.TCPProbeResult, error)
}

// Bound repeated probes as well as concurrent in-flight requests.
type probeRateLimit struct {
	mu    sync.Mutex
	start time.Time
	count int
}

func (limit *probeRateLimit) Allow() bool {
	limit.mu.Lock()
	defer limit.mu.Unlock()
	if time.Since(limit.start) >= 2*time.Second {
		limit.start, limit.count = time.Now(), 0
	}
	if limit.count >= 4 {
		return false
	}
	limit.count++
	return true
}

func (cp *controlPlane) ProbeApplication(ctx context.Context, request ApplicationProbeRequest) (proto.TCPProbeResult, error) {
	result := proto.TCPProbeResult{Version: 1, Status: "probe_unavailable"}
	if _, ok := actorUserID(ctx); !ok {
		return result, iam.ErrForbidden
	}
	if request.ApplicationID != 0 {
		if request.EdgeID != 0 || request.Host != "" || request.Port != 0 {
			return result, badRequest("INVALID_PROBE", "Use the saved application target")
		}
		if err := requireVisibleResource(ctx, cp.repo, resourceApplication, request.ApplicationID); err != nil {
			return result, err
		}
		app, err := cp.repo.GetApplicationByID(uint(request.ApplicationID))
		if err != nil {
			return result, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "Application not found")
		}
		if len(app.EdgeIDs) == 0 {
			return result, nil
		}
		request.EdgeID, request.Host, request.Port = uint64(app.EdgeIDs[0]), app.IP, app.Port
	}
	target := proto.TCPProbeRequest{Host: request.Host, Port: request.Port}
	if request.EdgeID == 0 || !target.Valid() {
		return result, badRequest("INVALID_PROBE", "Invalid connector, host or port")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceConnector, request.EdgeID); err != nil {
		return result, err
	}
	edge, err := cp.repo.GetEdge(request.EdgeID)
	if err != nil {
		return result, mapRecordNotFound(err, "EDGE_NOT_FOUND", "Connector not found")
	}
	if edge.Online != model.EdgeOnlineStatusOnline || edge.Status != model.EdgeStatusRunning {
		result.Status = "connector_offline"
		return result, nil
	}
	prober, ok := cp.frontierBound.(applicationProber)
	if !ok {
		result.Status = "edge_upgrade_required"
		return result, nil
	}
	if !cp.probeLimiter.Allow() {
		result.Status = "busy"
		return result, nil
	}
	select {
	case cp.probeSlots <- struct{}{}:
		defer func() { <-cp.probeSlots }()
	default:
		result.Status = "busy"
		return result, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 7*time.Second)
	defer cancel()
	reply, err := prober.ProbeApplication(ctx, request.EdgeID, target)
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.Status = "timeout"
		}
		return result, nil
	}
	if reply.Version != 1 {
		result.Status = "edge_upgrade_required"
		return result, nil
	}
	switch reply.Status {
	case "reachable", "refused", "timeout", "dns_error", "unreachable", "busy", "edge_upgrade_required", "probe_unavailable":
	default:
		return result, nil
	}
	if reply.DurationMS < 0 || reply.DurationMS > 7000 {
		return result, nil
	}
	return reply, nil
}
