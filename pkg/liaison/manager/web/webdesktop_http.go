package web

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"gorm.io/gorm"
)

const (
	webDesktopSessionTTL     = 60 * time.Second
	webDesktopDefaultWidth   = 1280
	webDesktopDefaultHeight  = 720
	webDesktopDefaultDPI     = 96
	webDesktopMaxMessageSize = 512 * 1024
	webDesktopWSBufferSize   = 128 * 1024
	webDesktopGuacdReadSize  = 128 * 1024
	webDesktopGuacdBatchMax  = 256 * 1024
	webDesktopCopyBufferSize = 128 * 1024
)

type createWebDesktopSessionRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	Domain         string `json:"domain"`
	SaveCredential bool   `json:"save_credential"`
	Width          int    `json:"width"`
	Height         int    `json:"height"`
	DPI            int    `json:"dpi"`
}

type createWebDesktopSessionResponse struct {
	Token     string `json:"token"`
	WSURL     string `json:"ws_url"`
	ExpiresAt string `json:"expires_at"`
}

type webDesktopSession struct {
	token           string
	userID          uint
	proxyID         uint
	protocol        string
	username        string
	domain          string
	password        []byte
	width           int
	height          int
	dpi             int
	saveCredential  bool
	savedCredential bool
	target          *controlplane.WebDesktopTarget
	expiresAt       time.Time
}

type webDesktopSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*webDesktopSession
	active   map[uint]map[string]context.CancelFunc
}

type guacInstruction struct {
	Opcode string
	Args   []string
	Raw    string
}

func newWebDesktopSessionStore() *webDesktopSessionStore {
	return &webDesktopSessionStore{
		sessions: map[string]*webDesktopSession{},
		active:   map[uint]map[string]context.CancelFunc{},
	}
}

func (s *webDesktopSessionStore) create(userID, proxyID uint, protocol, username, domain string, password []byte, width, height, dpi int, saveCredential, savedCredential bool) (*webDesktopSession, error) {
	token, err := randomWebSSHToken()
	if err != nil {
		return nil, err
	}
	width, height, dpi = normalizeWebDesktopSize(width, height, dpi)
	session := &webDesktopSession{
		token:           token,
		userID:          userID,
		proxyID:         proxyID,
		protocol:        protocol,
		username:        username,
		domain:          domain,
		password:        append([]byte(nil), password...),
		width:           width,
		height:          height,
		dpi:             dpi,
		saveCredential:  saveCredential,
		savedCredential: savedCredential,
		expiresAt:       time.Now().Add(webDesktopSessionTTL),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.sessions[token] = session
	return session, nil
}

func (s *webDesktopSessionStore) consume(token string) (*webDesktopSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	delete(s.sessions, token)
	if time.Now().After(session.expiresAt) {
		session.zero()
		return nil, false
	}
	return session, true
}

func (s *webDesktopSessionStore) cleanupLocked(now time.Time) {
	for token, session := range s.sessions {
		if now.After(session.expiresAt) {
			delete(s.sessions, token)
			session.zero()
		}
	}
}

func (s *webDesktopSessionStore) registerActive(proxyID uint, token string, cancel context.CancelFunc) func() {
	if cancel == nil {
		return func() {}
	}
	s.mu.Lock()
	if s.active[proxyID] == nil {
		s.active[proxyID] = map[string]context.CancelFunc{}
	}
	s.active[proxyID][token] = cancel
	s.mu.Unlock()

	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if sessions := s.active[proxyID]; sessions != nil {
			delete(sessions, token)
			if len(sessions) == 0 {
				delete(s.active, proxyID)
			}
		}
	}
}

func (s *webDesktopSessionStore) closeByProxy(proxyID uint) {
	s.mu.Lock()
	cancels := make([]context.CancelFunc, 0, len(s.active[proxyID]))
	for token, session := range s.sessions {
		if session.proxyID == proxyID {
			delete(s.sessions, token)
			session.zero()
		}
	}
	for token, cancel := range s.active[proxyID] {
		delete(s.active[proxyID], token)
		cancels = append(cancels, cancel)
	}
	delete(s.active, proxyID)
	s.mu.Unlock()

	for _, cancel := range cancels {
		cancel()
	}
}

