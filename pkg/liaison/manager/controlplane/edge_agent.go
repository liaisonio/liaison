package controlplane

import (
	"context"
	"strconv"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"k8s.io/klog/v2"
)

type edgeAgentCaller interface {
	EdgeAgent(context.Context, uint64, proto.EdgeAgentRPCRequest) (proto.EdgeAgentResult, error)
}
type AgentConnector struct {
	ID     uint64 `json:"id"`
	Name   string `json:"name"`
	Online bool   `json:"online"`
	Device string `json:"device,omitempty"`
}

func (cp *controlPlane) agentActor(ctx context.Context) (uint, error) {
	actor, ok := actorUserID(ctx)
	if !ok || cp.authorizeFeature == nil {
		return 0, iam.ErrForbidden
	}
	if err := cp.authorizeFeature(ctx, iam.FeatureAccessAI); err != nil {
		return 0, err
	}
	return actor, nil
}

// No root-admin bypass: this grants use of the Edge OS account's local Agent.
func (cp *controlPlane) ownsAgentConnector(actor uint, id uint64) bool {
	relations, err := cp.repo.ListIAMResourceRelations(resourceConnector, id)
	if err != nil {
		return false
	}
	for _, r := range relations {
		if r.Relation == model.IAMRelationOwner && r.SubjectType == model.IAMSubjectUser && r.SubjectID == actor {
			return true
		}
	}
	return false
}
func (cp *controlPlane) AgentConnectors(ctx context.Context) ([]AgentConnector, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return nil, err
	}
	ids, err := cp.repo.ListIAMResourceIDsForSubject(resourceConnector, model.IAMSubjectUser, actor)
	if err != nil {
		return nil, err
	}
	connectors := make([]AgentConnector, 0)
	for _, id := range ids {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if len(connectors) >= 100 {
			break
		}
		if !cp.ownsAgentConnector(actor, id) {
			continue
		}
		edge, err := cp.repo.GetEdge(id)
		if err != nil {
			continue
		}
		device := ""
		hostType := model.EdgeDeviceRelationHost
		if hosts, err := cp.repo.GetEdgeDevicesByEdgeID(id, &hostType); err == nil && len(hosts) > 0 {
			if host, err := cp.repo.GetDeviceByID(hosts[0].DeviceID); err == nil && host != nil {
				device = host.HostName
				if device == "" {
					device = host.Name
				}
			}
		}
		connectors = append(connectors, AgentConnector{ID: id, Name: edge.Name, Device: device, Online: edge.Online == model.EdgeOnlineStatusOnline && edge.Status == model.EdgeStatusRunning})
	}
	return connectors, nil
}
func (cp *controlPlane) edgeAgentLive(ctx context.Context, req proto.EdgeAgentRequest) (proto.EdgeAgentResult, error) {
	return cp.edgeAgentLiveEnvelope(ctx, req, nil)
}

func (cp *controlPlane) edgeAgentLiveEnvelope(ctx context.Context, req proto.EdgeAgentRequest, resume *proto.EdgeAgentResume, reset ...uint64) (proto.EdgeAgentResult, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	if req.EdgeID == 0 || !req.Valid() {
		return proto.EdgeAgentResult{}, badRequest("INVALID_AGENT_REQUEST", "Invalid agent request")
	}
	projectRoot := ""
	if req.AccessID != "" {
		entry, err := cp.repo.GetAgentAccess(ctx, actor, req.AccessID)
		if err != nil {
			return proto.EdgeAgentResult{}, mapRecordNotFound(err, "ACCESS_NOT_FOUND", "Access unavailable")
		}
		if entry.EdgeID != req.EdgeID || (req.Action == "start" && (entry.InstallationID != req.InstallationID || entry.Project != req.Project)) {
			return proto.EdgeAgentResult{}, notFound("ACCESS_NOT_FOUND", "Access unavailable", nil)
		}
		projectRoot = entry.Project
	}
	if !cp.ownsAgentConnector(actor, req.EdgeID) {
		return proto.EdgeAgentResult{}, notFound("CONNECTOR_NOT_FOUND", "Connector unavailable", nil)
	}
	edge, err := cp.repo.GetEdge(req.EdgeID)
	if err != nil {
		return proto.EdgeAgentResult{}, mapRecordNotFound(err, "CONNECTOR_NOT_FOUND", "Connector unavailable")
	}
	fallback := proto.EdgeAgentResult{Version: 1, Status: "unavailable"}
	if edge.Online != model.EdgeOnlineStatusOnline || edge.Status != model.EdgeStatusRunning {
		return fallback, nil
	}
	caller, ok := cp.frontierBound.(edgeAgentCaller)
	if !ok {
		fallback.Status = "upgrade_required"
		return fallback, nil
	}
	ctx, cancel := context.WithTimeout(ctx, 22*time.Second)
	defer cancel()
	var resetRevision uint64
	if len(reset) > 0 {
		resetRevision = reset[0]
	}
	reply, err := caller.EdgeAgent(ctx, req.EdgeID, proto.EdgeAgentRPCRequest{ResetRevision: resetRevision, Version: 1, ActorID: strconv.FormatUint(uint64(actor), 10), Request: req, ProjectRoot: projectRoot, Resume: resume})
	if err != nil {
		return fallback, nil
	}
	if reply.Version != 1 {
		fallback.Status = "upgrade_required"
		return fallback, nil
	}
	// A watch may wait for 15 seconds. Recheck ownership after the wait before
	// returning newly produced data, including entry deletion/reassignment.
	if req.Action == "watch" {
		if _, err := cp.agentActor(ctx); err != nil {
			return proto.EdgeAgentResult{}, err
		}
		if !cp.ownsAgentConnector(actor, req.EdgeID) {
			return proto.EdgeAgentResult{}, iam.ErrForbidden
		}
		if req.AccessID != "" {
			entry, err := cp.repo.GetAgentAccess(ctx, actor, req.AccessID)
			if err != nil || entry.EdgeID != req.EdgeID {
				return proto.EdgeAgentResult{}, notFound("ACCESS_NOT_FOUND", "Access unavailable", nil)
			}
		}
	}
	switch reply.Status {
	case "ok", "busy", "unavailable", "upgrade_required", "not_found", "unsupported_user", "launch_failed", "external_tools", "authentication_required", "session_closed", "turn_failed", "approval_declined", "output_limit", "expired", "invalid_request", "resume_unavailable":
	default:
		return fallback, nil
	}
	if req.Action == "answer" || req.Action == "approve" || req.Action == "permissions" || req.Action == "start" || req.Action == "resume" || req.Action == "send" || req.Action == "stop" || req.Action == "interrupt" || req.Action == "model" || req.Action == "rename" || req.Action == "delete" || req.Action == "discard" {
		// No prompt, output, path or credential is written to management logs.
		if err := cp.RecordManagementAudit(ctx, &ManagementAudit{UserID: actor, Module: "agent", Action: req.Action, Resource: "connector/" + strconv.FormatUint(req.EdgeID, 10), Method: "POST", Success: reply.Status == "ok" || reply.Status == "session_closed", StatusCode: 200}); err != nil {
			klog.Warning("agent management audit could not be recorded")
		}
	}
	return reply, nil
}
