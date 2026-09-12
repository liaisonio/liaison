package model

import "time"

type AIApplication struct {
	ApplicationID     uint      `gorm:"column:application_id;primaryKey;autoIncrement:false"`
	Protocol          string    `gorm:"column:protocol;not null"`
	BasePath          string    `gorm:"column:base_path;not null"`
	TLS               bool      `gorm:"column:tls;not null;default:false"`
	EncryptedKey      string    `gorm:"column:encrypted_key;not null;default:''" json:"-"`
	TargetFingerprint string    `gorm:"column:target_fingerprint;not null" json:"-"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (AIApplication) TableName() string { return "ai_applications" }

type AIAccess struct {
	ProxyID   uint      `gorm:"column:proxy_id;primaryKey;autoIncrement:false"`
	Enabled   bool      `gorm:"column:enabled;not null;default:false"`
	Models    string    `gorm:"column:models;not null;default:'{}'"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (AIAccess) TableName() string { return "ai_accesses" }

type AIKey struct {
	ID         uint       `gorm:"column:id;primaryKey" json:"id"`
	ProxyID    uint       `gorm:"column:proxy_id;not null;index:idx_ai_keys_owner,priority:1" json:"-"`
	UserID     uint       `gorm:"column:user_id;not null;index:idx_ai_keys_owner,priority:2" json:"-"`
	Name       string     `gorm:"column:name;not null" json:"name"`
	Digest     string     `gorm:"column:digest;not null;uniqueIndex:idx_ai_keys_digest" json:"-"`
	Models     string     `gorm:"column:models;not null" json:"-"`
	ExpiresAt  time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at" json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	TokenLimit *int64     `gorm:"column:token_limit" json:"token_limit"`
}

func (AIKey) TableName() string { return "ai_keys" }

type AIRequest struct {
	ID           uint      `gorm:"column:id;primaryKey" json:"-"`
	RequestID    string    `gorm:"column:request_id;not null" json:"request_id"`
	KeyID        uint      `gorm:"column:key_id;not null;default:0" json:"key_id"` // Zero identifies authenticated UI tests.
	ProxyID      uint      `gorm:"column:proxy_id;not null;index:idx_ai_requests_owner,priority:1" json:"-"`
	UserID       uint      `gorm:"column:user_id;not null;index:idx_ai_requests_owner,priority:2" json:"-"`
	Model        string    `gorm:"column:model;not null" json:"model"`
	Status       int       `gorm:"column:status;not null" json:"status"`
	DurationMS   int64     `gorm:"column:duration_ms;not null" json:"duration_ms"`
	InputTokens  *int64    `gorm:"column:input_tokens" json:"input_tokens"`
	OutputTokens *int64    `gorm:"column:output_tokens" json:"output_tokens"`
	Complete     bool      `gorm:"column:complete;not null;default:false" json:"complete"`
	CreatedAt    time.Time `gorm:"column:created_at" json:"created_at"`
}

func (AIRequest) TableName() string { return "ai_requests" }
