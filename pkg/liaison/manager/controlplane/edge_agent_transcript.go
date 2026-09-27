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

func (cp *controlPlane) previousTranscript(ctx context.Context, scope *model.EdgeAgentHistory, cursor, limitText string) (proto.EdgeAgentResult, error) {
	before, err := strconv.ParseUint(cursor, 10, 63)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	limit := 1
	if limitText != "" {
		limit, err = strconv.Atoi(limitText)
		if err != nil || limit < 1 || limit > 20 {
			return proto.EdgeAgentResult{}, errors.New("invalid history limit")
		}
	}
	pages, err := cp.repo.ListEdgeAgentHistoryPages(ctx, scope, before, limit, (2<<20)-1024)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return proto.EdgeAgentResult{Version: 1, Status: "not_found"}, nil
	}
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	result := proto.EdgeAgentResult{Version: 1, Status: "ok", SessionID: scope.SessionID, HistoryPersistent: true}
	for _, page := range pages {
		raw, openErr := cp.openHistory(page.Payload, historyAAD(scope)+"/page/"+strconv.FormatUint(page.Window, 10))
		if openErr != nil {
			return proto.EdgeAgentResult{}, openErr
		}
		var snapshot proto.EdgeAgentResult
		if err = json.Unmarshal(raw, &snapshot); err != nil {
			return proto.EdgeAgentResult{}, err
		}
		result.HistoryPages = append(result.HistoryPages, snapshot)
		if page.Window > 0 {
			result.HistoryBefore = strconv.FormatUint(page.Window, 10)
		} else {
			result.HistoryBefore = ""
		}
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
	if limitText == "" {
		if len(result.HistoryPages) == 0 {
			return proto.EdgeAgentResult{Version: 1, Status: "not_found"}, nil
		}
		return result.HistoryPages[0], nil
	}
	return result, nil
}
