package web

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentapplication "github.com/liaisonio/liaison/pkg/liaison/manager/agent/application"
	agentruntime "github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestAgentSessionsHTTP_RejectsClientAuthoredBindingFields(t *testing.T) {
	handler := &web{agentService: fakeAgentService{}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions", bytes.NewBufferString(
		`{"handle_id":"live","organization_id":99}`,
	))
	request = request.WithContext(context.WithValue(request.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
	recorder := httptest.NewRecorder()

	handler.handleAgentSessionsHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "invalid agent request")
}

func TestAgentSessionsHTTP_CreatesSessionFromOpaqueHandle(t *testing.T) {
	service := fakeAgentService{create: agentapplication.SessionDetail{Session: agentruntime.Session{ID: "session-1"}}}
	handler := &web{agentService: service}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions", bytes.NewBufferString(
		`{"handle_id":"live","title":"Investigate"}`,
	))
	request = request.WithContext(context.WithValue(request.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
	recorder := httptest.NewRecorder()

	handler.handleAgentSessionsHTTP(recorder, request)

	assert.Equal(t, http.StatusCreated, recorder.Code)
	assert.Contains(t, recorder.Body.String(), "session-1")
}

func TestWriteAgentSSE_UsesSequenceAndEventType(t *testing.T) {
	recorder := httptest.NewRecorder()
	err := writeAgentSSE(recorder, agentruntime.Event{
		Sequence: 4, SessionID: "session-1", Type: agentruntime.EventModelDelta,
		Payload: []byte(`{"delta":"ok"}`), Occurred: time.Unix(1, 0).UTC(),
	})
	require.NoError(t, err)
	body := recorder.Body.String()
	assert.True(t, strings.HasPrefix(body, "id: 4\nevent: model.delta\n"))
	assert.Contains(t, body, `"session_id":"session-1"`)
}

func TestAgentTurnHTTP_ReturnsRuntimeResult(t *testing.T) {
	service := fakeAgentService{run: agentruntime.RunResult{Turn: agentruntime.Turn{ID: "turn-1"}, Text: "done"}}
	handler := &web{agentService: service}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/session-1/turns", bytes.NewBufferString(`{"prompt":"inspect"}`))
	request.SetPathValue("id", "session-1")
	request = request.WithContext(context.WithValue(request.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
	recorder := httptest.NewRecorder()

	handler.handleAgentTurnHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.Contains(t, recorder.Body.String(), `"id":"turn-1"`)
	assert.Contains(t, recorder.Body.String(), `"text":"done"`)
}

type fakeAgentService struct {
	enabled  bool
	create   agentapplication.SessionDetail
	run      agentruntime.RunResult
	resolved *agentapplication.ResolveApprovalRequest
}

func (service fakeAgentService) Available() bool { return service.enabled }

func TestAgentStatusHTTP(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		handler, admin, _ := newPermissionHTTPTest(t)
		handler.agentService = fakeAgentService{enabled: enabled}
		request := httptest.NewRequest(http.MethodGet, "/api/v1/agent/status", nil)
		recorder := httptest.NewRecorder()
		handler.handleAgentStatusHTTP(recorder, request)
		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
		request = request.WithContext(context.WithValue(request.Context(), "user", admin))
		recorder = httptest.NewRecorder()
		handler.handleAgentStatusHTTP(recorder, request)
		assert.Equal(t, http.StatusOK, recorder.Code)
		if enabled {
			assert.JSONEq(t, `{"code":200,"data":{"enabled":true,"models":[]}}`, recorder.Body.String())
		} else {
			assert.JSONEq(t, `{"code":200,"data":{"enabled":false,"models":[]}}`, recorder.Body.String())
		}
	}
}

func (service fakeAgentService) CreateSession(context.Context, agentapplication.CreateSessionRequest) (agentapplication.SessionDetail, error) {
	return service.create, nil
}

func (fakeAgentService) ListSessions(context.Context, *model.User) ([]agentruntime.Session, error) {
	return nil, nil
}

func (fakeAgentService) GetSession(context.Context, *model.User, string) (agentapplication.SessionDetail, error) {
	return agentapplication.SessionDetail{}, nil
}

func (fakeAgentService) ArchiveSession(context.Context, *model.User, string, uint64) (agentruntime.Session, error) {
	return agentruntime.Session{}, nil
}

func (service fakeAgentService) RunTurn(context.Context, agentapplication.RunTurnRequest) (agentruntime.RunResult, error) {
	return service.run, nil
}

func (service fakeAgentService) ResolveApproval(_ context.Context, request agentapplication.ResolveApprovalRequest) (agentruntime.RunResult, error) {
	if service.resolved != nil {
		*service.resolved = request
	}
	return service.run, nil
}

func TestAgentApprovalHTTP_RequiresExplicitDecision(t *testing.T) {
	handler := &web{agentService: fakeAgentService{}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/session-1/approvals/approval-1", bytes.NewBufferString(`{}`))
	request.SetPathValue("id", "session-1")
	request.SetPathValue("approval_id", "approval-1")
	request = request.WithContext(context.WithValue(request.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
	recorder := httptest.NewRecorder()

	handler.handleAgentApprovalHTTP(recorder, request)

	assert.Equal(t, http.StatusBadRequest, recorder.Code)
}

func TestAgentApprovalHTTP_MapsDecisionWithoutAcceptingToolInput(t *testing.T) {
	var resolved agentapplication.ResolveApprovalRequest
	handler := &web{agentService: fakeAgentService{resolved: &resolved}}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/agent/sessions/session-1/approvals/approval-1", bytes.NewBufferString(`{"decision":"approve","note":"checked"}`))
	request.SetPathValue("id", "session-1")
	request.SetPathValue("approval_id", "approval-1")
	request = request.WithContext(context.WithValue(request.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
	recorder := httptest.NewRecorder()

	handler.handleAgentApprovalHTTP(recorder, request)

	assert.Equal(t, http.StatusOK, recorder.Code)
	assert.True(t, resolved.Approve)
	assert.Equal(t, "checked", resolved.Note)
}
