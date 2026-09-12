package controlplane

import (
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func normalizeAccessProtocol(raw string, application *model.Application, requestedPort int, exposePublicPort bool) (model.AccessProtocol, error) {
	protocol := model.AccessProtocol(strings.ToLower(strings.TrimSpace(raw)))
	if protocol == "" {
		switch {
		case application != nil && application.ApplicationType == model.ApplicationTypeHTTP:
			protocol = model.AccessProtocolHTTP
		case application != nil && isWebOnlyCapableApplicationType(application.ApplicationType) && requestedPort == 0 && !exposePublicPort:
			if application.ApplicationType == model.ApplicationTypeSSH {
				protocol = model.AccessProtocolWebSSH
			} else {
				protocol = model.AccessProtocolWeb
			}
		default:
			protocol = model.AccessProtocolTCP
		}
	}
	if err := validateAccessProtocol(protocol, application); err != nil {
		return "", err
	}
	return protocol, nil
}

func validateAccessProtocol(protocol model.AccessProtocol, application *model.Application) error {
	if application == nil {
		return notFound("APPLICATION_NOT_FOUND", "关联应用不存在", nil)
	}
	switch protocol {
	case model.AccessProtocolAI:
		if application.ApplicationType != model.ApplicationTypeLLM {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "AI API requires an LLM application")
		}
	case model.AccessProtocolTCP:
		// TCP is the protocol-agnostic escape hatch. Every application can be
		// exposed as an opaque L4 stream without invoking an L7/Web handler.
	case model.AccessProtocolHTTP:
		if application.ApplicationType != model.ApplicationTypeHTTP {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "HTTP 访问只能关联 HTTP 应用")
		}
	case model.AccessProtocolSSH, model.AccessProtocolWebSSH:
		if application.ApplicationType != model.ApplicationTypeSSH {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "SSH 访问只能关联 SSH 应用")
		}
	case model.AccessProtocolRDP:
		if application.ApplicationType != model.ApplicationTypeRDP {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "RDP 访问只能关联 RDP 应用")
		}
	case model.AccessProtocolVNC:
		if application.ApplicationType != model.ApplicationTypeVNC {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "VNC 访问只能关联 VNC 应用")
		}
	case model.AccessProtocolMySQL:
		if application.ApplicationType != model.ApplicationTypeMySQL {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "MySQL 访问只能关联 MySQL 应用")
		}
	case model.AccessProtocolPostgreSQL:
		if application.ApplicationType != model.ApplicationTypePostgreSQL {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "PostgreSQL 访问只能关联 PostgreSQL 应用")
		}
	case model.AccessProtocolRedis:
		if application.ApplicationType != model.ApplicationTypeRedis {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "Redis 访问只能关联 Redis 应用")
		}
	case model.AccessProtocolMongoDB:
		if application.ApplicationType != model.ApplicationTypeMongoDB {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "MongoDB 访问只能关联 MongoDB 应用")
		}
	case model.AccessProtocolWeb:
		if !isWebOnlyCapableApplicationType(application.ApplicationType) {
			return badRequest("ACCESS_PROTOCOL_MISMATCH", "该应用不支持网页访问")
		}
	default:
		return badRequest("ACCESS_PROTOCOL_INVALID", "访问协议无效")
	}
	return nil
}

func effectiveAccessProtocol(proxy *model.Proxy, application *model.Application) model.AccessProtocol {
	if proxy != nil {
		switch proxy.AccessProtocol {
		case model.AccessProtocolTCP, model.AccessProtocolHTTP, model.AccessProtocolSSH,
			model.AccessProtocolRDP, model.AccessProtocolVNC, model.AccessProtocolMySQL,
			model.AccessProtocolPostgreSQL, model.AccessProtocolRedis, model.AccessProtocolMongoDB,
			model.AccessProtocolWebSSH, model.AccessProtocolWeb, model.AccessProtocolAI:
			return proxy.AccessProtocol
		}
	}
	if application != nil && application.ApplicationType == model.ApplicationTypeHTTP {
		return model.AccessProtocolHTTP
	}
	if proxy != nil && proxy.Port == 0 && application != nil && isWebOnlyCapableApplicationType(application.ApplicationType) {
		if application.ApplicationType == model.ApplicationTypeSSH {
			return model.AccessProtocolWebSSH
		}
		return model.AccessProtocolWeb
	}
	return model.AccessProtocolTCP
}

func accessProtocolRequiresPublicPort(protocol model.AccessProtocol) bool {
	return protocol != model.AccessProtocolWebSSH && protocol != model.AccessProtocolWeb && protocol != model.AccessProtocolAI
}
