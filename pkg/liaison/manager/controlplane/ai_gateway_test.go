package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/aigateway"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

func TestAIGateway_RealDAOIsolationRevocationAndTargetBinding(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	admin, one, two := seedResourceScopeUsers(t, r)
	auth, err := iam.NewIAMService(r)
	require.NoError(t, err)
	encryption := make([]byte, 32)
	_, err = rand.Read(encryption)
	require.NoError(t, err)
	s, err := cp.NewAIService(auth, encryption)
	require.NoError(t, err)
	edge, app := createTestEdgeApplication(t, r)
	app.ApplicationType = model.ApplicationTypeLLM
	require.NoError(t, r.UpdateApplication(app))
	require.NoError(t, claimResource(one, r, resourceConnector, uint64(edge.ID)))
	require.NoError(t, claimResource(one, r, resourceApplication, uint64(app.ID)))
	p := &model.Proxy{ApplicationID: app.ID, Name: "AI test", Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolAI}
	require.NoError(t, r.CreateProxy(p))
	require.NoError(t, claimResource(one, r, resourceAccess, uint64(p.ID)))
	_, err = s.GetApplication(context.Background(), app.ID)
	require.ErrorIs(t, err, iam.ErrForbidden)
	_, err = s.GetApplication(two, app.ID)
	require.Error(t, err)
	_, err = s.SaveApplication(one, app.ID, AIApplicationConfig{Protocol: "openai-compatible", BasePath: "/v1", APIKey: "fixture-not-a-real-secret"})
	require.NoError(t, err)
	stored, err := r.GetAIApplication(one, app.ID)
	require.NoError(t, err)
	require.NotContains(t, stored.EncryptedKey, "fixture")
	view, err := s.GetApplication(admin, app.ID)
	require.NoError(t, err)
	require.True(t, view.HasAPIKey)
	require.Empty(t, view.APIKey)
	_, err = s.SaveAccess(one, p.ID, AIAccessConfig{Enabled: true, Models: map[string]string{"chat": "internal"}})
	require.NoError(t, err)
	workspace, err := s.Workspace(one, p.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"chat"}, workspace.Models)
	require.True(t, workspace.Enabled)
	require.Equal(t, "openai-compatible", workspace.UpstreamProtocol)
	require.Equal(t, "openai-compatible", workspace.ExternalProtocol)
	require.Equal(t, []string{"openai-compatible"}, workspace.ExternalProtocols)
	_, err = s.SaveApplication(one, app.ID, AIApplicationConfig{Protocol: "anthropic", BasePath: "/v1", APIKey: "fixture-not-a-real-secret"})
	require.NoError(t, err)
	nativeWorkspace, err := s.Workspace(one, p.ID)
	require.NoError(t, err)
	require.Equal(t, []string{"openai-compatible", "anthropic"}, nativeWorkspace.ExternalProtocols)
	_, err = s.SaveApplication(one, app.ID, AIApplicationConfig{Protocol: "openai-compatible", BasePath: "/v1", APIKey: "fixture-not-a-real-secret"})
	require.NoError(t, err)
	publicJSON, err := json.Marshal(workspace)
	require.NoError(t, err)
	require.NotContains(t, string(publicJSON), "internal")
	require.NotContains(t, string(publicJSON), "fixture")
	_, err = s.Workspace(two, p.ID)
	require.Error(t, err)
	_, err = s.Workspace(context.Background(), p.ID)
	require.ErrorIs(t, err, iam.ErrForbidden)
	key, err := s.CreateKey(one, p.ID, AIKeyRequest{Name: "client", Models: []string{"chat"}, ExpiresInDays: 1})
	require.NoError(t, err)
	listed, err := s.Keys(one, p.ID)
	require.NoError(t, err)
	require.Len(t, listed, 1)
	require.Empty(t, listed[0].Secret)
	_, err = s.Keys(two, p.ID)
	require.Error(t, err)
	grant, err := s.Grant(context.Background(), p.ID, key.Secret)
	require.NoError(t, err)
	grant.Upstream.Close()
	require.Equal(t, map[string]string{"chat": "internal"}, grant.Models)
	_, err = s.Grant(context.Background(), p.ID+1, key.Secret)
	require.Error(t, err)
	_, err = s.SaveAccess(one, p.ID, AIAccessConfig{Enabled: true, Models: map[string]string{"other": "internal"}})
	require.NoError(t, err)
	grant, err = s.Grant(context.Background(), p.ID, key.Secret)
	require.NoError(t, err)
	grant.Upstream.Close()
	require.Empty(t, grant.Models)
	require.NoError(t, s.RevokeKey(one, p.ID, key.ID))
	_, err = s.Grant(context.Background(), p.ID, key.Secret)
	require.Error(t, err)
	expiredSecret, digest, err := aigateway.NewKey()
	require.NoError(t, err)
	uid, _ := actorUserID(one)
	input, output := int64(12), int64(4)
	require.NoError(t, s.Record(one, &model.AIRequest{RequestID: "usage-fixture", ProxyID: p.ID, UserID: uid, KeyID: key.ID, Model: "chat", InputTokens: &input, OutputTokens: &output, Complete: true}))
	usage, err := s.TokenUsage(one, p.ID)
	require.NoError(t, err)
	require.EqualValues(t, 1, usage.Summary.Requests)
	require.Equal(t, input, *usage.Summary.InputTokens)
	require.Equal(t, key.ID, usage.Records[0].KeyID) // Revoked keys retain history.
	homeView, homeErr := cp.ManagementLLMOverview(one, p.ID, 24)
	require.NoError(t, homeErr)
	require.EqualValues(t, 1, homeView.Usage.Requests)
	require.Equal(t, []string{"other"}, homeView.Models)
	homeJSON, homeErr := json.Marshal(homeView)
	require.NoError(t, homeErr)
	require.NotContains(t, string(homeJSON), "internal")
	require.NotContains(t, string(homeJSON), key.Secret)
	_, homeErr = cp.ManagementLLMOverview(two, p.ID, 24)
	require.Error(t, homeErr)
	_, homeErr = cp.ManagementLLMOverview(context.Background(), p.ID, 24)
	require.ErrorIs(t, homeErr, iam.ErrForbidden)
	for _, hours := range []int{1, 6, 24, 168, 720} {
		window, windowErr := s.TokenUsage(one, p.ID, hours)
		require.NoError(t, windowErr)
		require.EqualValues(t, 1, window.Summary.Requests)
		require.WithinDuration(t, time.Now().UTC().Add(-time.Duration(hours)*time.Hour), window.Since, 2*time.Second)
	}
	for _, hours := range []int{-1, 0, 2, 721} {
		_, windowErr := s.TokenUsage(one, p.ID, hours)
		require.ErrorIs(t, windowErr, ErrAIInvalid)
	}
	_, err = s.TokenUsage(two, p.ID)
	require.Error(t, err)
	_, err = s.TokenUsage(context.Background(), p.ID)
	require.ErrorIs(t, err, iam.ErrForbidden)
	adminUsage, err := s.TokenUsage(admin, p.ID)
	require.NoError(t, err)
	require.Zero(t, adminUsage.Summary.Requests) // Management access is not ownership.
	require.NoError(t, r.CreateAIKey(one, &model.AIKey{ProxyID: p.ID, UserID: uid, Name: "expired", Digest: digest, Models: `["other"]`, ExpiresAt: time.Now().Add(-time.Hour)}))
	_, err = s.Grant(context.Background(), p.ID, expiredSecret)
	require.Error(t, err)
	// Target changes must not forward a credential authorized for the old target.
	app.Port++
	require.NoError(t, r.UpdateApplication(app))
	_, err = s.SaveApplication(one, app.ID, view)
	require.ErrorIs(t, err, ErrAIInvalid)
	view.ClearKey = true
	_, err = s.SaveApplication(one, app.ID, view)
	require.NoError(t, err)
	key, err = s.CreateKey(one, p.ID, AIKeyRequest{Name: "next", Models: []string{"other"}, ExpiresInDays: 1})
	require.NoError(t, err)
	p.Status = model.ProxyStatusStopped
	require.NoError(t, r.UpdateProxy(p))
	_, err = s.Grant(context.Background(), p.ID, key.Secret)
	require.ErrorIs(t, err, ErrAIUnavailable)
	p.Status = model.ProxyStatusRunning
	require.NoError(t, r.UpdateProxy(p))
	user, err := r.GetUserByID(uid)
	require.NoError(t, err)
	user.Status = model.UserStatusInactive
	require.NoError(t, r.UpdateUser(user))
	_, err = s.Grant(context.Background(), p.ID, key.Secret)
	require.ErrorIs(t, err, iam.ErrForbidden)
	encoded, err := json.Marshal(stored)
	require.NoError(t, err)
	require.NotContains(t, string(encoded), stored.EncryptedKey)
}

