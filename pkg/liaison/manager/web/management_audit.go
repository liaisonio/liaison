package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	kratoserrors "github.com/go-kratos/kratos/v2/errors"
	"github.com/go-kratos/kratos/v2/middleware"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
)

func managementAuditMiddleware(controlPlane controlplane.ControlPlane) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			httpReq, ok := kratoshttp.RequestFromServerContext(ctx)
			if !ok {
				return handler(ctx, req)
			}
			module, action, resource, auditable := classifyManagementOperation(httpReq.Method, httpReq.URL.Path)
			started := time.Now()
			reply, err := handler(ctx, req)
			if !auditable {
				return reply, err
			}
			userID, _ := ctx.Value("user_id").(uint)
			if userID == 0 {
				return reply, err
			}
			statusCode := http.StatusOK
			if err != nil {
				statusCode = int(kratoserrors.Code(err))
				if statusCode == 0 {
					statusCode = http.StatusInternalServerError
				}
			}
			if recordErr := controlPlane.RecordManagementAudit(ctx, &controlplane.ManagementAudit{UserID: userID, Module: module, Action: action, Resource: resource, Method: httpReq.Method, ClientIP: iam.ExtractClientIP(httpReq), Success: err == nil, StatusCode: statusCode, ElapsedMS: time.Since(started).Milliseconds()}); recordErr != nil {
				log.Warnf("management audit record failed: module=%s action=%s user_id=%d err=%v", module, action, userID, recordErr)
			}
			return reply, err
		}
	}
}

func classifyManagementOperation(method, path string) (module, action, resource string, ok bool) {
	method = strings.ToUpper(strings.TrimSpace(method))
	path = strings.TrimSpace(path)
	if method != http.MethodPost && method != http.MethodPut && method != http.MethodPatch && method != http.MethodDelete {
		return "", "", "", false
	}
	if strings.HasPrefix(path, "/api/v1/webssh/") || strings.HasPrefix(path, "/api/v1/webdesktop/") || strings.HasPrefix(path, "/api/v1/webdata/") || strings.HasPrefix(path, "/api/v1/audits/") {
		return "", "", "", false
	}
	switch {
	case strings.HasPrefix(path, "/api/v1/edges"):
		module = "connector"
	case strings.HasPrefix(path, "/api/v1/devices"):
		module = "device"
	case strings.HasPrefix(path, "/api/v1/applications"):
		module = "application"
	case strings.HasPrefix(path, "/api/v1/proxies"):
		module = "access"
	case strings.HasPrefix(path, "/api/v1/iam/tokens"):
		module = "token"
	case strings.HasPrefix(path, "/api/v1/iam/users"):
		module = "user"
	case strings.HasPrefix(path, "/api/v1/iam/organizations"):
		module = "organization"
	case path == "/api/v1/iam/password":
		module = "account"
	case path == "/api/v1/iam/logout":
		module = "account"
	default:
		return "", "", "", false
	}
	switch method {
	case http.MethodPost:
		action = "create"
	case http.MethodPut, http.MethodPatch:
		action = "update"
	case http.MethodDelete:
		action = "delete"
	}
	if path == "/api/v1/iam/password" {
		action = "change_password"
	} else if path == "/api/v1/iam/logout" {
		action = "logout"
	} else if strings.HasSuffix(path, "/scan_application_tasks") {
		action = "scan_applications"
	} else if strings.HasSuffix(path, "/firewall") {
		module = "firewall"
		if method == http.MethodDelete {
			action = "restore_default"
		} else {
			action = "update"
		}
	}
	return module, action, path, true
}

func (web *web) recordLoginManagementAudit(ctx context.Context, email string, userID uint, clientIP string, success bool, statusCode int) {
	if err := web.controlPlane.RecordManagementAudit(ctx, &controlplane.ManagementAudit{UserID: userID, UserEmail: strings.TrimSpace(email), Module: "account", Action: "login", Resource: "/api/v1/iam/login", Method: http.MethodPost, ClientIP: strings.TrimSpace(clientIP), Success: success, StatusCode: statusCode}); err != nil {
		log.Warnf("login management audit record failed: user_id=%d err=%v", userID, err)
	}
}
