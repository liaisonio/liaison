package dao

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAIGatewayMigrationAndBoundedStorage(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "test.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	up, err := os.ReadFile("../../../../db/migrations/20260912010000_create_ai_gateway.up.sql")
	require.NoError(t, err)
	down, err := os.ReadFile("../../../../db/migrations/20260912010000_create_ai_gateway.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.Exec(string(up)).Error) // Idempotent application.
	usageUp, err := os.ReadFile("../../../../db/migrations/20260912020000_create_llm_token_usage.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(usageUp)).Error)
	require.NoError(t, db.AutoMigrate(&model.LLMTokenUsage{}))
	quotaUp, err := os.ReadFile("../../../../db/migrations/20260912030000_add_ai_key_token_limit.up.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(quotaUp)).Error)
	d := &dao{db: db}
	ctx := context.Background()
	for i := 0; i < 100; i++ {
		require.NoError(t, d.CreateAIKey(ctx, &model.AIKey{ProxyID: 1, UserID: 1, Name: "test", Digest: fmt.Sprint(i), Models: `["chat"]`, ExpiresAt: time.Now().Add(time.Hour)}))
	}
	require.Error(t, d.CreateAIKey(ctx, &model.AIKey{ProxyID: 1, UserID: 1, Digest: "over-limit", ExpiresAt: time.Now().Add(time.Hour)}))
	require.NoError(t, d.RevokeAIKey(ctx, 1, 2, 1)) // Other user cannot revoke.
	_, err = d.GetAIKey(ctx, "0")
	require.NoError(t, err)
	require.NoError(t, d.RevokeAIKey(ctx, 1, 1, 1))
	_, err = d.GetAIKey(ctx, "0")
	require.ErrorIs(t, err, gorm.ErrRecordNotFound)
	for i := 0; i < 502; i++ {
		require.NoError(t, d.RecordAIRequest(ctx, &model.AIRequest{ProxyID: 1, UserID: 1, RequestID: fmt.Sprint(i), Model: "chat", Status: 200}))
	}
	var count int64
	require.NoError(t, db.Model(&model.AIRequest{}).Count(&count).Error)
	require.EqualValues(t, 500, count)
	report, err := d.GetLLMTokenUsage(ctx, 1, 1, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.EqualValues(t, 502, report.Summary.Requests)
	require.EqualValues(t, 502, report.Summary.UnknownRequests)
	require.Nil(t, report.Summary.InputTokens)
	require.Len(t, report.Records, 100)
	input, output := int64(21), int64(7)
	measurement := &model.AIRequest{ProxyID: 1, UserID: 1, KeyID: 1, RequestID: "measured", Model: "chat", InputTokens: &input, OutputTokens: &output, Complete: true}
	require.NoError(t, d.RecordAIRequest(ctx, measurement))
	require.NoError(t, d.RecordAIRequest(ctx, measurement))
	report, err = d.GetLLMTokenUsage(ctx, 1, 1, time.Now().Add(-time.Hour))
	require.NoError(t, err)
	require.EqualValues(t, 503, report.Summary.Requests)
	require.Equal(t, input, *report.Summary.InputTokens)
	require.Equal(t, output, *report.Summary.OutputTokens)
	future, err := d.GetLLMTokenUsage(ctx, 1, 1, time.Now().Add(time.Hour))
	require.NoError(t, err)
	require.Zero(t, future.Summary.Requests)
	require.Nil(t, future.Summary.InputTokens)
	require.Error(t, d.RecordAIRequest(ctx, nil))
	negative := int64(-1)
	require.Error(t, d.RecordAIRequest(ctx, &model.AIRequest{RequestID: "negative", ProxyID: 1, UserID: 1, InputTokens: &negative}))
	for _, scope := range [][2]uint{{1, 2}, {2, 1}} {
		isolated, err := d.GetLLMTokenUsage(ctx, scope[0], scope[1], time.Now().Add(-time.Hour))
		require.NoError(t, err)
		require.Zero(t, isolated.Summary.Requests)
		require.Empty(t, isolated.Records)
	}
	rows, err := d.ListAIRequests(ctx, 1, 1, 0)
	require.NoError(t, err)
	require.Len(t, rows, 50)
	require.Equal(t, "measured", rows[0].RequestID)
	rows, err = d.ListAIRequests(ctx, 1, 2, 100)
	require.NoError(t, err)
	require.Empty(t, rows)
	quotaDown, err := os.ReadFile("../../../../db/migrations/20260912030000_add_ai_key_token_limit.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(quotaDown)).Error)
	require.False(t, db.Migrator().HasColumn(&model.AIKey{}, "token_limit"))
	require.NoError(t, db.Exec(string(quotaUp)).Error)
	require.NoError(t, db.AutoMigrate(&model.AIKey{}))
	require.NoError(t, db.Exec(string(down)).Error)
	for _, table := range []string{"ai_applications", "ai_accesses", "ai_keys", "ai_requests"} {
		require.False(t, db.Migrator().HasTable(table))
	}
	require.NoError(t, db.Exec(string(up)).Error)
	require.NoError(t, db.AutoMigrate(&model.AIApplication{}, &model.AIAccess{}, &model.AIKey{}, &model.AIRequest{}))
	usageDown, err := os.ReadFile("../../../../db/migrations/20260912020000_create_llm_token_usage.down.sql")
	require.NoError(t, err)
	require.NoError(t, db.Exec(string(usageDown)).Error)
	require.False(t, db.Migrator().HasTable("llm_token_usage"))
}
