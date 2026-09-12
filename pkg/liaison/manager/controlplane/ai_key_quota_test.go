package controlplane

import (
	"context"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
)

func TestAIKeyQuota_LifetimeIsolationAndAdjustment(t *testing.T) {
	cp, r := newTestControlPlane(t)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	admin, one, two := seedResourceScopeUsers(t, r)
	auth, err := iam.NewIAMService(r)
	require.NoError(t, err)
	s, err := cp.NewAIService(auth, make([]byte, 32))
	require.NoError(t, err)
	edge, app := createTestEdgeApplication(t, r)
	app.ApplicationType = model.ApplicationTypeLLM
	require.NoError(t, r.UpdateApplication(app))
	require.NoError(t, claimResource(one, r, resourceConnector, uint64(edge.ID)))
	require.NoError(t, claimResource(one, r, resourceApplication, uint64(app.ID)))
	p := &model.Proxy{ApplicationID: app.ID, Name: "quota fixture", Status: model.ProxyStatusRunning, AccessProtocol: model.AccessProtocolAI}
	require.NoError(t, r.CreateProxy(p))
	require.NoError(t, claimResource(one, r, resourceAccess, uint64(p.ID)))
	_, err = s.SaveAccess(one, p.ID, AIAccessConfig{Enabled: true, Models: map[string]string{"chat": "fixture"}})
	require.NoError(t, err)
	limit := int64(100)
	k, err := s.CreateKey(one, p.ID, AIKeyRequest{Name: "limited", Models: []string{"chat"}, ExpiresInDays: 1, TokenLimit: &limit})
	require.NoError(t, err)
	uid, _ := actorUserID(one)
	g := &AIGrant{KeyID: k.ID, UserID: uid, ProxyID: p.ID}
	require.NoError(t, s.CheckKeyQuota(one, g))
	in, out := int64(60), int64(40)
	record := &model.AIRequest{RequestID: "quota-old", UserID: uid, ProxyID: p.ID, KeyID: k.ID, Model: "chat", InputTokens: &in, OutputTokens: &out, Complete: true, CreatedAt: time.Now().Add(-45 * 24 * time.Hour)}
	require.NoError(t, s.Record(one, record))
	require.NoError(t, s.Record(one, record))
	require.ErrorIs(t, s.CheckKeyQuota(one, g), ErrAITokenQuota)
	keys, err := s.Keys(one, p.ID)
	require.NoError(t, err)
	require.EqualValues(t, 100, keys[0].UsedTokens)
	require.EqualValues(t, 0, *keys[0].RemainingTokens)
	require.Error(t, s.UpdateKeyQuota(two, p.ID, k.ID, nil))
	require.Error(t, s.UpdateKeyQuota(admin, p.ID, k.ID, nil)) // Not this key's owner.
	require.Error(t, s.UpdateKeyQuota(context.Background(), p.ID, k.ID, nil))
	limit = 200
	require.NoError(t, s.UpdateKeyQuota(one, p.ID, k.ID, &limit))
	require.NoError(t, s.CheckKeyQuota(one, g))
	require.NoError(t, s.Record(one, &model.AIRequest{RequestID: "quota-unknown", UserID: uid, ProxyID: p.ID, KeyID: k.ID, Model: "chat"}))
	require.ErrorIs(t, s.CheckKeyQuota(one, g), ErrAITokenUsage)
	keys, err = s.Keys(one, p.ID)
	require.NoError(t, err)
	require.Nil(t, keys[0].RemainingTokens)
	require.EqualValues(t, 1, keys[0].UnknownRequests)
	require.NoError(t, s.UpdateKeyQuota(one, p.ID, k.ID, nil))
	require.NoError(t, s.CheckKeyQuota(one, g))
	require.NoError(t, s.CheckKeyQuota(one, &AIGrant{})) // Dashboard not assigned a key.
	for _, n := range []int64{-1, 1_000_000_000_001} {
		require.ErrorIs(t, s.UpdateKeyQuota(one, p.ID, k.ID, &n), ErrAIInvalid)
		_, err = s.CreateKey(one, p.ID, AIKeyRequest{Name: "invalid", Models: []string{"chat"}, ExpiresInDays: 1, TokenLimit: &n})
		require.ErrorIs(t, err, ErrAIInvalid)
	}
	limit = 0
	require.NoError(t, s.UpdateKeyQuota(one, p.ID, k.ID, &limit))
	require.ErrorIs(t, s.CheckKeyQuota(one, g), ErrAITokenQuota)
	require.NoError(t, s.RevokeKey(one, p.ID, k.ID))
	require.ErrorIs(t, s.CheckKeyQuota(one, g), iam.ErrForbidden)
}
