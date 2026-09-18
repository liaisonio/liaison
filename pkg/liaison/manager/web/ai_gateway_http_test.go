package web

import (
	"context"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/aigateway"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

func TestFinalizeAIRequestPreservesUsageAndClassifiesInterruption(t *testing.T) {
	for _, tc := range []struct {
		name                      string
		interruption              error
		initialStatus, wantStatus int
		complete, wantComplete    bool
	}{
		{"success", nil, 200, 200, true, true},
		{"upstream failure", nil, 502, 502, false, false},
		{"timeout", context.DeadlineExceeded, 504, 504, false, false},
		{"stream timeout", context.DeadlineExceeded, 502, 504, false, false},
		{"canceled", context.Canceled, 502, 499, false, false},
		{"canceled at completion", context.Canceled, 200, 499, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.interruption == context.DeadlineExceeded {
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Now().Add(-time.Second))
				defer cancel()
			} else if tc.interruption == context.Canceled {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			input := int64(12)
			record := &model.AIRequest{Status: tc.initialStatus}
			finalizeAIRequest(ctx, record, aigateway.Usage{Input: &input, Complete: tc.complete}, 25*time.Millisecond)
			require.Equal(t, tc.wantStatus, record.Status)
			require.Equal(t, tc.wantComplete, record.Complete)
			require.EqualValues(t, 12, *record.InputTokens)
			require.Nil(t, record.OutputTokens, "unknown usage must not become zero")
			require.EqualValues(t, 25, record.DurationMS)
		})
	}
}

func TestAIDecodeBoundsAndStrictConfig(t *testing.T) {
	for _, body := range []string{`{"enabled":true} {}`, `{"enabled":true,"unknown":true}`, `{"enabled":"true"}`, strings.Repeat(" ", 1<<20) + `{}`} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("PUT", "/api/v1/ai/accesses/1", strings.NewReader(body))
		var c controlplane.AIAccessConfig
		require.ErrorIs(t, aiDecode(w, r, &c), controlplane.ErrAIInvalid)
	}
	w := httptest.NewRecorder()
	r := httptest.NewRequest("PUT", "/api/v1/ai/accesses/1", strings.NewReader(`{"enabled":true,"models":{"chat":"internal"}}`))
	var c controlplane.AIAccessConfig
	require.NoError(t, aiDecode(w, r, &c))
	require.True(t, c.Enabled)
}

func TestAIErrorsNeverExposeUnderlyingDetails(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
	}{
		{iam.ErrForbidden, 403}, {controlplane.ErrAIInvalid, 400},
		{controlplane.ErrAIUnavailable, 503}, {aigateway.ErrModelDenied, 403},
		{aigateway.ErrUnsupported, 400}, {errors.New("secret upstream credential and prompt"), 500},
	} {
		w := httptest.NewRecorder()
		require.Equal(t, tc.status, aiStatus(tc.err))
		aiError(w, aiStatus(tc.err))
		require.NotContains(t, w.Body.String(), "credential")
		require.NotContains(t, w.Body.String(), "prompt")
	}
}

func TestAIGatewayUnconfiguredFailsClosed(t *testing.T) {
	w := httptest.NewRecorder()
	(&web{}).handleAIGatewayHTTP(w, httptest.NewRequest("GET", "/api/v1/ai/accesses/1/v1/models", nil))
	require.Equal(t, 503, w.Code)
	require.Equal(t, "no-store", w.Header().Get("Cache-Control"))
}

func TestNativeOllamaRoutesRequireKeyAndRejectLifecycleOperations(t *testing.T) {
	server := &web{aiGateway: &controlplane.AIService{}}
	for _, tc := range []struct {
		method, path string
		status       int
	}{
		{"GET", "api/tags", 401},
		{"POST", "api/chat", 401},
		{"POST", "api/pull", 405},
		{"POST", "api/create", 405},
		{"DELETE", "api/delete", 405},
		{"GET", "api/ps", 405},
		{"GET", "api/tags?key=fixture", 400},
	} {
		t.Run(tc.method+"/"+tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			r := httptest.NewRequest(tc.method, "/api/v1/ai/accesses/1/"+tc.path, nil)
			r.Header.Set("Cookie", "session=not-an-api-key")
			server.handleAIGatewayHTTP(w, r)
			require.Equal(t, tc.status, w.Code)
		})
	}
}

func TestGeminiRoutesRejectQueryCredentialsAndRequireAuthentication(t *testing.T) {
	server := &web{aiGateway: &controlplane.AIService{}}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"v1beta/models/chat:generateContent", 401},
		{"v1beta/models/chat:streamGenerateContent?alt=sse", 401},
		{"v1beta/models/chat:generateContent?key=fixture", 400},
		{"v1beta/models/chat:generateContent?alt=sse", 400},
		{"v1beta/models/chat:streamGenerateContent?alt=sse&key=fixture", 400},
		{"v1beta/models/chat:streamGenerateContent?alt=sse&alt=sse", 400},
		{"v1beta/models/chat:delete", 405},
	} {
		t.Run(tc.path, func(t *testing.T) {
			w := httptest.NewRecorder()
			server.handleAIGatewayHTTP(w, httptest.NewRequest("POST", "/api/v1/ai/accesses/1/"+tc.path, nil))
			require.Equal(t, tc.status, w.Code)
		})
	}
}

func TestGeminiAuthenticationRejectsAmbiguousKeys(t *testing.T) {
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Set("x-goog-api-key", "fixture")
	key, ok := geminiInferenceKey(r)
	require.True(t, ok)
	require.Equal(t, "fixture", key)
	r.Header.Set("Authorization", "Bearer other")
	_, ok = geminiInferenceKey(r)
	require.False(t, ok)
	r.Header.Del("Authorization")
	r.Header.Add("x-goog-api-key", "other")
	_, ok = geminiInferenceKey(r)
	require.False(t, ok)
	r.Header.Del("x-goog-api-key")
	r.Header.Set("Authorization", "Bearer fixture")
	_, ok = geminiInferenceKey(r)
	require.True(t, ok)
	r.Header.Set("x-api-key", "other")
	_, ok = geminiInferenceKey(r)
	require.False(t, ok)
}

func TestAIErrorCarriesSafeReasonAndRequestID(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("X-Request-ID", "test-request")
	aiError(w, 502, "UPSTREAM_AUTHENTICATION_FAILED")
	require.Equal(t, 502, w.Code)
	require.Contains(t, w.Body.String(), "UPSTREAM_AUTHENTICATION_FAILED")
	require.Contains(t, w.Body.String(), "test-request")
}

func TestAIInferenceKeyRejectsAmbiguousCredentials(t *testing.T) {
	for _, tc := range []struct {
		name, key, authorization string
		native, want             bool
	}{
		{"native key", "test-key", "", true, true},
		{"not accepted on OpenAI", "test-key", "", false, false},
		{"ambiguous", "test-key", "Bearer other", true, false},
		{"whitespace", " test-key", "", true, false},
		{"bearer fallback", "", "Bearer test-key", true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/", nil)
			if tc.key != "" {
				r.Header.Set("x-api-key", tc.key)
			}
			if tc.authorization != "" {
				r.Header.Set("Authorization", tc.authorization)
			}
			_, ok := aiInferenceKey(r, tc.native)
			require.Equal(t, tc.want, ok)
		})
	}
	r := httptest.NewRequest("POST", "/", nil)
	r.Header.Add("x-api-key", "one")
	r.Header.Add("x-api-key", "two")
	_, ok := aiInferenceKey(r, true)
	require.False(t, ok)
}
