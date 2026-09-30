package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"strings"
)

type AgentApplicationInput struct {
	Name           string `json:"name"`
	Kind           string `json:"kind"`
	EdgeID         uint64 `json:"edge_id"`
	InstallationID string `json:"installation_id"`
}

func supportedAgentKind(kind string) bool { return kind == "codex" || kind == "claude" }

type AgentApplicationList struct {
	Items []model.AgentApplication `json:"items"`
	Total int64                    `json:"total"`
}

func (cp *controlPlane) GetAgentApplication(ctx context.Context, id string) (*model.AgentApplication, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return nil, err
	}
	row, err := cp.repo.GetAgentApplication(ctx, actor, id)
	if err != nil {
		return nil, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "Application unavailable")
	}
	if !cp.ownsAgentConnector(actor, row.EdgeID) {
		return nil, notFound("CONNECTOR_NOT_FOUND", "Connector unavailable", nil)
	}
	return row, nil
}
func (cp *controlPlane) AgentApplications(ctx context.Context, page, size int, edge uint64) (AgentApplicationList, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return AgentApplicationList{}, err
	}
	if page < 1 || page > 10000 || size < 1 || size > 100 {
		return AgentApplicationList{}, badRequest("INVALID_PAGE", "Invalid page")
	}
	if edge != 0 && !cp.ownsAgentConnector(actor, edge) {
		return AgentApplicationList{}, notFound("CONNECTOR_NOT_FOUND", "Connector unavailable", nil)
	}
	rows, total, err := cp.repo.ListAgentApplications(ctx, actor, edge, page, size)
	return AgentApplicationList{rows, total}, err
}
func (cp *controlPlane) SaveAgentApplication(ctx context.Context, id string, input AgentApplicationInput) (*model.AgentApplication, error) {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return nil, err
	}
	input.Name = strings.TrimSpace(input.Name)
	if input.Name == "" || len(input.Name) > 120 || (id != "" && len(id) != 32) {
		return nil, badRequest("INVALID_APPLICATION", "Invalid application")
	}
	create := id == ""
	var row *model.AgentApplication
	if create {
		if !supportedAgentKind(input.Kind) || len(input.InstallationID) != 32 {
			return nil, badRequest("INVALID_APPLICATION", "Invalid application")
		}
		result, err := cp.edgeAgentLive(ctx, proto.EdgeAgentRequest{Action: "discover", EdgeID: input.EdgeID})
		if err != nil {
			return nil, err
		}
		found := false
		for _, i := range result.Installations {
			if i.ID == input.InstallationID && i.Kind == input.Kind {
				found = true
			}
		}
		if result.Status != "ok" || !found {
			return nil, badRequest("INSTALLATION_UNAVAILABLE", "Discover an available installation first")
		}
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		row = &model.AgentApplication{ID: hex.EncodeToString(random[:]), OwnerID: actor, EdgeID: input.EdgeID, Kind: input.Kind, InstallationID: input.InstallationID}
	} else {
		row, err = cp.repo.GetAgentApplication(ctx, actor, id)
		if err != nil {
			return nil, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "Application unavailable")
		}
		if !cp.ownsAgentConnector(actor, row.EdgeID) {
			return nil, notFound("CONNECTOR_NOT_FOUND", "Connector unavailable", nil)
		}
		if (input.EdgeID != 0 && input.EdgeID != row.EdgeID) || (input.Kind != "" && input.Kind != row.Kind) || (input.InstallationID != "" && input.InstallationID != row.InstallationID) {
			return nil, badRequest("IMMUTABLE_INSTALLATION", "Installation association cannot be changed")
		}
	}
	row.Name = input.Name
	if err := cp.repo.SaveAgentApplication(ctx, row, create); err != nil {
		return nil, mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "Application unavailable")
	}
	return row, nil
}
func (cp *controlPlane) DeleteAgentApplication(ctx context.Context, id string) error {
	actor, err := cp.agentActor(ctx)
	if err != nil {
		return err
	}
	row, err := cp.repo.GetAgentApplication(ctx, actor, id)
	if err != nil {
		return mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "Application unavailable")
	}
	if !cp.ownsAgentConnector(actor, row.EdgeID) {
		return notFound("CONNECTOR_NOT_FOUND", "Connector unavailable", nil)
	}
	err = cp.repo.DeleteAgentApplication(ctx, actor, id)
	if errors.Is(err, dao.ErrAgentApplicationInUse) {
		return conflict("APPLICATION_IN_USE", "Remove linked accesses first")
	}
	return mapRecordNotFound(err, "APPLICATION_NOT_FOUND", "Application unavailable")
}
