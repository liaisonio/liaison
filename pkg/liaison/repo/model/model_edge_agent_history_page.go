package model

// EdgeAgentHistoryPage stores one bounded, encrypted completed display window.
type EdgeAgentHistoryPage struct {
	OwnerID   uint   `gorm:"primaryKey;autoIncrement:false"`
	AccessID  string `gorm:"primaryKey;size:32"`
	EdgeID    uint64 `gorm:"primaryKey;autoIncrement:false"`
	SessionID string `gorm:"primaryKey;size:32"`
	Window    uint64 `gorm:"primaryKey;autoIncrement:false"`
	Revision  uint64 `gorm:"not null"`
	Payload   []byte
}
