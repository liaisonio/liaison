package web

import (
	"crypto/sha256"
	"errors"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-kratos/kratos/v2"
	"github.com/go-kratos/kratos/v2/middleware/recovery"
	kratoshttp "github.com/go-kratos/kratos/v2/transport/http"
	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/utils"
)

// @title Liaison Swagger API
// @version 1.0
// @description Liaison Swagger API
// @contact.name Austin Zhai
// @contact.email singchia@163.com
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
type Web interface {
	Serve() error
	Close() error
}

type web struct {
	app *kratos.App

	// deps
	controlPlane    controlplane.ControlPlane
	iamService      *iam.IAMService
	webSSH          *webSSHSessionStore
	webDesktop      *webDesktopSessionStore
	webData         *webDataSessionStore
	credentialKey   []byte
	guacdAddr       string
	guacdBridgeAddr string
	guacdBridgeHost string
}

func NewWebServer(conf *config.Configuration, controlPlane controlplane.ControlPlane, iamService *iam.IAMService) (Web, error) {
	ln, err := NewListener(conf)
	if err != nil {
		return nil, err
	}
	web, err := NewWebServerWithListener(conf, controlPlane, iamService, ln)
	if err != nil {
		_ = ln.Close()
		return nil, err
	}
	return web, nil
}

func NewListener(conf *config.Configuration) (net.Listener, error) {
	return utils.Listen(&conf.Manager.Listen)
}

func NewWebServerWithListener(conf *config.Configuration, controlPlane controlplane.ControlPlane, iamService *iam.IAMService, ln net.Listener) (Web, error) {
	if ln == nil {
		return nil, errors.New("web listener is nil")
	}
	credentialKey := deriveWebSSHCredentialKey(conf)
	web := &web{
		controlPlane:    controlPlane,
		iamService:      iamService,
		webSSH:          newWebSSHSessionStore(),
		webDesktop:      newWebDesktopSessionStore(),
		webData:         newWebDataSessionStore(),
		credentialKey:   credentialKey,
		guacdAddr:       managerGuacdAddr(conf),
		guacdBridgeAddr: managerGuacdBridgeAddr(conf),
		guacdBridgeHost: managerGuacdBridgeHost(conf),
	}
	// 创建认证中间件
	authMiddleware := iam.AuthMiddleware(web.iamService)

	opts := []kratoshttp.ServerOption{
		kratoshttp.Middleware(recovery.Recovery()),
		kratoshttp.Middleware(authMiddleware),
		kratoshttp.Listener(ln),
	}
	srv := kratoshttp.NewServer(opts...)
	v1.RegisterLiaisonServiceHTTPServer(srv, web)

	// PAT 管理
	srv.HandleFunc("/api/v1/iam/tokens", web.handleTokensHTTP)
	srv.HandleFunc("/api/v1/iam/tokens/{id}", web.handleTokenByIDHTTP)

	// 公开：返回调用方的出口 IP（用于前端防火墙面板的「我的 IP」一键加白）
	srv.HandleFunc("/api/v1/iam/client_ip", web.handleClientIP)

	// 代理防火墙
	srv.HandleFunc("/api/v1/proxies/{id}/firewall", web.handleFirewallHTTP)

	// WebSSH
	srv.HandleFunc("/api/v1/webssh/proxies/{id}", web.handleWebSSHTargetHTTP)
	srv.HandleFunc("/api/v1/webssh/proxies/{id}/session", web.handleCreateWebSSHSessionHTTP)
	srv.HandleFunc("/api/v1/webssh/proxies/{id}/credential", web.handleWebSSHCredentialHTTP)
	srv.HandleFunc("/api/v1/webssh/proxies/{id}/host-key", web.handleWebSSHHostKeyHTTP)
	srv.HandleFunc("/api/v1/webssh/sessions/{token}/connect", web.handleWebSSHConnectHTTP)

	// WebDesktop (RDP/VNC)
	srv.HandleFunc("/api/v1/webdesktop/proxies/{id}", web.handleWebDesktopTargetHTTP)
	srv.HandleFunc("/api/v1/webdesktop/proxies/{id}/session", web.handleCreateWebDesktopSessionHTTP)
	srv.HandleFunc("/api/v1/webdesktop/proxies/{id}/credential", web.handleWebDesktopCredentialHTTP)
	srv.HandleFunc("/api/v1/webdesktop/sessions/{token}/connect", web.handleWebDesktopConnectHTTP)

	// WebData (MySQL/PostgreSQL/Redis/MongoDB)
	srv.HandleFunc("/api/v1/webdata/proxies/{id}", web.handleWebDataTargetHTTP)
	srv.HandleFunc("/api/v1/webdata/proxies/{id}/session", web.handleCreateWebDataSessionHTTP)
	srv.HandleFunc("/api/v1/webdata/proxies/{id}/test", web.handleTestWebDataConnectionHTTP)
	srv.HandleFunc("/api/v1/webdata/proxies/{id}/credential", web.handleWebDataCredentialHTTP)
	srv.HandleFunc("/api/v1/webdata/proxies/{id}/audits", web.handleWebDataAuditsHTTP)
	srv.HandleFunc("/api/v1/webdata/sessions/{token}/execute", web.handleWebDataSessionHTTP)
	srv.HandleFunc("/api/v1/webdata/sessions/{token}/metadata", web.handleWebDataMetadataHTTP)
	srv.HandleFunc("/api/v1/webdata/sessions/{token}/object", web.handleWebDataObjectHTTP)
	srv.HandleFunc("/api/v1/webdata/sessions/{token}", web.handleWebDataSessionHTTP)

	// Audit
	srv.HandleFunc("/api/v1/audits/access", web.handleAccessAuditListHTTP)
	srv.HandleFunc("/api/v1/audits/webdata", web.handleWebDataAuditListHTTP)

	// 文件服务
	if err := web.serveFiles(conf, srv); err != nil {
		return nil, err
	}

	web.app = kratos.New(
		kratos.Name("liaison"),
		kratos.Server(srv),
	)

	return web, nil
}

