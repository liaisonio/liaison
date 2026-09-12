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
