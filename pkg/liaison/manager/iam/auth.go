package iam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/liaison/pkg/liaison/repo"
	"github.com/singchia/liaison/pkg/liaison/repo/model"
	"github.com/singchia/liaison/pkg/utils"
)

// IAMService IAM服务
type IAMService struct {
	repo repo.Repo
}

// NewIAMService 创建IAM服务
func NewIAMService(repo repo.Repo) *IAMService {
	return &IAMService{
		repo: repo,
	}
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Token string      `json:"token"`
	User  *model.User `json:"user"`
}

// GetProfileRequest 获取用户信息请求
type GetProfileRequest struct{}

// GetProfileResponse 获取用户信息响应
type GetProfileResponse struct {
	User *User `json:"user"`
}

// LogoutRequest 登出请求
type LogoutRequest struct{}

// LogoutResponse 登出响应
type LogoutResponse struct {
	Message string `json:"message"`
}

// ChangePasswordRequest 修改密码请求
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" binding:"required"`
	NewPassword string `json:"new_password" binding:"required"`
}

// ChangePasswordResponse 修改密码响应
type ChangePasswordResponse struct {
	Message string `json:"message"`
}

// User 用户信息（用于API响应）
type User struct {
	ID    uint   `json:"id"`
	Email string `json:"email"`
}

// Login 用户登录
func (s *IAMService) Login(req *LoginRequest, loginIP string) (*LoginResponse, error) {
	// 获取用户
	user, err := s.repo.GetUserByEmail(req.Email)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	// 检查用户状态
	if user.Status != model.UserStatusActive {
		return nil, errors.New("user account is disabled")
	}

	// 验证密码
	valid, err := utils.VerifyPassword(req.Password, user.Password)
	if err != nil {
		return nil, err
	}
	if !valid {
		return nil, errors.New("invalid password")
	}

	// 更新最后登录时间和IP
	if err := s.repo.UpdateUserLastLoginAndIP(user.ID, loginIP); err != nil {
		log.Errorf("Failed to update last login time and IP: %v", err)
	}

	// 生成JWT token
	token, err := utils.GenerateToken(user.ID, user.Email)
	if err != nil {
		return nil, err
	}

	return &LoginResponse{
		Token: token,
		User:  user,
	}, nil
}

// CreateDefaultUser 创建默认用户账户
func (s *IAMService) CreateDefaultUser() error {
	// 检查是否已存在default用户
	exists, err := s.repo.CheckUserExists("default@liaison.local")
	if err != nil {
		return err
	}
	if exists {
		return nil // 已存在，不需要创建
	}

	// 生成随机密码
	password, err := utils.GenerateRandomPassword(16)
	if err != nil {
		return err
	}

	// 加密密码
	hashedPassword, err := utils.HashPassword(password)
	if err != nil {
		return err
	}

	// 创建default用户
	user := &model.User{
		Email:    "default@liaison.local",
		Password: hashedPassword,
		Status:   model.UserStatusActive,
	}

	if err := s.repo.CreateUser(user); err != nil {
		return err
	}

	// 将密码保存到文件
	if err := s.saveDefaultPassword(password); err != nil {
		log.Errorf("Failed to save default password: %v", err)
	}

	log.Infof("Default user account created successfully: default@liaison.local")
	log.Infof("Default password: %s", password)
	log.Infof("Password saved to: %s", s.getPasswordFilePath())

	return nil
}

// saveDefaultPassword 保存默认密码到文件
func (s *IAMService) saveDefaultPassword(password string) error {
	filePath := s.getPasswordFilePath()

	// 确保目录存在
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// 写入密码文件
	content := "Liaison 默认用户账户信息\n"
	content += "================================\n"
	content += "邮箱: default@liaison.local\n"
	content += fmt.Sprintf("密码: %s\n", password)
	content += "================================\n"
	content += "请妥善保管此信息，首次登录后建议修改密码\n"

	return os.WriteFile(filePath, []byte(content), 0600)
}

// getPasswordFilePath 获取密码文件路径
func (s *IAMService) getPasswordFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "/tmp"
	}
	return filepath.Join(homeDir, ".liaison", "admin_password.txt")
}

// ValidateToken 验证JWT token
func (s *IAMService) ValidateToken(tokenString string) (*utils.Claims, error) {
	return utils.ValidateToken(tokenString)
}

// GetUserByToken 根据token获取用户信息
func (s *IAMService) GetUserByToken(tokenString string) (*model.User, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, err
	}

	user, err := s.repo.GetUserByID(claims.UserID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}

	return user, nil
}

// GetUserByID 根据用户ID获取用户信息
func (s *IAMService) GetUserByID(userID uint) (*model.User, error) {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, errors.New("user not found")
	}
	return user, nil
}

// UpdateProfile 更新用户资料（如头像）
func (s *IAMService) UpdateProfile(userID uint, avatar string) error {
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}
	user.Avatar = avatar
	return s.repo.UpdateUser(user)
}

// ChangePassword 修改密码
func (s *IAMService) ChangePassword(userID uint, req *ChangePasswordRequest) error {
	// 获取用户
	user, err := s.repo.GetUserByID(userID)
	if err != nil {
		return err
	}
	if user == nil {
		return errors.New("user not found")
	}

	// 验证旧密码
	valid, err := utils.VerifyPassword(req.OldPassword, user.Password)
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("invalid old password")
	}

	// 加密新密码
	hashedPassword, err := utils.HashPassword(req.NewPassword)
	if err != nil {
		return err
	}

	// 更新密码
	user.Password = hashedPassword
	if err := s.repo.UpdateUser(user); err != nil {
		return err
	}

	return nil
}
