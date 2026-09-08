package model

import "time"

// AgentModelSetting stores the complete configuration as authenticated ciphertext.
type AgentModelSetting struct {
	ID        uint   `gorm:"primaryKey"`
	Payload   []byte `gorm:"not null"`
	UpdatedAt time.Time
}
