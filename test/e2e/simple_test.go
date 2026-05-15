package e2e

import (
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHealthCheck 测试健康检查端点
func TestHealthCheck(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	t.Run("Health check endpoint requires auth", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/health")
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该返回401或相应的错误响应，因为需要认证
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// 检查是否包含认证错误信息
		bodyStr := string(body)
		assert.True(t,
			resp.StatusCode == http.StatusUnauthorized ||
				contains(bodyStr, "No authentication token provided") ||
				contains(bodyStr, "Token validation failed"),
			"Expected authentication error, got: %s", bodyStr)
	})
}

// TestIAMEndpoints 测试IAM端点（不需要认证）
func TestIAMEndpoints(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	t.Run("Login endpoint accessible without auth", func(t *testing.T) {
		resp, err := client.Post(baseURL+"/api/v1/iam/login", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该返回400或422（缺少请求体），而不是401（未认证）
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Logout endpoint accessible without auth", func(t *testing.T) {
		resp, err := client.Post(baseURL+"/api/v1/iam/logout", "application/json", nil)
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该返回200或400，而不是401（未认证）
		assert.NotEqual(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestProtectedEndpoints 测试受保护的端点
func TestProtectedEndpoints(t *testing.T) {
	ts := NewTestSuite()
	ts.Setup(t)
	defer ts.Teardown(t)
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	t.Run("Applications endpoint requires auth", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/v1/applications")
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该返回401或相应的错误响应
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// 检查是否包含认证错误信息
		bodyStr := string(body)
		assert.True(t,
			resp.StatusCode == http.StatusUnauthorized ||
				contains(bodyStr, "No authentication token provided") ||
				contains(bodyStr, "Token validation failed"),
			"Expected authentication error, got: %s", bodyStr)
	})

	t.Run("Profile endpoint requires auth", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/api/v1/iam/profile")
		require.NoError(t, err)
		defer resp.Body.Close()

		// 应该返回401或相应的错误响应
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		// 检查是否包含认证错误信息
		bodyStr := string(body)
		assert.True(t,
			resp.StatusCode == http.StatusUnauthorized ||
				contains(bodyStr, "No authentication token provided") ||
				contains(bodyStr, "Token validation failed"),
			"Expected authentication error, got: %s", bodyStr)
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) &&
			(s[:len(substr)] == substr ||
				s[len(s)-len(substr):] == substr ||
				containsSubstring(s, substr))))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
