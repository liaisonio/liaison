package utils

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// JWTConfig JWT配置
type JWTConfig struct {
	SecretKey      string
	ExpirationTime time.Duration
}

// DefaultJWTConfig 默认JWT配置
var DefaultJWTConfig = &JWTConfig{
	SecretKey:      "",             // 必须从配置文件设置，不允许使用默认值
	ExpirationTime: 24 * time.Hour, // 24小时过期
}

// SetJWTSecret 设置JWT密钥（用于从配置文件读取）
// 如果secret为空，将返回错误
func SetJWTSecret(secret string) error {
	if secret == "" {
		return errors.New("JWT secret key is required and cannot be empty")
	}
	if len(secret) < 32 {
		return errors.New("JWT secret key must be at least 32 characters long for security")
	}
	DefaultJWTConfig.SecretKey = secret
	return nil
}

// Claims JWT声明
type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// GenerateToken 生成JWT token
func GenerateToken(userID uint, email string) (string, error) {
	if DefaultJWTConfig.SecretKey == "" {
		return "", errors.New("JWT secret key is not configured")
	}
	expirationTime := time.Now().Add(DefaultJWTConfig.ExpirationTime)
	var sessionID [32]byte
	if _, err := rand.Read(sessionID[:]); err != nil {
		return "", err
	}

	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        hex.EncodeToString(sessionID[:]),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "liaison",
			Subject:   email,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(DefaultJWTConfig.SecretKey))
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

// ValidateToken 验证JWT token
func ValidateToken(tokenString string) (*Claims, error) {
	if DefaultJWTConfig.SecretKey == "" {
		return nil, errors.New("JWT secret key is not configured")
	}
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(DefaultJWTConfig.SecretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// RefreshToken 刷新token
func RefreshToken(tokenString string) (string, error) {
	claims, err := ValidateToken(tokenString)
	if err != nil {
		return "", err
	}

	// 检查token是否即将过期（剩余时间少于1小时）
	if time.Until(claims.ExpiresAt.Time) < time.Hour {
		return GenerateToken(claims.UserID, claims.Email)
	}

	// 如果还有足够时间，返回原token
	return tokenString, nil
}
