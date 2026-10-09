package web

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/proto"
)

type agentRequestRecord struct {
	Service   string `json:"service"`
	Event     string `json:"event"`
	TraceID   string `json:"trace_id"`
	SessionID string `json:"session_id,omitempty"`
	Window    uint64 `json:"window"`
	Action    string `json:"action"`
	ElapsedMS int64  `json:"elapsed_ms"`
	Outcome   string `json:"outcome"`
}

func agentRequestDiagnostic(req proto.EdgeAgentRequest, snapshot proto.EdgeAgentResult, err error, elapsed time.Duration) (agentRequestRecord, bool) {
	switch req.Action {
	case "start", "resume", "send", "interrupt", "stop":
	default:
		return agentRequestRecord{}, false // Never log watch/poll/token traffic.
	}
	id := snapshot.SessionID
	if id == "" {
		id = req.SessionID
	}
	if _, decodeErr := hex.DecodeString(id); decodeErr != nil || len(id) != 32 {
		id = ""
	}
	outcome := "rejected"
	if snapshot.Status == "ok" || snapshot.Status == "session_closed" {
		outcome = "ok"
	}
	if err != nil {
		outcome = "failed"
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		outcome = "canceled"
	}
	return agentRequestRecord{Service: "liaison-manager", Event: "agent_request_completed", TraceID: rand.Text(), SessionID: id, Window: snapshot.Window, Action: req.Action, ElapsedMS: max(int64(0), elapsed.Milliseconds()), Outcome: outcome}, true
}

func recordAgentRequest(req proto.EdgeAgentRequest, snapshot proto.EdgeAgentResult, err error, elapsed time.Duration) {
	record, ok := agentRequestDiagnostic(req, snapshot, err, elapsed)
	if !ok {
		return
	}
	data, encodeErr := json.Marshal(record)
	if encodeErr != nil {
		log.Warn("agent request diagnostic encoding failed")
		return
	}
	log.Info(string(data))
}
