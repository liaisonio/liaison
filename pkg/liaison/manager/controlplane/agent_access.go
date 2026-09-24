package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"strings"
)

type AgentAccessInput struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	EdgeID         uint64 `json:"edge_id"`
	InstallationID string `json:"installation_id"`
	Project        string `json:"project"`
}

func (cp *controlPlane) GetAgentAccess(ctx context.Context, id string) (*model.AgentAccess, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return nil, err
	}
	row, err := cp.repo.GetAgentAccess(ctx, actor, id)
	return row, mapRecordNotFound(err, "ACCESS_NOT_FOUND", "Access unavailable")
}

type AgentAccessList struct {
	Items []model.AgentAccess `json:"items"`
	Total int64               `json:"total"`
}

func (cp *controlPlane) AgentAccesses(ctx context.Context, page, size int, filters ...model.AgentAccessFilter) (AgentAccessList, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return AgentAccessList{}, err
	}
	if page < 1 || page > 10000 || size < 1 || size > 100 {
		return AgentAccessList{}, badRequest("INVALID_PAGE", "Invalid page")
	}
	if len(filters) > 0 && (len(filters[0].Name) > 120 || (filters[0].Kind != "" && filters[0].Kind != "codex")) {
		return AgentAccessList{}, badRequest("INVALID_FILTER", "Invalid filter")
	}
	items, total, err := cp.repo.ListAgentAccesses(ctx, actor, page, size, filters...)
	return AgentAccessList{Items: items, Total: total}, err
}
func (cp *controlPlane) SaveAgentAccess(ctx context.Context, id string, input AgentAccessInput) (*model.AgentAccess, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	input.Project = strings.TrimSpace(input.Project)
	if input.Kind != "codex" || input.Name == "" || len(input.Name) > 120 || len(input.InstallationID) != 32 || input.Project == "" || len(input.Project) > 4096 || strings.ContainsRune(input.Project, 0) || (id != "" && len(id) != 32) {
		return nil, badRequest("INVALID_AGENT_ACCESS", "Invalid Agent access")
	}
	if !cp.ownsAgentConnector(actor, input.EdgeID) {
		return nil, notFound("CONNECTOR_NOT_FOUND", "Connector unavailable", nil)
	}
	if _, err := cp.repo.GetEdge(input.EdgeID); err != nil {
		return nil, mapRecordNotFound(err, "CONNECTOR_NOT_FOUND", "Connector unavailable")
	}
	create := id == ""
	if create {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		id = hex.EncodeToString(random[:])
	} else {
		if _, err := cp.repo.GetAgentAccess(ctx, actor, id); err != nil {
			return nil, mapRecordNotFound(err, "ACCESS_NOT_FOUND", "Access unavailable")
		}
	}
	row := &model.AgentAccess{ID: id, OwnerID: actor, Name: input.Name, Kind: input.Kind, EdgeID: input.EdgeID, InstallationID: input.InstallationID, Project: input.Project}
	if err := cp.repo.SaveAgentAccess(ctx, row, create); err != nil {
		return nil, mapRecordNotFound(err, "ACCESS_NOT_FOUND", "Access unavailable")
	}
	return cp.repo.GetAgentAccess(ctx, actor, id)
}
func (cp *controlPlane) DeleteAgentAccess(ctx context.Context, id string) error {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return err
	}
	return mapRecordNotFound(cp.repo.DeleteAgentAccess(ctx, actor, id), "ACCESS_NOT_FOUND", "Access unavailable")
}