func (s *webDesktopSession) zero() {
	for i := range s.password {
		s.password[i] = 0
	}
	s.password = nil
}

func (web *web) handleWebDesktopTargetHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDesktopProxyID(r, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebDesktopTarget(ctx, proxyID)
	if err != nil {
		status := webDesktopHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": target})
}

func (web *web) handleCreateWebDesktopSessionHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDesktopProxyID(r, "/session")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	var req createWebDesktopSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid request body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	req.Domain = strings.TrimSpace(req.Domain)
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebDesktopTarget(ctx, proxyID)
	if err != nil {
		status := webDesktopHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	if target.EffectiveStatus != "active" {
		writeJSON(w, http.StatusConflict, map[string]any{"code": http.StatusConflict, "message": target.EffectiveStatusMessage})
		return
	}
	password := []byte(req.Password)
	savedCredential := false
	if len(password) == 0 {
		credential, err := web.controlPlane.GetWebDesktopCredentialSecret(ctx, proxyID, target.Protocol, req.Username, req.Domain)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "没有保存的远程桌面密码，请输入密码"})
				return
			}
			status := webDesktopHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
		password, err = web.decryptWebSSHPassword(credential.EncryptedPassword, credential.Nonce)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "已保存密码无法解密，请清除后重新保存"})
			return
		}
		req.Username = credential.Username
		req.Domain = credential.Domain
		savedCredential = true
	} else if target.Protocol == "rdp" && req.Username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "RDP 用户名不能为空"})
		for i := range password {
			password[i] = 0
		}
		return
	}
	saveCredential := req.SaveCredential && !savedCredential
	if saveCredential && web.credentialKey == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "WebDesktop 凭据加密密钥未配置"})
		for i := range password {
			password[i] = 0
		}
		return
	}
	session, err := web.webDesktop.create(user.ID, proxyID, target.Protocol, req.Username, req.Domain, password, req.Width, req.Height, req.DPI, saveCredential, savedCredential)
	for i := range password {
		password[i] = 0
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "failed to create session"})
		return
	}
	session.target = target
	req.Password = ""
	wsPath := fmt.Sprintf("/api/v1/webdesktop/sessions/%s/connect", session.token)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    200,
		"message": "success",
		"data": createWebDesktopSessionResponse{
			Token:     session.token,
			WSURL:     web.webSSHURL(r, wsPath),
			ExpiresAt: session.expiresAt.Format(time.RFC3339),
		},
	})
}

func (web *web) handleWebDesktopCredentialHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		w.Header().Set("Allow", "DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDesktopProxyID(r, "/credential")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, targetErr := web.controlPlane.GetWebDesktopTarget(ctx, proxyID)
	if targetErr != nil {
		status := webDesktopHTTPStatus(targetErr)
		writeJSON(w, status, map[string]any{"code": status, "message": targetErr.Error()})
		return
	}
	if err := web.controlPlane.DeleteWebDesktopCredential(ctx, proxyID, r.URL.Query().Get("protocol"), r.URL.Query().Get("username"), r.URL.Query().Get("domain")); err != nil {
		web.recordWebDesktopAudit(target, user.ID, r, "delete_credential", r.URL.Query().Get("username"), r.URL.Query().Get("domain"), false, 0, err.Error())
		status := webDesktopHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	web.recordWebDesktopAudit(target, user.ID, r, "delete_credential", r.URL.Query().Get("username"), r.URL.Query().Get("domain"), true, 0, "")
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success"})
}

func (web *web) handleWebDesktopConnectHTTP(w http.ResponseWriter, r *http.Request) {
	token := parseWebDesktopSessionToken(r)
	session, ok := web.webDesktop.consume(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": http.StatusUnauthorized, "message": "invalid or expired webdesktop session"})
		return
	}
	defer session.zero()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  webDesktopWSBufferSize,
		WriteBufferSize: webDesktopWSBufferSize,
		Subprotocols:    []string{"guacamole"},
		CheckOrigin:     web.checkWebSSHOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	tuneLowLatencyTCP(conn.UnderlyingConn())
	conn.SetReadLimit(webDesktopMaxMessageSize)
	web.runWebDesktop(context.WithoutCancel(r.Context()), conn, session, r)
}

func (web *web) runWebDesktop(ctx context.Context, wsConn *websocket.Conn, session *webDesktopSession, r *http.Request) {
	started := time.Now()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	unregister := web.webDesktop.registerActive(session.proxyID, session.token, cancel)
	defer unregister()

	targetListener, err := web.startWebDesktopTargetBridge(ctx, session.proxyID)
	if err != nil {
		web.recordWebDesktopAudit(session.target, session.userID, r, "open_session", session.username, session.domain, false, time.Since(started).Milliseconds(), err.Error())
		log.Debugf("webdesktop target bridge failed: proxy_id=%d err=%v", session.proxyID, err)
		_ = wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1011, err.Error()))
		return
	}
	defer targetListener.Close()

	guacdConn, err := net.DialTimeout("tcp", web.guacdAddr, 5*time.Second)
	if err != nil {
		web.recordWebDesktopAudit(session.target, session.userID, r, "open_session", session.username, session.domain, false, time.Since(started).Milliseconds(), err.Error())
		log.Debugf("webdesktop guacd dial failed: proxy_id=%d addr=%s err=%v", session.proxyID, web.guacdAddr, err)
		_ = wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1011, "guacd 未连接或不可用"))
		return
	}
	defer guacdConn.Close()
	tuneLowLatencyTCP(guacdConn)

	if err := web.writeGuacamoleWebSocketUUID(wsConn, session.token); err != nil {
		web.recordWebDesktopAudit(session.target, session.userID, r, "open_session", session.username, session.domain, false, time.Since(started).Milliseconds(), err.Error())
		return
	}
	if err := web.handshakeGuacd(guacdConn, targetListener.Addr().(*net.TCPAddr), session); err != nil {
		web.recordWebDesktopAudit(session.target, session.userID, r, "open_session", session.username, session.domain, false, time.Since(started).Milliseconds(), err.Error())
		log.Debugf("webdesktop guacd handshake failed: proxy_id=%d err=%v", session.proxyID, err)
		_ = wsConn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(1011, err.Error()))
		return
	}
	if session.saveCredential {
		encryptedPassword, nonce, err := web.encryptWebSSHPassword(session.password)
		sessionCtx := context.WithValue(context.Background(), "user_id", session.userID)
		if err != nil {
			log.Errorf("webdesktop credential encrypt failed: proxy_id=%d err=%v", session.proxyID, err)
			web.recordWebDesktopAudit(session.target, session.userID, r, "save_credential", session.username, session.domain, false, 0, err.Error())
		} else if err := web.controlPlane.SaveWebDesktopCredential(sessionCtx, session.proxyID, session.protocol, session.username, session.domain, encryptedPassword, nonce); err != nil {
			log.Errorf("webdesktop credential save failed: proxy_id=%d err=%v", session.proxyID, err)
			web.recordWebDesktopAudit(session.target, session.userID, r, "save_credential", session.username, session.domain, false, 0, err.Error())
		} else {
			web.recordWebDesktopAudit(session.target, session.userID, r, "save_credential", session.username, session.domain, true, 0, "")
		}
	} else if session.savedCredential {
		sessionCtx := context.WithValue(context.Background(), "user_id", session.userID)
		if err := web.controlPlane.TouchWebDesktopCredential(sessionCtx, session.proxyID, session.protocol, session.username, session.domain); err != nil {
			log.Debugf("webdesktop credential touch failed: proxy_id=%d err=%v", session.proxyID, err)
		}
	}
	session.zero()
	web.recordWebDesktopAudit(session.target, session.userID, r, "open_session", session.username, session.domain, true, time.Since(started).Milliseconds(), "")
	sessionStarted := time.Now()
	defer func() {
		web.recordWebDesktopAudit(session.target, session.userID, r, "close_session", session.username, session.domain, true, time.Since(sessionStarted).Milliseconds(), "")
	}()

	errCh := make(chan error, 2)
	var wsWriteMu sync.Mutex
	go func() { errCh <- copyGuacdToWebSocket(wsConn, guacdConn, &wsWriteMu) }()
	go func() { errCh <- copyWebSocketToGuacd(wsConn, guacdConn, &wsWriteMu) }()
	select {
	case <-ctx.Done():
	case err := <-errCh:
		if err != nil && !errors.Is(err, io.EOF) && !strings.Contains(err.Error(), "close") {
			log.Debugf("webdesktop bridge ended: proxy_id=%d err=%v", session.proxyID, err)
		}
	}
}

