package web

import (
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/stretchr/testify/require"
	"net/http/httptest"
	"testing"
)

func TestAISetupRoutingRequiresAuthentication(t *testing.T) {
	server := &web{aiGateway: &controlplane.AIService{}}
	for _, tc := range []struct {
		path, method string
		status       int
	}{
		{"/api/v1/ai/setup", "POST", 401},
		{"/api/v1/ai/setup/probe", "POST", 401},
		{"/api/v1/ai/setup?api_key=secret", "POST", 400},
		{"/api/v1/ai/setup/probe/extra", "POST", 400},
		{"/api/v1/ai/setup", "GET", 400},
	} {
		t.Run(tc.path+tc.method, func(t *testing.T) {
			out := httptest.NewRecorder()
			server.handleAIGatewayHTTP(out, httptest.NewRequest(tc.method, tc.path, nil))
			require.Equal(t, tc.status, out.Code)
			require.NotContains(t, out.Body.String(), "secret")
			require.Equal(t, "no-store", out.Header().Get("Cache-Control"))
		})
	}
}
