package web

import (
	"context"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/application"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type routingAgent struct {
	fakeAgentService
	session string
}

func (s *routingAgent) RunTurn(_ context.Context, r application.RunTurnRequest) (runtime.RunResult, error) {
	s.session = r.SessionID
	return runtime.RunResult{}, nil
}
func (s *routingAgent) GetSession(_ context.Context, _ *model.User, id string) (application.SessionDetail, error) {
	s.session = id
	return application.SessionDetail{}, nil
}
func TestAgentRoutesReadKratosPathParameters(t *testing.T) {
	resolved := application.ResolveApprovalRequest{}
	service := &routingAgent{fakeAgentService: fakeAgentService{resolved: &resolved}}
	web := &web{agentService: service}
	server := kratoshttp.NewServer()
	server.HandleFunc("/api/v1/agent/sessions/{id}", web.handleAgentSessionHTTP)
	server.HandleFunc("/api/v1/agent/sessions/{id}/turns", web.handleAgentTurnHTTP)
	server.HandleFunc("/api/v1/agent/sessions/{id}/approvals/{approval_id}", web.handleAgentApprovalHTTP)
	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/api/v1/agent/sessions/session-1", ""},
		{"POST", "/api/v1/agent/sessions/session-1/turns", `{"prompt":"hello"}`},
		{"POST", "/api/v1/agent/sessions/session-1/approvals/approval-1", `{"decision":"approve"}`},
	} {
		request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
		request = request.WithContext(context.WithValue(request.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		require.Equal(t, http.StatusOK, recorder.Code, recorder.Body.String())
	}
	require.Equal(t, "session-1", service.session)
	require.Equal(t, "session-1", resolved.SessionID)
	require.Equal(t, "approval-1", resolved.ApprovalID)
}
