package model

import "time"

// RevokedSession stores only a SHA-256 fingerprint, never a bearer credential.
type RevokedSession struct {
	TokenHash string    `gorm:"primaryKey;size:64"`
	ExpiresAt time.Time `gorm:"not null;index"`
}
