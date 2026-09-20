package web

import (
	"context"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type modelAgentService struct {
	fakeAgentService
	actor     uint
	selection runtime.ModelSelection
}

func (s *modelAgentService) SetSessionModel(_ context.Context, actor *model.User, id string, version uint64, selection runtime.ModelSelection) (runtime.Session, error) {
	s.actor = actor.ID
	s.selection = selection
	return runtime.Session{ID: id, Version: version + 1, ModelSelection: selection}, nil
}
func TestAgentModelHTTPContract(t *testing.T) {
	for _, body := range []string{`{"version":1,"model_selection":{}}`, `{"version":1,"model_selection":{"provider_id":"one","model":"two"}}`, `{"version":1}`, `{"version":1,"model_selection":{"api_key":"secret"}}`, `{"version":1,"model_selection":{},"created_by":99}`} {
		service := &modelAgentService{}
		handler := &web{agentService: service}
		r := httptest.NewRequest(http.MethodPatch, "/api/v1/agent/sessions/one", strings.NewReader(body))
		r.SetPathValue("id", "one")
		r = r.WithContext(context.WithValue(r.Context(), "user", &model.User{Model: gorm.Model{ID: 7}}))
		w := httptest.NewRecorder()
		handler.handleAgentSessionHTTP(w, r)
		if strings.Contains(body, "api_key") || strings.Contains(body, "created_by") || body == `{"version":1}` {
			require.Equal(t, 400, w.Code)
			require.Zero(t, service.actor)
		} else {
			require.Equal(t, 200, w.Code)
			require.Equal(t, uint(7), service.actor)
		}
	}
}