func TestNativeApplicationProtocolBinding(t *testing.T) {
	for _, protocol := range []string{"openai", "anthropic", "ark", "qwen", "gemini", "ollama", "openai-compatible"} {
		t.Run(protocol, func(t *testing.T) {
			cp, r := newTestControlPlane(t)
			t.Cleanup(func() { require.NoError(t, r.Close()) })
			_, owner, other := seedResourceScopeUsers(t, r)
			auth, err := iam.NewIAMService(r)
			require.NoError(t, err)
			service, err := cp.NewAIService(auth, make([]byte, 32))
			require.NoError(t, err)
			edge, app := createTestEdgeApplication(t, r)
			app.ApplicationType = model.ApplicationType(protocol)
			require.NoError(t, r.UpdateApplication(app))
			require.NoError(t, claimResource(owner, r, resourceConnector, uint64(edge.ID)))
			require.NoError(t, claimResource(owner, r, resourceApplication, uint64(app.ID)))
			config, err := service.GetApplication(owner, app.ID)
			require.NoError(t, err)
			expected := protocol
			if protocol == "openai" {
				expected = "openai-compatible"
			}
			require.Equal(t, expected, config.Protocol)
			require.Equal(t, protocol, config.ApplicationType)
			_, err = service.SaveApplication(other, app.ID, config)
			require.Error(t, err)
			_, err = service.SaveApplication(owner, app.ID, AIApplicationConfig{Protocol: "invalid"})
			require.Error(t, err)
			config.APIKey = "test-fixture-not-a-real-key"
			_, err = service.SaveApplication(owner, app.ID, config)
			require.NoError(t, err)
			proxy := &model.Proxy{ApplicationID: app.ID, Name: "Native fixture", Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolAI}
			require.NoError(t, r.CreateProxy(proxy))
			require.NoError(t, claimResource(owner, r, resourceAccess, uint64(proxy.ID)))
			access, err := service.SaveAccess(owner, proxy.ID, AIAccessConfig{Enabled: true, Models: map[string]string{"public": "private"}, ExternalProtocol: aigateway.NativeClientProtocols(expected)[0]})
			require.NoError(t, err)
			require.Equal(t, aigateway.NativeClientProtocols(expected)[0], access.ExternalProtocol)
			key, err := service.CreateKey(owner, proxy.ID, AIKeyRequest{Name: "fixture", Models: []string{"public"}, ExpiresInDays: 1})
			require.NoError(t, err)
			grant, err := service.Grant(context.Background(), proxy.ID, key.Secret)
			require.NoError(t, err)
			require.Equal(t, expected, grant.Protocol)
			grant.Upstream.Close()
			if aiOpenAIProfile(protocol) {
				for _, profile := range []string{"openai", "openai-compatible"} {
					config.Protocol = profile
					config.APIKey = ""
					saved, e := service.SaveApplication(owner, app.ID, config)
					require.NoError(t, e)
					require.True(t, saved.HasAPIKey)
					require.Empty(t, saved.APIKey)
					next, e := service.Grant(context.Background(), proxy.ID, key.Secret)
					require.NoError(t, e)
					require.Equal(t, profile, next.Protocol)
					require.Equal(t, "test-fixture-not-a-real-key", next.UpstreamKey)
					next.Upstream.Close()
				}
				app.Port++
				require.NoError(t, r.UpdateApplication(app))
				config.Protocol = "openai"
				_, e := service.SaveApplication(owner, app.ID, config)
				require.ErrorIs(t, e, ErrAIInvalid)
			}
			require.NoError(t, service.RevokeKey(owner, proxy.ID, key.ID))
			_, err = service.Grant(context.Background(), proxy.ID, key.Secret)
			require.Error(t, err)
		})
	}
}
