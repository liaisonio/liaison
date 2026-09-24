package controlplane

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
	"strconv"
)

func (cp *controlPlane) archiveTranscript(ctx context.Context, scope *model.EdgeAgentHistory, snapshot proto.EdgeAgentResult) error {
	page := proto.EdgeAgentResult{Version: 1, Status: "ok", SessionID: snapshot.SessionID, Window: snapshot.Window, Messages: snapshot.Messages, Activities: snapshot.Activities, Truncated: snapshot.Truncated, Archived: true, Closed: true, HistoryPersistent: true, HistoryWindowing: true}
	raw, err := json.Marshal(page)
	if err != nil {
		return err
	}
	if len(raw) > 512<<10 {
		return errors.New("history page too large")
	}
	encrypted, err := cp.sealHistory(raw, historyAAD(scope)+"/page/"+strconv.FormatUint(snapshot.Window, 10))
	if err != nil {
		return err
	}
	return cp.repo.SaveEdgeAgentHistoryPage(ctx, &model.EdgeAgentHistoryPage{OwnerID: scope.OwnerID, AccessID: scope.AccessID, EdgeID: scope.EdgeID, SessionID: scope.SessionID, Window: snapshot.Window, Revision: snapshot.Revision, Payload: encrypted})
}

func (cp *controlPlane) previousTranscript(ctx context.Context, scope *model.EdgeAgentHistory, cursor string) (proto.EdgeAgentResult, error) {
	before, err := strconv.ParseUint(cursor, 10, 63)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	page, err := cp.repo.PreviousEdgeAgentHistoryPage(ctx, scope, before)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return proto.EdgeAgentResult{Version: 1, Status: "not_found"}, nil
	}
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	raw, err := cp.openHistory(page.Payload, historyAAD(scope)+"/page/"+strconv.FormatUint(page.Window, 10))
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	var result proto.EdgeAgentResult
	if err = json.Unmarshal(raw, &result); err != nil {
		return result, err
	}
	// A deletion or ownership change during a delayed read must not expose history.
	if _, err = cp.historyScope(ctx, proto.EdgeAgentRequest{Action: "transcript", EdgeID: scope.EdgeID, AccessID: scope.AccessID, SessionID: scope.SessionID, HistoryBefore: cursor}); err != nil {
		return proto.EdgeAgentResult{}, err
	}
	parent, err := cp.repo.GetEdgeAgentHistory(ctx, scope)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	if parent.Deleted {
		return proto.EdgeAgentResult{Version: 1, Status: "not_found"}, nil
	}
	return result, nil
}
