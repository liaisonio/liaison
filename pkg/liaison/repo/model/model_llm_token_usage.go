package model

import "time"

// LLMTokenUsage is immutable request-level metering, not a billing ledger.
// No FK cascades: revoking a key or deleting a resource must not erase usage.
type LLMTokenUsage struct {
	ID           uint      `gorm:"column:id;primaryKey" json:"-"`
	RequestID    string    `gorm:"column:request_id;not null;uniqueIndex:idx_llm_usage_request" json:"request_id"`
	ProxyID      uint      `gorm:"column:proxy_id;not null;index:idx_llm_usage_owner,priority:1" json:"-"`
	UserID       uint      `gorm:"column:user_id;not null;index:idx_llm_usage_owner,priority:2" json:"-"`
	KeyID        uint      `gorm:"column:key_id;not null;default:0;index:idx_llm_usage_key" json:"key_id"`
	Model        string    `gorm:"column:model;not null" json:"model"`
	InputTokens  *int64    `gorm:"column:input_tokens" json:"input_tokens"`
	OutputTokens *int64    `gorm:"column:output_tokens" json:"output_tokens"`
	Complete     bool      `gorm:"column:complete;not null;default:false" json:"complete"`
	CreatedAt    time.Time `gorm:"column:created_at;not null;index:idx_llm_usage_owner,priority:3" json:"created_at"`
}

func (LLMTokenUsage) TableName() string { return "llm_token_usage" }

type LLMTokenUsageSummary struct {
	Requests        int64  `json:"requests"`
	UnknownRequests int64  `json:"unknown_requests"`
	InputTokens     *int64 `json:"input_tokens"`
	OutputTokens    *int64 `json:"output_tokens"`
}

type LLMTokenUsageReport struct {
	Summary LLMTokenUsageSummary `json:"summary"`
	Records []LLMTokenUsage      `json:"records"`
	Since   time.Time            `json:"since"`
}

// AIKeyUsage is a scoped projection, never includes the stored key digest.
type AIKeyUsage struct {
	KeyID           uint   `json:"key_id"`
	TokenLimit      *int64 `json:"token_limit"`
	UsedTokens      int64  `json:"used_tokens"`
	UnknownRequests int64  `json:"unknown_requests"`
}