func (web *web) recordWebDesktopAudit(target *controlplane.WebDesktopTarget, userID uint, r *http.Request, action, username, domain string, success bool, elapsedMS int64, errText string) {
	if target == nil || userID == 0 {
		return
	}
	clientIP, clientIPSource := remoteClientIPInfo(r)
	details := map[string]any{
		"remote_user":      strings.TrimSpace(username),
		"domain":           strings.TrimSpace(domain),
		"client_ip_source": clientIPSource,
	}
	statement := webDataAuditStatement(action)
	if err := web.controlPlane.RecordWebDataAudit(context.Background(), &controlplane.WebDataAudit{
		UserID:           userID,
		ProxyID:          target.ProxyID,
		ApplicationID:    target.ApplicationID,
		Protocol:         target.Protocol,
		Action:           action,
		StatementPreview: statement,
		StatementSHA256:  webDataStatementHash(statement),
		Success:          success,
		Error:            errText,
		ElapsedMS:        elapsedMS,
		ClientIP:         clientIP,
		Details:          details,
	}); err != nil {
		log.Warnf("webdesktop audit record failed: proxy_id=%d user_id=%d action=%s err=%v", target.ProxyID, userID, action, err)
	}
}

func (web *web) startWebDesktopTargetBridge(ctx context.Context, proxyID uint) (net.Listener, error) {
	listener, err := net.Listen("tcp", web.guacdBridgeAddr)
	if err != nil {
		return nil, err
	}
	go func() {
		defer listener.Close()
		accepted := make(chan net.Conn, 1)
		errCh := make(chan error, 1)
		go func() {
			conn, err := listener.Accept()
			if err != nil {
				errCh <- err
				return
			}
			accepted <- conn
		}()
		select {
		case <-ctx.Done():
			return
		case err := <-errCh:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				log.Debugf("webdesktop local accept failed: proxy_id=%d err=%v", proxyID, err)
			}
			return
		case localConn := <-accepted:
			defer localConn.Close()
			tuneLowLatencyTCP(localConn)
			targetConn, _, err := web.controlPlane.OpenWebDesktopStream(ctx, proxyID)
			if err != nil {
				log.Debugf("webdesktop target stream failed: proxy_id=%d err=%v", proxyID, err)
				return
			}
			defer targetConn.Close()
			copyConnPair(localConn, targetConn)
		}
	}()
	return listener, nil
}

func (web *web) handshakeGuacd(conn net.Conn, targetAddr *net.TCPAddr, session *webDesktopSession) error {
	reader := bufio.NewReaderSize(conn, webDesktopGuacdReadSize)
	if err := writeGuacInstruction(conn, "select", session.protocol); err != nil {
		return err
	}
	args, err := readGuacInstruction(reader)
	if err != nil {
		return err
	}
	if args.Opcode != "args" {
		return fmt.Errorf("guacd returned unexpected handshake opcode %q", args.Opcode)
	}
	if err := writeGuacInstruction(conn, "size", strconv.Itoa(session.width), strconv.Itoa(session.height), strconv.Itoa(session.dpi)); err != nil {
		return err
	}
	if err := writeGuacInstruction(conn, "audio"); err != nil {
		return err
	}
	if err := writeGuacInstruction(conn, "video"); err != nil {
		return err
	}
	if err := writeGuacInstruction(conn, "image", "image/jpeg", "image/png"); err != nil {
		return err
	}
	values := make([]string, 0, len(args.Args))
	for _, name := range args.Args {
		values = append(values, web.guacdArgValue(name, targetAddr, session))
	}
	return writeGuacInstruction(conn, "connect", values...)
}

