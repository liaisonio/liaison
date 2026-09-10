package controlplane

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/config"
)

// connectorInstallCommands 为所有系统使用同一组服务端地址配置。
func connectorInstallCommands(manager config.Manager, accessKey, secretKey string) (string, string, error) {
	serverURL := strings.TrimSpace(manager.ServerURL)
	if serverURL == "" {
		scheme := "http"
		if manager.Listen.TLS.Enable {
			scheme = "https"
		}
		serverURL = scheme + "://" + manager.Listen.Addr
	} else if !strings.Contains(serverURL, "://") {
		serverURL = "https://" + serverURL
	}
	u, err := url.Parse(serverURL)
	if err != nil {
		return "", "", fmt.Errorf("parse manager.server_url: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", "", fmt.Errorf("manager.server_url must be an HTTP(S) service URL without credentials, query or fragment")
	}
	edgePort := manager.FrontierEdgePort
	if edgePort == 0 {
		edgePort = 30012
	}
	if edgePort < 1 || edgePort > 65535 {
		return "", "", fmt.Errorf("manager.frontier_edge_port must be between 1 and 65535")
	}
	edgeAddr := net.JoinHostPort(u.Hostname(), strconv.Itoa(edgePort))
	baseURL := strings.TrimRight(u.String(), "/")
	// HTTPS 的 host:port 格式兼容旧安装脚本；其他情况传完整 URL 保留协议和路径。
	downloadAddr := baseURL
	if u.Scheme == "https" && (u.Path == "" || u.Path == "/") {
		downloadAddr = u.Host
	}
	shQuote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'" }
	psQuote := func(value string) string { return "'" + strings.ReplaceAll(value, "'", "''") + "'" }
	unix := fmt.Sprintf("curl -k -fsSL %s | bash -s -- --access-key=%s --secret-key=%s --server-http-addr=%s --server-edge-addr=%s",
		shQuote(baseURL+"/install.sh"), shQuote(accessKey), shQuote(secretKey), shQuote(downloadAddr), shQuote(edgeAddr))
	windows := fmt.Sprintf("curl.exe -fsSL %s -o install.ps1; if ($LASTEXITCODE -eq 0) { powershell -ExecutionPolicy Bypass -File ./install.ps1 -AccessKey %s -SecretKey %s -ServerHttpAddr %s -ServerEdgeAddr %s }",
		psQuote(baseURL+"/install.ps1"), psQuote(accessKey), psQuote(secretKey), psQuote(downloadAddr), psQuote(edgeAddr))
	return unix, windows, nil
}
