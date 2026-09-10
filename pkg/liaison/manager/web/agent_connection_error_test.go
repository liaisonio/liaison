package web

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/application"
	"github.com/stretchr/testify/require"
)

func TestAgentConnectionError_ReconnectMessage(t *testing.T) {
	for _, err := range []error{application.ErrConnectionUnavailable, accesssession.ErrHandleNotFound, accesssession.ErrHandleMismatch} {
		recorder := httptest.NewRecorder()
		writeAgentError(recorder, fmt.Errorf("private internal detail: %w", err))
		require.Equal(t, http.StatusConflict, recorder.Code)
		require.Contains(t, recorder.Body.String(), "reconnect and start a new agent session")
		require.NotContains(t, recorder.Body.String(), "private internal detail")
	}
}