func (web *web) guacdArgValue(name string, targetAddr *net.TCPAddr, session *webDesktopSession) string {
	normalized := strings.ToLower(strings.TrimSpace(name))
	switch normalized {
	case "hostname":
		if web.guacdBridgeHost != "" {
			return web.guacdBridgeHost
		}
		if targetAddr.IP != nil && !targetAddr.IP.IsUnspecified() {
			return targetAddr.IP.String()
		}
		return "127.0.0.1"
	case "port":
		return strconv.Itoa(targetAddr.Port)
	case "username":
		return session.username
	case "password":
		return string(session.password)
	case "domain":
		return session.domain
	case "security":
		return "any"
	case "ignore-cert":
		return "true"
	case "enable-wallpaper":
		return "false"
	case "enable-theming":
		return "false"
	case "enable-font-smoothing":
		return "false"
	case "enable-full-window-drag":
		return "false"
	case "enable-desktop-composition":
		return "false"
	case "enable-menu-animations":
		return "false"
	case "disable-bitmap-caching", "disable-offscreen-caching", "disable-glyph-caching":
		return "false"
	case "color-depth":
		return "16"
	case "force-lossless":
		return "false"
	case "disable-audio":
		return "true"
	case "enable-audio-input":
		return "false"
	case "read-only":
		return "false"
	case "width":
		return strconv.Itoa(session.width)
	case "height":
		return strconv.Itoa(session.height)
	case "dpi":
		return strconv.Itoa(session.dpi)
	default:
		if strings.HasPrefix(strings.TrimSpace(name), "VERSION_") {
			return strings.TrimSpace(name)
		}
		return ""
	}
}

func (web *web) writeGuacamoleWebSocketUUID(conn *websocket.Conn, uuid string) error {
	return conn.WriteMessage(websocket.TextMessage, []byte(formatGuacInstruction("", uuid)))
}

func copyGuacdToWebSocket(wsConn *websocket.Conn, guacdConn net.Conn, wsWriteMu *sync.Mutex) error {
	reader := bufio.NewReaderSize(guacdConn, webDesktopGuacdReadSize)
	var batch strings.Builder
	flush := func() error {
		if batch.Len() == 0 {
			return nil
		}
		data := []byte(batch.String())
		batch.Reset()
		wsWriteMu.Lock()
		err := wsConn.WriteMessage(websocket.TextMessage, data)
		wsWriteMu.Unlock()
		return err
	}
	for {
		inst, err := readGuacInstruction(reader)
		if err != nil {
			if batch.Len() > 0 {
				_ = flush()
			}
			return err
		}
		batch.WriteString(inst.Raw)
		if inst.Opcode == "sync" || batch.Len() >= webDesktopGuacdBatchMax {
			if err := flush(); err != nil {
				return err
			}
		}
	}
}

func copyWebSocketToGuacd(wsConn *websocket.Conn, guacdConn net.Conn, wsWriteMu *sync.Mutex) error {
	for {
		messageType, data, err := wsConn.ReadMessage()
		if err != nil {
			return err
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}
		if isGuacamoleTunnelInternalMessage(data) {
			wsWriteMu.Lock()
			err = wsConn.WriteMessage(websocket.TextMessage, data)
			wsWriteMu.Unlock()
			if err != nil {
				return err
			}
			continue
		}
		if _, err := guacdConn.Write(data); err != nil {
			return err
		}
	}
}

func isGuacamoleTunnelInternalMessage(data []byte) bool {
	return strings.HasPrefix(string(data), "0.,")
}