func (web *web) serveFiles(conf *config.Configuration, srv *kratoshttp.Server) error {
	// 安装包服务 - edge 文件在 /opt/liaison/edge 目录
	edgeDir := "/opt/liaison/edge"
	// 确保 edgeDir 是绝对路径
	edgeDirAbs, err := filepath.Abs(edgeDir)
	if err != nil {
		return err
	}

	// 安装脚本服务 - 从 /opt/liaison/edge/install.sh 提供服务
	installScriptPath := filepath.Join(edgeDirAbs, "install.sh")
	if _, err := os.Stat(installScriptPath); err == nil {
		srv.HandleFunc("/install.sh", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, installScriptPath)
		})
	}

	// PowerShell 安装脚本服务 - 从 /opt/liaison/edge/install.ps1 提供服务
	installPSScriptPath := filepath.Join(edgeDirAbs, "install.ps1")
	if _, err := os.Stat(installPSScriptPath); err == nil {
		srv.HandleFunc("/install.ps1", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, installPSScriptPath)
		})
	}

	// Windows 批处理安装脚本服务 - 从 /opt/liaison/edge/install.bat 提供服务
	installBatScriptPath := filepath.Join(edgeDirAbs, "install.bat")
	if _, err := os.Stat(installBatScriptPath); err == nil {
		srv.HandleFunc("/install.bat", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, installBatScriptPath)
		})
	}

	// 卸载脚本服务 - 从 /opt/liaison/edge/uninstall.sh 提供服务
	uninstallScriptPath := filepath.Join(edgeDirAbs, "uninstall.sh")
	if _, err := os.Stat(uninstallScriptPath); err == nil {
		srv.HandleFunc("/uninstall.sh", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, uninstallScriptPath)
		})
	}

	// PowerShell 卸载脚本服务 - 从 /opt/liaison/edge/uninstall.ps1 提供服务
	uninstallPSScriptPath := filepath.Join(edgeDirAbs, "uninstall.ps1")
	if _, err := os.Stat(uninstallPSScriptPath); err == nil {
		srv.HandleFunc("/uninstall.ps1", func(w http.ResponseWriter, r *http.Request) {
			http.ServeFile(w, r, uninstallPSScriptPath)
		})
	}

	// Edge 安装包文件服务
	if _, err := os.Stat(edgeDirAbs); err == nil {
		// http.FileServer 使用绝对路径时，内置了路径穿越保护（会拒绝包含 .. 的路径）
		fileServer := http.FileServer(http.Dir(edgeDirAbs))
		srv.HandlePrefix("/packages/edge/", http.StripPrefix("/packages/edge/", fileServer))
	}

	// 前端文件服务
	webDir := conf.Manager.WebDir
	// 确保 webDir 是绝对路径
	webDirAbs, err := filepath.Abs(webDir)
	if err != nil {
		return err
	}
	if _, err := os.Stat(webDirAbs); err != nil {
		return err
	}
	fileServer := http.FileServer(http.Dir(webDirAbs))
	// 前端文件服务：作为 fallback，处理所有非 API、非 install.sh、非 install.ps1、非 uninstall.sh、非 packages 的路径
	srv.HandlePrefix("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		// 如果不是 API、install.sh、install.ps1、install.bat、uninstall.sh、uninstall.ps1 和 packages 路径，就走 web 服务
		if !strings.HasPrefix(path, "/api/") &&
			path != "/install.sh" &&
			path != "/install.ps1" &&
			path != "/install.bat" &&
			path != "/uninstall.sh" &&
			path != "/uninstall.ps1" &&
			!strings.HasPrefix(path, "/packages/") {
			setWebStaticNoCache(w)
			// 检查文件是否存在

			filePath := filepath.Join(webDirAbs, path)
			if info, err := os.Stat(filePath); err == nil && !info.IsDir() {
				// 文件存在，直接使用 FileServer
				fileServer.ServeHTTP(w, r)
				return
			}
			// 文件不存在，返回 index.html（SPA 路由）
			http.ServeFile(w, r, filepath.Join(webDirAbs, "index.html"))
			return
		}
		// 其他路径由其他处理器处理，这里不做处理
	}))
	return nil
}

