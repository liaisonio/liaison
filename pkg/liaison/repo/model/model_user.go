package model

import (
	"time"

	"gorm.io/gorm"
)

// User 用户模型
type User struct {
	gorm.Model
	Email     string     `gorm:"column:email;type:varchar(255);uniqueIndex;not null" json:"email"`
	Password  string     `gorm:"column:password;type:varchar(255);not null" json:"-"` // 不序列化密码
	Status    UserStatus `gorm:"column:status;type:varchar(50);not null;default:'active'" json:"status"`
	LastLogin *time.Time `gorm:"column:last_login;type:datetime" json:"last_login"`
	LoginIP   string     `gorm:"column:login_ip;type:varchar(45)" json:"login_ip"`   // IPv6最长45字符
	Avatar    string     `gorm:"column:avatar;type:varchar(512)" json:"avatar"`     // 头像 URL
}

// UserStatus 用户状态
type UserStatus string

const (
	UserStatusActive   UserStatus = "active"
	UserStatusInactive UserStatus = "inactive"
	UserStatusLocked   UserStatus = "locked"
)

// TableName 指定表名
func (User) TableName() string {
	return "users"
}