func readGuacInstruction(reader *bufio.Reader) (*guacInstruction, error) {
	var raw strings.Builder
	var elements []string
	for {
		lengthText, err := reader.ReadString('.')
		if err != nil {
			return nil, err
		}
		raw.WriteString(lengthText)
		lengthText = strings.TrimSuffix(lengthText, ".")
		length, err := strconv.Atoi(lengthText)
		if err != nil || length < 0 {
			return nil, fmt.Errorf("invalid guacamole element length %q", lengthText)
		}
		buf := make([]byte, length)
		if _, err := io.ReadFull(reader, buf); err != nil {
			return nil, err
		}
		raw.Write(buf)
		term, err := reader.ReadByte()
		if err != nil {
			return nil, err
		}
		raw.WriteByte(term)
		elements = append(elements, string(buf))
		switch term {
		case ',':
			continue
		case ';':
			if len(elements) == 0 {
				return nil, errors.New("empty guacamole instruction")
			}
			return &guacInstruction{Opcode: elements[0], Args: elements[1:], Raw: raw.String()}, nil
		default:
			return nil, fmt.Errorf("invalid guacamole element terminator %q", term)
		}
	}
}

func writeGuacInstruction(writer io.Writer, opcode string, args ...string) error {
	_, err := io.WriteString(writer, formatGuacInstruction(opcode, args...))
	return err
}

func formatGuacInstruction(opcode string, args ...string) string {
	elements := append([]string{opcode}, args...)
	var b strings.Builder
	for i, element := range elements {
		b.WriteString(strconv.Itoa(len(element)))
		b.WriteByte('.')
		b.WriteString(element)
		if i == len(elements)-1 {
			b.WriteByte(';')
		} else {
			b.WriteByte(',')
		}
	}
	return b.String()
}

func copyConnPair(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.CopyBuffer(a, b, make([]byte, webDesktopCopyBufferSize))
		_ = a.Close()
		_ = b.Close()
	}()
	go func() {
		defer wg.Done()
		_, _ = io.CopyBuffer(b, a, make([]byte, webDesktopCopyBufferSize))
		_ = a.Close()
		_ = b.Close()
	}()
	wg.Wait()
}

func tuneLowLatencyTCP(conn net.Conn) {
	tcpConn, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcpConn.SetNoDelay(true)
	_ = tcpConn.SetKeepAlive(true)
	_ = tcpConn.SetKeepAlivePeriod(30 * time.Second)
	_ = tcpConn.SetReadBuffer(webDesktopCopyBufferSize)
	_ = tcpConn.SetWriteBuffer(webDesktopCopyBufferSize)
}

func parseWebDesktopProxyID(r *http.Request, suffix string) (uint, error) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webdesktop/proxies/")
	if suffix != "" {
		if !strings.HasSuffix(path, suffix) {
			return 0, errors.New("invalid proxy id")
		}
		path = strings.TrimSuffix(path, suffix)
	} else if strings.Contains(path, "/") {
		return 0, errors.New("invalid proxy id")
	}
	id, err := strconv.ParseUint(strings.Trim(path, "/"), 10, 32)
	if err != nil || id == 0 {
		return 0, errors.New("invalid proxy id")
	}
	return uint(id), nil
}

func parseWebDesktopSessionToken(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webdesktop/sessions/")
	return strings.TrimSuffix(path, "/connect")
}

func normalizeWebDesktopSize(width, height, dpi int) (int, int, int) {
	if width < 320 {
		width = webDesktopDefaultWidth
	}
	if height < 240 {
		height = webDesktopDefaultHeight
	}
	if dpi < 36 {
		dpi = webDesktopDefaultDPI
	}
	if width > 7680 {
		width = 7680
	}
	if height > 4320 {
		height = 4320
	}
	if dpi > 240 {
		dpi = 240
	}
	return width, height, dpi
}

func webDesktopHTTPStatus(err error) int {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound
	}
	msg := err.Error()
	if strings.Contains(msg, "仅 RDP/VNC") || strings.Contains(msg, "不能为空") || strings.Contains(msg, "无效") {
		return http.StatusBadRequest
	}
	if strings.Contains(msg, "不可用") || strings.Contains(msg, "离线") || strings.Contains(msg, "禁用") {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
