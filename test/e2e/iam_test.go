package e2e

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/utils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

const (
	baseURL      = "http://127.0.0.1:18088"
	testDB       = "test_liaison_e2e.db"
	testEmail    = "default@liaison.local"
	testPassword = "default123"
)

var repoRoot string

// TestConfig 测试配置
type TestConfig struct {
	BaseURL string
	DBPath  string
}

// LoginRequest 登录请求
type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// LoginResponse 登录响应
type LoginResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Token string `json:"token"`
		User  struct {
			ID    uint   `json:"id"`
			Email string `json:"email"`
		} `json:"user"`
	} `json:"data"`
}

// ProfileResponse 用户信息响应
type ProfileResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		ID    uint   `json:"id"`
		Email string `json:"email"`
	} `json:"data"`
}

// ErrorResponse 错误响应
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// TestSuite E2E测试套件
type TestSuite struct {
	config     *TestConfig
	httpClient *http.Client
	serverCmd  *exec.Cmd
	token      string
	configPath string
}

// NewTestSuite 创建测试套件
func NewTestSuite() *TestSuite {
	dbPath := filepath.Join(repoRoot, testDB)
	return &TestSuite{
		config: &TestConfig{
			BaseURL: baseURL,
			DBPath:  dbPath,
		},
		configPath: filepath.Join(repoRoot, "test_config.yaml"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Setup 设置测试环境
func (ts *TestSuite) Setup(t *testing.T) {
	if os.Getenv("LIAISON_E2E") != "1" {
		t.Skip("set LIAISON_E2E=1 to run manager process e2e tests")
	}
	// 清理测试数据库
	ts.cleanupTestDB(t)

	// 启动服务器
	ts.startServer(t)

	// 等待服务器启动
	ts.waitForServer(t)
}

// Teardown 清理测试环境
func (ts *TestSuite) Teardown(t *testing.T) {
	// 停止服务器
	if ts.serverCmd != nil {
		ts.serverCmd.Process.Kill()
		ts.serverCmd.Wait()
	}

	// 清理测试数据库
	ts.cleanupTestDB(t)
}

// cleanupTestDB 清理测试数据库
func (ts *TestSuite) cleanupTestDB(t *testing.T) {
	for _, path := range []string{ts.config.DBPath, ts.config.DBPath + "-wal", ts.config.DBPath + "-shm", ts.configPath} {
		if _, err := os.Stat(path); err == nil {
			err := os.Remove(path)
			require.NoErrorf(t, err, "Failed to remove %s", path)
		}
	}
}

// startServer 启动服务器
func (ts *TestSuite) startServer(t *testing.T) {
	// 创建测试配置文件
	ts.createTestConfig(t)
	ts.ensureServerBinary(t)

	// 启动服务器进程
	ts.serverCmd = exec.Command(filepath.Join(repoRoot, "bin", "liaison"), "-c", ts.configPath)
	ts.serverCmd.Dir = repoRoot

	// 设置环境变量
	ts.serverCmd.Env = append(os.Environ(), "LIAISON_DB_PATH="+ts.config.DBPath)

	// 启动服务器
	err := ts.serverCmd.Start()
	require.NoError(t, err, "Failed to start server")
}

// createTestConfig 创建测试配置文件
func (ts *TestSuite) createTestConfig(t *testing.T) {
	configContent := fmt.Sprintf(`
manager:
  listen:
    addr: "127.0.0.1:18088"
    network: "tcp"
  db: "%s"
  jwt_secret: "e2e-jwt-secret-please-change-1234567890"
  credential_secret: "e2e-credential-secret-please-change-123"
  frontier_edge_port: 13012
frontier:
  controlplane_url: "http://127.0.0.1:13010"
  dial:
    addrs:
      - "127.0.0.1:13011"
    network: "tcp"
daemon:
  pprof:
    enable: false
    rlimit:
      enable: false
log:
  level: error
  file: "%s"
  maxsize: 10
  maxrolls: 1
`, ts.config.DBPath, filepath.Join(repoRoot, "test_e2e.log"))

	err := os.WriteFile(ts.configPath, []byte(configContent), 0644)
	require.NoError(t, err, "Failed to create test config")
}

func (ts *TestSuite) ensureServerBinary(t *testing.T) {
	binaryPath := filepath.Join(repoRoot, "bin", "liaison")
	if _, err := os.Stat(binaryPath); err == nil {
		return
	}
	cmd := exec.Command("go", "build", "-o", binaryPath, "cmd/manager/main.go")
	cmd.Dir = repoRoot
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	require.NoErrorf(t, err, "Failed to build liaison binary: %s", string(out))
}

// waitForServer 等待服务器启动
func (ts *TestSuite) waitForServer(t *testing.T) {
	maxRetries := 30
	for i := 0; i < maxRetries; i++ {
		resp, err := ts.httpClient.Get(ts.config.BaseURL + "/api/health")
		if err == nil {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	require.Fail(t, "Server failed to start within 30 seconds")
}

// makeRequest 发送HTTP请求
func (ts *TestSuite) makeRequest(method, path string, body interface{}, token string) (*http.Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequest(method, ts.config.BaseURL+path, reqBody)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	return ts.httpClient.Do(req)
}

// TestIAMLogin 测试用户登录
func TestIAMLogin(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)

	t.Run("Login with valid credentials", func(t *testing.T) {
		// 首先需要创建默认用户
		ts.createDefaultUser(t)

		loginReq := LoginRequest{
			Email:    testEmail,
			Password: testPassword,
		}

		resp, err := ts.makeRequest("POST", "/api/v1/iam/login", loginReq, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var loginResp LoginResponse
		err = json.NewDecoder(resp.Body).Decode(&loginResp)
		require.NoError(t, err)

		assert.Equal(t, 200, loginResp.Code)
		assert.NotEmpty(t, loginResp.Data.Token)
		assert.Equal(t, testEmail, loginResp.Data.User.Email)

		// 保存token供后续测试使用
		ts.token = loginResp.Data.Token
	})

	t.Run("Login with invalid credentials", func(t *testing.T) {
		loginReq := LoginRequest{
			Email:    testEmail,
			Password: "wrongpassword",
		}

		resp, err := ts.makeRequest("POST", "/api/v1/iam/login", loginReq, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errorResp ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)

		assert.NotEqual(t, 200, errorResp.Code)
	})
}

// TestIAMProfile 测试获取用户信息
func TestIAMProfile(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)

	// 先登录获取token
	ts.loginAndGetToken(t)

	t.Run("Get profile with valid token", func(t *testing.T) {
		resp, err := ts.makeRequest("GET", "/api/v1/iam/profile", nil, ts.token)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var profileResp ProfileResponse
		err = json.NewDecoder(resp.Body).Decode(&profileResp)
		require.NoError(t, err)

		assert.Equal(t, 200, profileResp.Code)
		assert.Equal(t, testEmail, profileResp.Data.Email)
	})

	t.Run("Get profile without token", func(t *testing.T) {
		resp, err := ts.makeRequest("GET", "/api/v1/iam/profile", nil, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errorResp ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)

		assert.NotEqual(t, 200, errorResp.Code)
		assert.Contains(t, errorResp.Message, "No authentication token provided")
	})

	t.Run("Get profile with invalid token", func(t *testing.T) {
		resp, err := ts.makeRequest("GET", "/api/v1/iam/profile", nil, "invalid_token")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errorResp ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)

		assert.NotEqual(t, 200, errorResp.Code)
		assert.Contains(t, errorResp.Message, "Token validation failed")
	})
}

// TestAuthenticationMiddleware 测试认证中间件
func TestAuthenticationMiddleware(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)

	// 先登录获取token
	ts.loginAndGetToken(t)

	t.Run("Access protected endpoint with valid token", func(t *testing.T) {
		resp, err := ts.makeRequest("GET", "/api/v1/applications", nil, ts.token)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该能正常访问，返回200或相应的业务状态码
		assert.True(t, resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNotFound)
	})

	t.Run("Access protected endpoint without token", func(t *testing.T) {
		resp, err := ts.makeRequest("GET", "/api/v1/applications", nil, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)

		var errorResp ErrorResponse
		err = json.NewDecoder(resp.Body).Decode(&errorResp)
		require.NoError(t, err)

		assert.NotEqual(t, 200, errorResp.Code)
		assert.Contains(t, errorResp.Message, "No authentication token provided")
	})

	t.Run("Access login endpoint without token", func(t *testing.T) {
		loginReq := LoginRequest{
			Email:    testEmail,
			Password: testPassword,
		}

		resp, err := ts.makeRequest("POST", "/api/v1/iam/login", loginReq, "")
		require.NoError(t, err)
		defer resp.Body.Close()

		// 登录接口应该不需要认证
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})
}

// TestIAMLogout 测试用户登出
func TestIAMLogout(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)

	// 先登录获取token
	ts.loginAndGetToken(t)

	t.Run("Logout with valid token", func(t *testing.T) {
		resp, err := ts.makeRequest("POST", "/api/v1/iam/logout", nil, ts.token)
		require.NoError(t, err)
		defer resp.Body.Close()

		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var logoutResp struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		err = json.NewDecoder(resp.Body).Decode(&logoutResp)
		require.NoError(t, err)

		assert.Equal(t, 200, logoutResp.Code)
	})
}

// Helper methods

// createDefaultUser 创建默认用户（模拟安装脚本的行为）
func (ts *TestSuite) createDefaultUser(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(ts.config.DBPath), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&model.User{}))
	hash, err := utils.HashPassword(testPassword)
	require.NoError(t, err)
	var user model.User
	err = db.Where("email = ?", testEmail).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		require.NoError(t, db.Create(&model.User{
			Email:    testEmail,
			Password: hash,
			Status:   model.UserStatusActive,
		}).Error)
		return
	}
	require.NoError(t, err)
	require.NoError(t, db.Model(&user).Updates(map[string]any{
		"password": hash,
		"status":   model.UserStatusActive,
	}).Error)
}

// loginAndGetToken 登录并获取token
func (ts *TestSuite) loginAndGetToken(t *testing.T) {
	// 创建默认用户
	ts.createDefaultUser(t)

	loginReq := LoginRequest{
		Email:    testEmail,
		Password: testPassword,
	}

	resp, err := ts.makeRequest("POST", "/api/v1/iam/login", loginReq, "")
	require.NoError(t, err)
	defer resp.Body.Close()

	var loginResp LoginResponse
	err = json.NewDecoder(resp.Body).Decode(&loginResp)
	require.NoError(t, err)

	if loginResp.Code == 200 {
		ts.token = loginResp.Data.Token
	} else {
		// 如果登录失败，可能需要先创建用户
		t.Logf("Login failed, may need to create default user first: %s", loginResp.Message)
	}
}

// TestMain 测试主函数
func TestMain(m *testing.M) {
	// 确保在项目根目录
	_ = os.Chdir("../..")
	repoRoot, _ = os.Getwd()

	// 运行测试
	code := m.Run()

	// 清理测试配置文件
	os.Remove("test_config.yaml")
	os.Remove(testDB)
	os.Remove(testDB + "-wal")
	os.Remove(testDB + "-shm")
	os.Remove("test_e2e.log")

	os.Exit(code)
}
