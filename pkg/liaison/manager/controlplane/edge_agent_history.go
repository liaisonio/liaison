package controlplane

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
	"k8s.io/klog/v2"
)

func newHistoryCipher(secret string) (cipher.AEAD, error) {
	key := sha256.Sum256([]byte("liaison-edge-agent-history-v1:" + secret))
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
func historyAAD(r *model.EdgeAgentHistory) string {
	return fmt.Sprintf("%d/%s/%d/%s", r.OwnerID, r.AccessID, r.EdgeID, r.SessionID)
}
func (cp *controlPlane) sealHistory(raw []byte, aad string) ([]byte, error) {
	nonce := make([]byte, cp.historyCipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	return cp.historyCipher.Seal(nonce, nonce, raw, []byte(aad)), nil
}
func (cp *controlPlane) openHistory(raw []byte, aad string) ([]byte, error) {
	n := cp.historyCipher.NonceSize()
	if len(raw) < n {
		return nil, errors.New("invalid history ciphertext")
	}
	return cp.historyCipher.Open(nil, raw[:n], raw[n:], []byte(aad))
}
func (cp *controlPlane) historySnapshot(r *model.EdgeAgentHistory) (proto.EdgeAgentResult, error) {
	var out proto.EdgeAgentResult
	raw, err := cp.openHistory(r.Payload, historyAAD(r))
	if err != nil {
		return out, err
	}
	if err = json.Unmarshal(raw, &out); err != nil {
		return out, err
	}
	if len(r.TitleOverride) > 0 {
		title, err := cp.openHistory(r.TitleOverride, historyAAD(r)+"/title")
		if err != nil {
			return out, err
		}
		out.Title = string(title)
	}
	out.HistoryPersistent = true
	out.SessionManagement = true
	return out, nil
}
func (cp *controlPlane) saveHistory(ctx context.Context, scope *model.EdgeAgentHistory, snapshot proto.EdgeAgentResult) error {
	if len(snapshot.SessionID) != 32 || snapshot.SessionID != scope.SessionID {
		return errors.New("invalid session snapshot")
	}
	old, err := cp.repo.GetEdgeAgentHistory(ctx, scope)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	if err == nil && (old.Deleted || old.Revision >= snapshot.Revision) {
		return nil
	}
	snapshot.Models = nil
	// Native approvals are live, one-shot capabilities, never archived or replayed.
	snapshot.Approvals = nil
	snapshot.InputRequests = nil
	snapshot.PermissionsAvailable = false
	snapshot.Skills = nil
	snapshot.Sessions = nil
	snapshot.Directories = nil
	snapshot.HistoryPersistent = false
	snapshot.Archived = false
	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	if len(raw) > 512<<10 {
		return errors.New("history snapshot too large")
	}
	row := *scope
	row.Payload, err = cp.sealHistory(raw, historyAAD(scope))
	if err != nil {
		return err
	}
	row.Revision = snapshot.Revision
	row.Closed = snapshot.Closed
	row.UpdatedAt = time.Now()
	row.SyncedAt = row.UpdatedAt
	return cp.repo.SaveEdgeAgentHistory(ctx, &row)
}
func (cp *controlPlane) historyScope(ctx context.Context, req proto.EdgeAgentRequest) (*model.EdgeAgentHistory, error) {
	owner, err := cp.agentActor(ctx)
	if err != nil {
		return nil, err
	}
	if req.EdgeID == 0 || !req.Valid() {
		return nil, badRequest("INVALID_AGENT_REQUEST", "Invalid agent request")
	}
	entry, err := cp.repo.GetAgentAccess(ctx, owner, req.AccessID)
	if err != nil || entry.EdgeID != req.EdgeID || !cp.ownsAgentConnector(owner, req.EdgeID) {
		return nil, notFound("ACCESS_NOT_FOUND", "Access unavailable", nil)
	}
	return &model.EdgeAgentHistory{OwnerID: owner, AccessID: req.AccessID, EdgeID: req.EdgeID, SessionID: req.SessionID}, nil
}
func (cp *controlPlane) EdgeAgent(ctx context.Context, req proto.EdgeAgentRequest) (proto.EdgeAgentResult, error) {
	if cp.historyCipher == nil || req.AccessID == "" {
		return cp.edgeAgentLive(ctx, req)
	}
	scope, err := cp.historyScope(ctx, req)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	if req.Action == "sessions" {
		return cp.historyList(ctx, req, scope)
	}
	var saved *model.EdgeAgentHistory
	if req.SessionID != "" {
		saved, err = cp.repo.GetEdgeAgentHistory(ctx, scope)
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return proto.EdgeAgentResult{}, err
		}
		if errors.Is(err, gorm.ErrRecordNotFound) {
			saved = nil
		}
		if saved != nil && saved.Deleted {
			return proto.EdgeAgentResult{Version: 1, Status: "not_found"}, nil
		}
	}
	if saved != nil && req.Action == "rename" {
		saved.TitleOverride, err = cp.sealHistory([]byte(req.Title), historyAAD(saved)+"/title")
		if err != nil {
			return proto.EdgeAgentResult{}, err
		}
		if err = cp.repo.MutateEdgeAgentHistory(ctx, saved, false); err != nil {
			return proto.EdgeAgentResult{}, err
		}
		out, err := cp.historySnapshot(saved)
		out.Archived = true
		out.Closed = true
		out.Running = false
		cp.auditHistoryMutation(ctx, scope, req.Action)
		return out, err
	}
	if saved != nil && req.Action == "delete" {
		if err = cp.repo.MutateEdgeAgentHistory(ctx, saved, true); err != nil {
			return proto.EdgeAgentResult{}, err
		}
		// Tombstone first: a late watch or background sync cannot resurrect payloads.
		stop := req
		stop.Action = "stop"
		if _, err := cp.edgeAgentLive(ctx, stop); err != nil {
			klog.Warning("deleted agent history could not stop remote session")
		}
		remove := req
		remove.Action = "delete"
		if _, err := cp.edgeAgentLive(ctx, remove); err != nil {
			klog.Warning("deleted agent history could not remove remote binding")
		}
		cp.auditHistoryMutation(ctx, scope, req.Action)
		return proto.EdgeAgentResult{Version: 1, Status: "ok", HistoryPersistent: true}, nil
	}
	var reply proto.EdgeAgentResult
	if req.Action == "transcript" {
		if saved == nil {
			return proto.EdgeAgentResult{Version: 1, Status: "not_found"}, nil
		}
		return cp.previousTranscript(ctx, scope, req.HistoryBefore)
	}
	if req.Action == "send" {
		currentReq := proto.EdgeAgentRequest{Action: "snapshot", EdgeID: req.EdgeID, AccessID: req.AccessID, SessionID: req.SessionID}
		current, e := cp.edgeAgentLive(ctx, currentReq)
		if e != nil {
			return current, e
		}
		if current.Running {
			return proto.EdgeAgentResult{Version: 1, Status: "busy"}, nil
		}
		if current.Closed || current.SessionID == "" {
			return current, nil
		}
		var reset uint64
		if current.HistoryWindowing && len(current.Messages) > 0 {
			if err = cp.saveHistory(ctx, scope, current); err != nil {
				return proto.EdgeAgentResult{}, err
			}
			if err = cp.archiveTranscript(ctx, scope, current); err != nil {
				return proto.EdgeAgentResult{}, err
			}
			reset = current.Revision
		}
		reply, err = cp.edgeAgentLiveEnvelope(ctx, req, nil, reset)
	} else if req.Action == "resume" {
		if saved == nil {
			return proto.EdgeAgentResult{Version: 1, Status: "resume_unavailable"}, nil
		}
		snapshot, snapshotErr := cp.historySnapshot(saved)
		if snapshotErr != nil {
			return proto.EdgeAgentResult{}, snapshotErr
		}
		seed := &proto.EdgeAgentResume{Truncated: snapshot.Truncated, Window: snapshot.Window, SessionID: req.SessionID, Messages: snapshot.Messages, Activities: snapshot.Activities, Title: snapshot.Title, Model: snapshot.Model, StartedAt: snapshot.StartedAt, Revision: snapshot.Revision}
		reply, err = cp.edgeAgentLiveEnvelope(ctx, req, seed)
	} else {
		reply, err = cp.edgeAgentLive(ctx, req)
	}
	if err != nil {
		return reply, err
	}
	// Check permission again before storing or exposing a delayed remote reply.
	if _, err = cp.historyScope(ctx, req); err != nil {
		return proto.EdgeAgentResult{}, err
	}
	if (req.Action == "discard" || req.Action == "delete") && reply.Status == "ok" && saved != nil {
		if err = cp.repo.MutateEdgeAgentHistory(ctx, saved, true); err != nil {
			return proto.EdgeAgentResult{}, err
		}
	}
	if reply.SessionID != "" {
		scope.SessionID = reply.SessionID
		if req.Action != "watch" || !reply.Running {
			if err = cp.saveHistory(ctx, scope, reply); err != nil {
				return proto.EdgeAgentResult{}, err
			}
		}
		reply.HistoryPersistent = true
		if saved != nil && len(saved.TitleOverride) > 0 {
			title, e := cp.openHistory(saved.TitleOverride, historyAAD(saved)+"/title")
			if e != nil {
				return proto.EdgeAgentResult{}, e
			}
			reply.Title = string(title)
		}
		return reply, nil
	}
	if saved != nil && (req.Action == "poll" || req.Action == "watch" || req.Action == "snapshot" || req.Action == "stop") && (reply.Status == "not_found" || reply.Status == "unavailable" || reply.Status == "upgrade_required") {
		out, err := cp.historySnapshot(saved)
		out.Archived = true
		out.Closed = true
		out.Running = false
		out.Status = "ok"
		return out, err
	}
	return reply, nil
}
func (cp *controlPlane) auditHistoryMutation(ctx context.Context, scope *model.EdgeAgentHistory, action string) {
	if err := cp.RecordManagementAudit(ctx, &ManagementAudit{UserID: scope.OwnerID, Module: "agent", Action: action, Resource: "connector/" + strconv.FormatUint(scope.EdgeID, 10), Method: "POST", Success: true, StatusCode: 200}); err != nil {
		klog.Warning("agent history management audit could not be recorded")
	}
}
func (cp *controlPlane) historyList(ctx context.Context, req proto.EdgeAgentRequest, scope *model.EdgeAgentHistory) (proto.EdgeAgentResult, error) {
	liveReq := req
	liveReq.HistoryPage = ""
	live, err := cp.edgeAgentLive(ctx, liveReq)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	visible := make(map[string]proto.AgentSessionSummary)
	for _, s := range live.Sessions {
		visible[s.SessionID] = s
		key := *scope
		key.SessionID = s.SessionID
		if _, err := cp.repo.GetEdgeAgentHistory(ctx, &key); errors.Is(err, gorm.ErrRecordNotFound) {
			fetch := req
			fetch.Action = "snapshot"
			fetch.HistoryPage = ""
			fetch.SessionID = s.SessionID
			out, err := cp.edgeAgentLive(ctx, fetch)
			if err != nil {
				return proto.EdgeAgentResult{}, err
			}
			if out.SessionID != "" {
				if err = cp.saveHistory(ctx, &key, out); err != nil {
					return proto.EdgeAgentResult{}, err
				}
			}
		} else if err != nil {
			return proto.EdgeAgentResult{}, err
		}
	}
	if _, err = cp.historyScope(ctx, req); err != nil {
		return proto.EdgeAgentResult{}, err
	}
	page := 1
	if req.HistoryPage != "" {
		page, _ = strconv.Atoi(req.HistoryPage)
	}
	rows, total, err := cp.repo.ListEdgeAgentHistories(ctx, scope, page)
	if err != nil {
		return proto.EdgeAgentResult{}, err
	}
	out := proto.EdgeAgentResult{Version: 1, Status: "ok", HistoryPersistent: true, HistoryTotal: total, SessionsAvailable: true, SessionManagement: true, Sessions: []proto.AgentSessionSummary{}}
	for _, row := range rows {
		s, err := cp.historySnapshot(&row)
		if err != nil {
			return proto.EdgeAgentResult{}, err
		}
		item := proto.AgentSessionSummary{SessionID: row.SessionID, ThreadID: s.ThreadID, Title: s.Title, Project: s.Project, UpdatedAt: row.UpdatedAt.UTC().Format(time.RFC3339Nano), Closed: true, Status: "session_closed"}
		if current, ok := visible[row.SessionID]; ok {
			item.Closed = current.Closed
			item.Running = current.Running
			item.Status = current.Status
		}
		out.Sessions = append(out.Sessions, item)
	}
	return out, nil
}

// Lifecycle-owned synchronization continues when every browser is closed.
func (cp *controlPlane) RunAgentHistory(ctx context.Context) {
	if cp.historyCipher == nil {
		return
	}
	defer func() {
		if recover() != nil {
			klog.Error("agent history synchronizer stopped unexpectedly")
		}
	}()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		rows, err := cp.repo.PendingEdgeAgentHistories(ctx)
		if err != nil {
			klog.Warning("agent history synchronization unavailable")
			continue
		}
		var workers sync.WaitGroup
		// Bound concurrency; an unavailable connector must not serialize every snapshot.
		for worker := 0; worker < 4; worker++ {
			workers.Add(1)
			go func(offset int) {
				defer workers.Done()
				for i := offset; i < len(rows) && ctx.Err() == nil; i += 4 {
					cp.syncAgentHistory(ctx, rows[i])
				}
			}(worker)
		}
		workers.Wait()
	}
}
func (cp *controlPlane) syncAgentHistory(parent context.Context, row model.EdgeAgentHistory) {
	ctx, cancel := context.WithTimeout(context.WithValue(parent, "user_id", row.OwnerID), 3*time.Second)
	defer cancel()
	// Use a separate bounded context even if the remote request times out.
	// Rotating the checkpoint also prevents revoked/offline rows starving others.
	defer func() {
		checkpoint, done := context.WithTimeout(parent, time.Second)
		defer done()
		if err := cp.repo.TouchEdgeAgentHistory(checkpoint, &row, false); err != nil && parent.Err() == nil {
			klog.Warning("agent history synchronization checkpoint failed")
		}
	}()
	req := proto.EdgeAgentRequest{Action: "snapshot", EdgeID: row.EdgeID, AccessID: row.AccessID, SessionID: row.SessionID}
	if _, err := cp.historyScope(ctx, req); err != nil {
		return
	}
	out, err := cp.edgeAgentLive(ctx, req)
	if err == nil && out.SessionID != "" {
		if _, err = cp.historyScope(ctx, req); err == nil {
			err = cp.saveHistory(ctx, &row, out)
		}
	}
	if err != nil {
		klog.Warning("agent history snapshot synchronization failed")
	}
	if out.Status == "not_found" {
		if err := cp.repo.TouchEdgeAgentHistory(ctx, &row, true); err != nil {
			klog.Warning("agent history synchronization checkpoint failed")
		}
	}
}
