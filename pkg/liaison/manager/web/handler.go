package web

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
)

func mapServiceError(err error) error {
	if err == nil {
		return nil
	}
	var httpErr *controlplane.HTTPError
	if errors.As(err, &httpErr) {
		return kratoserrors.New(httpErr.Status(), httpErr.Reason(), httpErr.Message())
	}
	if errors.Is(err, controlplane.ErrForbidden()) {
		return kratoserrors.New(403, "FORBIDDEN", "无权访问该资源")
	}
	return err
}

//-- Edge --//

// @Summary CreateEdge
// @Tags 1.0
// @Param params query v1.CreateEdgeRequest true "queries"
// @Success 200 {object} v1.CreateEdgeResponse
// @Router /api/v1/edges [post]
func (web *web) CreateEdge(ctx context.Context, req *v1.CreateEdgeRequest) (*v1.CreateEdgeResponse, error) {
	resp, err := web.controlPlane.CreateEdge(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary GetEdge
// @Tags 1.0
// @Param id path int true "edge id"
// @Param params query v1.GetEdgeRequest true "queries"
// @Success 200 {object} v1.GetEdgeResponse
// @Router /api/v1/edges/{id} [get]
func (web *web) GetEdge(ctx context.Context, req *v1.GetEdgeRequest) (*v1.GetEdgeResponse, error) {
	resp, err := web.controlPlane.GetEdge(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary ListEdges
// @Tags 1.0
// @Param params query v1.ListEdgesRequest true "queries"
// @Success 200 {object} v1.ListEdgesResponse
// @Router /api/v1/edges [get]
func (web *web) ListEdges(ctx context.Context, req *v1.ListEdgesRequest) (*v1.ListEdgesResponse, error) {
	resp, err := web.controlPlane.ListEdges(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary UpdateEdge
// @Tags 1.0
// @Param id path int true "edge id"
// @Param params query v1.UpdateEdgeRequest true "queries"
// @Success 200 {object} v1.UpdateEdgeResponse
// @Router /api/v1/edges/{id} [put]
func (web *web) UpdateEdge(ctx context.Context, req *v1.UpdateEdgeRequest) (*v1.UpdateEdgeResponse, error) {
	resp, err := web.controlPlane.UpdateEdge(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary DeleteEdge
// @Tags 1.0
// @Param id path int true "edge id"
// @Success 200 {object} v1.DeleteEdgeResponse
// @Router /api/v1/edges/{id} [delete]
func (web *web) DeleteEdge(ctx context.Context, req *v1.DeleteEdgeRequest) (*v1.DeleteEdgeResponse, error) {
	resp, err := web.controlPlane.DeleteEdge(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

//-- Device --//

// @Summary ListDevices
// @Tags 1.0
// @Param params query v1.ListDevicesRequest true "queries"
// @Success 200 {object} v1.ListDevicesResponse
// @Router /api/v1/devices [get]
func (web *web) ListDevices(ctx context.Context, req *v1.ListDevicesRequest) (*v1.ListDevicesResponse, error) {
	resp, err := web.controlPlane.ListDevices(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary GetDevice
// @Tags 1.0
// @Param id path int true "device id"
// @Param params query v1.GetDeviceRequest true "queries"
// @Success 200 {object} v1.GetDeviceResponse
// @Router /api/v1/devices/{id} [get]
func (web *web) GetDevice(ctx context.Context, req *v1.GetDeviceRequest) (*v1.GetDeviceResponse, error) {
	resp, err := web.controlPlane.GetDevice(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary UpdateDevice
// @Tags 1.0
// @Param id path int true "device id"
// @Param params query v1.UpdateDeviceRequest true "queries"
// @Success 200 {object} v1.UpdateDeviceResponse
// @Router /api/v1/devices/{id} [put]
func (web *web) UpdateDevice(ctx context.Context, req *v1.UpdateDeviceRequest) (*v1.UpdateDeviceResponse, error) {
	resp, err := web.controlPlane.UpdateDevice(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary DeleteDevice
// @Tags 1.0
// @Param id path int true "device id"
// @Success 200 {object} v1.DeleteDeviceResponse
// @Router /api/v1/devices/{id} [delete]
func (web *web) DeleteDevice(ctx context.Context, req *v1.DeleteDeviceRequest) (*v1.DeleteDeviceResponse, error) {
	resp, err := web.controlPlane.DeleteDevice(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

//-- Application --//

// @Summary CreateApplication
// @Tags 1.0
// @Param params query v1.CreateApplicationRequest true "queries"
// @Success 200 {object} v1.CreateApplicationResponse
// @Router /api/v1/applications [post]
func (web *web) CreateApplication(ctx context.Context, req *v1.CreateApplicationRequest) (*v1.CreateApplicationResponse, error) {
	resp, err := web.controlPlane.CreateApplication(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary ListApplications
// @Tags 1.0
// @Param params query v1.ListApplicationsRequest true "queries"
// @Success 200 {object} v1.ListApplicationsResponse
// @Router /api/v1/applications [get]
func (web *web) ListApplications(ctx context.Context, req *v1.ListApplicationsRequest) (*v1.ListApplicationsResponse, error) {
	resp, err := web.controlPlane.ListApplications(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary UpdateApplication
// @Tags 1.0
// @Param id path int true "application id"
// @Param params query v1.UpdateApplicationRequest true "queries"
// @Success 200 {object} v1.UpdateApplicationResponse
// @Router /api/v1/applications/{id} [put]
func (web *web) UpdateApplication(ctx context.Context, req *v1.UpdateApplicationRequest) (*v1.UpdateApplicationResponse, error) {
	resp, err := web.controlPlane.UpdateApplication(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary DeleteApplication
// @Tags 1.0
// @Param id path int true "application id"
// @Success 200 {object} v1.DeleteApplicationResponse
// @Router /api/v1/applications/{id} [delete]
func (web *web) DeleteApplication(ctx context.Context, req *v1.DeleteApplicationRequest) (*v1.DeleteApplicationResponse, error) {
	resp, err := web.controlPlane.DeleteApplication(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

//-- Proxy --//

// @Summary ListProxies
// @Tags 1.0
// @Param params query v1.ListProxiesRequest true "queries"
// @Success 200 {object} v1.ListProxiesResponse
// @Router /api/v1/proxies [get]
func (web *web) ListProxies(ctx context.Context, req *v1.ListProxiesRequest) (*v1.ListProxiesResponse, error) {
	resp, err := web.controlPlane.ListProxies(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary CreateProxy
// @Tags 1.0
// @Param params query v1.CreateProxyRequest true "queries"
// @Success 200 {object} v1.CreateProxyResponse
// @Router /api/v1/proxies [post]
func (web *web) CreateProxy(ctx context.Context, req *v1.CreateProxyRequest) (*v1.CreateProxyResponse, error) {
	resp, err := web.controlPlane.CreateProxy(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary UpdateProxy
// @Tags 1.0
// @Param id path int true "proxy id"
// @Param params query v1.UpdateProxyRequest true "queries"
// @Success 200 {object} v1.UpdateProxyResponse
// @Router /api/v1/proxies/{id} [put]
func (web *web) UpdateProxy(ctx context.Context, req *v1.UpdateProxyRequest) (*v1.UpdateProxyResponse, error) {
	resp, err := web.controlPlane.UpdateProxy(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	if req.Status == "stopped" {
		web.closeProxyInteractiveSessions(uint(req.Id))
	}
	return resp, nil
}

// @Summary DeleteProxy
// @Tags 1.0
// @Param id path int true "proxy id"
// @Success 200 {object} v1.DeleteProxyResponse
// @Router /api/v1/proxies/{id} [delete]
func (web *web) DeleteProxy(ctx context.Context, req *v1.DeleteProxyRequest) (*v1.DeleteProxyResponse, error) {
	resp, err := web.controlPlane.DeleteProxy(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	web.closeProxyInteractiveSessions(uint(req.Id))
	return resp, nil
}

func (web *web) closeProxyInteractiveSessions(proxyID uint) {
	if proxyID == 0 {
		return
	}
	if web.webSSH != nil {
		web.webSSH.closeByProxy(proxyID)
	}
	if web.webDesktop != nil {
		web.webDesktop.closeByProxy(proxyID)
	}
	if web.webData != nil {
		web.webData.closeByProxy(proxyID)
	}
}

//-- Traffic Metric --//

// @Summary ListTrafficMetrics
// @Tags 1.0
// @Param params query v1.ListTrafficMetricsRequest true "queries"
// @Success 200 {object} v1.ListTrafficMetricsResponse
// @Router /api/v1/traffic-metrics [get]
func (web *web) ListTrafficMetrics(ctx context.Context, req *v1.ListTrafficMetricsRequest) (*v1.ListTrafficMetricsResponse, error) {
	resp, err := web.controlPlane.ListTrafficMetrics(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

//-- Task --//

// @Summary CreateEdgeScanApplicationTask
// @Tags 1.0
// @Param params query v1.CreateEdgeScanApplicationTaskRequest true "queries"
// @Success 200 {object} v1.CreateEdgeScanApplicationTaskResponse
// @Router /api/v1/edges/{edge_id}/scan_application_tasks [post]
func (web *web) CreateEdgeScanApplicationTask(ctx context.Context, req *v1.CreateEdgeScanApplicationTaskRequest) (*v1.CreateEdgeScanApplicationTaskResponse, error) {
	resp, err := web.controlPlane.CreateEdgeScanApplicationTask(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

// @Summary GetEdgeScanApplicationTask
// @Tags 1.0
// @Param params query v1.GetEdgeScanApplicationTaskRequest true "queries"
// @Success 200 {object} v1.GetEdgeScanApplicationTaskResponse
// @Router /api/v1/edges/{edge_id}/scan_application_tasks [get]
func (web *web) GetEdgeScanApplicationTask(ctx context.Context, req *v1.GetEdgeScanApplicationTaskRequest) (*v1.GetEdgeScanApplicationTaskResponse, error) {
	resp, err := web.controlPlane.GetEdgeScanApplicationTask(ctx, req)
	if err != nil {
		return nil, mapServiceError(err)
	}
	return resp, nil
}

//-- Auth --//

// @Summary Login
// @Tags IAM
// @Param params body v1.LoginRequest true "登录请求"
// @Success 200 {object} v1.LoginResponse
// @Router /api/v1/iam/login [post]
// Login 用户登录
func (web *web) Login(ctx context.Context, req *v1.LoginRequest) (*v1.LoginResponse, error) {
	// 获取客户端IP
	loginIP := getClientIP(ctx)

	// 转换请求类型
	iamReq := &iam.LoginRequest{
		Email:    req.Email,
		Password: req.Password,
	}

	resp, err := web.iamService.Login(iamReq, loginIP)
	if err != nil {
		statusCode := http.StatusInternalServerError
		// 根据错误类型返回适当的 HTTP 状态码
		errMsg := err.Error()
		if errMsg == "invalid password" || errMsg == "密码错误" {
			statusCode = http.StatusUnauthorized
			web.recordLoginManagementAudit(ctx, req.Email, 0, loginIP, false, statusCode)
			return nil, kratoserrors.New(401, "INVALID_PASSWORD", "密码错误")
		}
		if errMsg == "user not found" || errMsg == "用户不存在" {
			statusCode = http.StatusUnauthorized
			web.recordLoginManagementAudit(ctx, req.Email, 0, loginIP, false, statusCode)
			return nil, kratoserrors.New(401, "USER_NOT_FOUND", "用户不存在")
		}
		if errMsg == "user account is disabled" || errMsg == "用户账户已禁用" {
			statusCode = http.StatusForbidden
			web.recordLoginManagementAudit(ctx, req.Email, 0, loginIP, false, statusCode)
			return nil, kratoserrors.New(403, "ACCOUNT_DISABLED", "用户账户已禁用")
		}
		web.recordLoginManagementAudit(ctx, req.Email, 0, loginIP, false, statusCode)
		// 其他错误返回 500
		return nil, err
	}
	web.recordLoginManagementAudit(ctx, resp.User.Email, resp.User.ID, loginIP, true, http.StatusOK)

	// 转换响应类型
	userV1 := &v1.User{
		Id:    uint64(resp.User.ID),
		Email: resp.User.Email,
	}
	// 填充注册时间
	if !resp.User.CreatedAt.IsZero() {
		userV1.CreatedAt = resp.User.CreatedAt.Format(time.DateTime)
	}
	// 填充最后登录时间
	if resp.User.LastLogin != nil && !resp.User.LastLogin.IsZero() {
		userV1.LastLogin = resp.User.LastLogin.Format(time.DateTime)
	}
	// 填充登录IP
	userV1.LoginIp = resp.User.LoginIP

	return &v1.LoginResponse{
		Code:    200,
		Message: "success",
		Data: &v1.LoginData{
			Token: resp.Token,
			User:  userV1,
		},
	}, nil
}

// getClientIP 从context中获取客户端IP地址
func getClientIP(ctx context.Context) string {
	if httpReq, ok := kratoshttp.RequestFromServerContext(ctx); ok {
		// 优先检查 X-Forwarded-For 头（用于反向代理）
		forwarded := httpReq.Header.Get("X-Forwarded-For")
		if forwarded != "" {
			// X-Forwarded-For 可能包含多个IP，取第一个
			ips := strings.Split(forwarded, ",")
			if len(ips) > 0 {
				ip := strings.TrimSpace(ips[0])
				if ip != "" {
					return ip
				}
			}
		}

		// 检查 X-Real-IP 头
		realIP := httpReq.Header.Get("X-Real-IP")
		if realIP != "" {
			return realIP
		}

		// 从 RemoteAddr 获取
		ip, _, err := net.SplitHostPort(httpReq.RemoteAddr)
		if err == nil {
			return ip
		}
		return httpReq.RemoteAddr
	}
	return ""
}

// @Summary GetProfile
// @Tags IAM
// @Param params query v1.GetProfileRequest true "获取用户信息请求"
// @Success 200 {object} v1.GetProfileResponse
// @Router /api/v1/iam/profile [get]
// GetProfile 获取用户信息
func (web *web) GetProfile(ctx context.Context, req *v1.GetProfileRequest) (*v1.GetProfileResponse, error) {
	// 从context中获取用户信息（需要中间件设置）
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, errors.New("未认证")
	}

	// 从数据库获取完整的用户信息
	user, err := web.iamService.GetUserByID(userID.(uint))
	if err != nil {
		return nil, err
	}

	// 转换用户信息
	userV1 := &v1.User{
		Id:    uint64(user.ID),
		Email: user.Email,
	}
	// 填充注册时间
	if !user.CreatedAt.IsZero() {
		userV1.CreatedAt = user.CreatedAt.Format(time.DateTime)
	}
	// 填充最后登录时间
	if user.LastLogin != nil && !user.LastLogin.IsZero() {
		userV1.LastLogin = user.LastLogin.Format(time.DateTime)
	}
	// 填充登录IP
	userV1.LoginIp = user.LoginIP

	return &v1.GetProfileResponse{
		Code:    200,
		Message: "success",
		Data:    userV1,
	}, nil
}

// @Summary Logout
// @Tags IAM
// @Param params body v1.LogoutRequest true "登出请求"
// @Success 200 {object} v1.LogoutResponse
// @Router /api/v1/iam/logout [post]
// Logout 用户登出
func (web *web) Logout(ctx context.Context, req *v1.LogoutRequest) (*v1.LogoutResponse, error) {
	// JWT是无状态的，登出只需要客户端删除token
	return &v1.LogoutResponse{
		Code:    200,
		Message: "登出成功",
	}, nil
}

// @Summary ChangePassword
// @Tags IAM
// @Param params body v1.ChangePasswordRequest true "修改密码请求"
// @Success 200 {object} v1.ChangePasswordResponse
// @Router /api/v1/iam/password [post]
// ChangePassword 修改密码
func (web *web) ChangePassword(ctx context.Context, req *v1.ChangePasswordRequest) (*v1.ChangePasswordResponse, error) {
	// 从context中获取用户ID（需要中间件设置）
	userID := ctx.Value("user_id")
	if userID == nil {
		return nil, errors.New("未认证")
	}

	// 转换请求类型
	iamReq := &iam.ChangePasswordRequest{
		OldPassword: req.OldPassword,
		NewPassword: req.NewPassword,
	}

	// 调用IAM服务修改密码
	err := web.iamService.ChangePassword(userID.(uint), iamReq)
	if err != nil {
		if err.Error() == "invalid old password" {
			return nil, kratoserrors.New(400, "INVALID_OLD_PASSWORD", "当前密码错误")
		}
		if err.Error() == "user not found" {
			return nil, kratoserrors.New(404, "USER_NOT_FOUND", "用户不存在")
		}
		return nil, err
	}

	return &v1.ChangePasswordResponse{
		Code:    200,
		Message: "密码修改成功",
	}, nil
}

// Health 健康检查
func (web *web) Health(ctx context.Context, req *v1.HealthRequest) (*v1.HealthResponse, error) {
	return &v1.HealthResponse{
		Status: "ok",
	}, nil
}
