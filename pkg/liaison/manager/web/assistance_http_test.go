package web

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeAssistanceService struct {
	fakeAgentService
	calls int
	err   error
}

func (s *fakeAssistanceService) Suggest(_ context.Context, actor *model.User, handle, editor string, input assistance.Input, closeSession bool) (assistance.Suggestion, error) {
	s.calls++
	if s.err != nil {
		return assistance.Suggestion{}, s.err
	}
	return assistance.Suggestion{Revision: input.Revision, Cursor: input.Cursor, Text: " -h"}, nil
}

func TestAssistanceHTTP_ModelFailureIsNotBadRequest(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{{assistance.ErrSuggestion, 502}, {assistance.ErrInvalid, 400}} {
		service := &fakeAssistanceService{err: tc.err}
		web := &web{agentService: service}
		r := httptest.NewRequest(http.MethodPost, "/api/v1/assistance/suggestions", strings.NewReader(`{"handle_id":"live","editor_id":"editor","revision":1}`))
		r = r.WithContext(context.WithValue(r.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
		w := httptest.NewRecorder()
		web.handleAssistanceHTTP(w, r)
		require.Equal(t, tc.status, w.Code)
	}
}
func TestAssistanceHTTP_AuthenticatedAndStrictInput(t *testing.T) {
	for _, tc := range []struct {
		name, body string
		auth       bool
		want       int
	}{
		{"unauthenticated", `{}`, false, 401},
		{"unknown context", `{"handle_id":"live","messages":[]}`, true, 400},
		{"trailing json", `{} {}`, true, 400},
		{"suggest", `{"handle_id":"live","editor_id":"editor-123","revision":1,"text":"du","cursor":2}`, true, 200},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service := &fakeAssistanceService{}
			web := &web{agentService: service}
			r := httptest.NewRequest(http.MethodPost, "/api/v1/assistance/suggestions", strings.NewReader(tc.body))
			if tc.auth {
				r = r.WithContext(context.WithValue(r.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
			}
			w := httptest.NewRecorder()
			web.handleAssistanceHTTP(w, r)
			require.Equal(t, tc.want, w.Code)
			if tc.want == 200 {
				require.Contains(t, w.Body.String(), `"text":" -h"`)
			} else {
				require.Zero(t, service.calls)
			}
		})
	}
}
