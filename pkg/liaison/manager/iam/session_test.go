package iam

import (
	"context"
	"github.com/golang-jwt/jwt/v5"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/utils"
	"github.com/stretchr/testify/require"
	"path/filepath"
	"testing"
	"time"
)

func TestSessionRevocationPersistsAndIsIsolated(t *testing.T) {
	old := utils.DefaultJWTConfig.SecretKey
	require.NoError(t, utils.SetJWTSecret("session-test-only-key-at-least-32-bytes"))
	t.Cleanup(func() { utils.DefaultJWTConfig.SecretKey = old })
	conf := &config.Configuration{Manager: config.Manager{DB: filepath.Join(t.TempDir(), "sessions.db")}}
	r, err := repo.NewRepo(conf)
	require.NoError(t, err)
	s := &IAMService{repo: r}
	a, err := utils.GenerateToken(1, "test@example.test")
	require.NoError(t, err)
	b, err := utils.GenerateToken(1, "test@example.test")
	require.NoError(t, err)
	require.NotEqual(t, a, b, "same-second logins must have independent session IDs")
	_, err = s.ValidateToken(a)
	require.NoError(t, err)
	require.NoError(t, s.RevokeSessionToken(context.Background(), a))
	require.NoError(t, s.RevokeSessionToken(context.Background(), a), "idempotent storage")
	_, err = s.ValidateToken(a)
	require.Error(t, err)
	_, err = s.ValidateToken(b)
	require.NoError(t, err)
	require.Error(t, s.RevokeSessionToken(context.Background(), "invalid"))
	legacy := &utils.Claims{UserID: 1, RegisteredClaims: jwt.RegisteredClaims{ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour))}}
	legacyToken, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString([]byte(utils.DefaultJWTConfig.SecretKey))
	require.NoError(t, err)
	require.NoError(t, s.RevokeSessionToken(context.Background(), legacyToken))
	require.NoError(t, r.Close())
	r, err = repo.NewRepo(conf)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, r.Close()) })
	s = &IAMService{repo: r}
	_, err = s.ValidateToken(a)
	require.Error(t, err)
	_, err = s.ValidateToken(legacyToken)
	require.Error(t, err)
	_, err = s.ValidateToken(b)
	require.NoError(t, err)
	require.NoError(t, r.Close())
	_, err = s.ValidateToken(b)
	require.Error(t, err, "storage failure must fail closed")
}