func setWebStaticNoCache(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.Header().Set("Pragma", "no-cache")
	w.Header().Set("Expires", "0")
}

func (web *web) Serve() error {
	err := web.app.Run()
	if err != nil {
		return err
	}
	return nil
}

func (web *web) Close() error {
	return web.app.Stop()
}

func deriveWebSSHCredentialKey(conf *config.Configuration) []byte {
	secret := ""
	if conf != nil {
		secret = strings.TrimSpace(conf.Manager.CredentialSecret)
		if secret == "" {
			secret = strings.TrimSpace(conf.Manager.JWTSecret)
		}
	}
	if secret == "" {
		return nil
	}
	sum := sha256.Sum256([]byte("liaison-webssh-credential:" + secret))
	return sum[:]
}

func managerGuacdAddr(conf *config.Configuration) string {
	if conf == nil || strings.TrimSpace(conf.Manager.GuacdAddr) == "" {
		return "127.0.0.1:4822"
	}
	return strings.TrimSpace(conf.Manager.GuacdAddr)
}

func managerGuacdBridgeAddr(conf *config.Configuration) string {
	if conf == nil || strings.TrimSpace(conf.Manager.GuacdBridgeAddr) == "" {
		return "127.0.0.1:0"
	}
	return strings.TrimSpace(conf.Manager.GuacdBridgeAddr)
}

func managerGuacdBridgeHost(conf *config.Configuration) string {
	if conf == nil {
		return ""
	}
	return strings.TrimSpace(conf.Manager.GuacdBridgeHost)
}
