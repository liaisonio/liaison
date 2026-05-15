package web

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"golang.org/x/crypto/ssh"
	"gorm.io/gorm"
)

const (
	webSSHSessionTTL     = 60 * time.Second
	webSSHDefaultColumns = 120
	webSSHDefaultRows    = 32
	webSSHMaxMessageSize = 128 * 1024
	webSSHHeartbeatTTL   = 75 * time.Second
	webSSHWSBufferSize   = 32 * 1024
	webSSHOutputReadSize = 32 * 1024
	webSSHOutputBatchMax = 64 * 1024
	webSSHOutputFlush    = 4 * time.Millisecond
	webSSHMaxAuditRunes  = 4096
)

type createWebSSHSessionRequest struct {
	Username       string `json:"username"`
	Password       string `json:"password"`
	SaveCredential bool   `json:"save_credential"`
	Cols           int    `json:"cols"`
	Rows           int    `json:"rows"`
}

type createWebSSHSessionResponse struct {
	Token     string `json:"token"`
	WSURL     string `json:"ws_url"`
	ExpiresAt string `json:"expires_at"`
}

type webSSHSession struct {
	token           string
	userID          uint
	proxyID         uint
	username        string
	password        []byte
	cols            int
	rows            int
	saveCredential  bool
	savedCredential bool
	expiresAt       time.Time
}

type webSSHSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*webSSHSession
}

type webSSHClientMessage struct {
	Type string `json:"type"`
	Data string `json:"data,omitempty"`
	Cols int    `json:"cols,omitempty"`
	Rows int    `json:"rows,omitempty"`
}

type webSSHServerMessage struct {
	Type    string `json:"type"`
	Status  string `json:"status,omitempty"`
	Data    string `json:"data,omitempty"`
	Message string `json:"message,omitempty"`
}

type webSSHCommandCollector struct {
	mu               sync.Mutex
	buffer           []rune
	escapeState      int
	suppressNextLine bool
}

type webSSHWSWriter struct {
	conn *websocket.Conn
	mu   sync.Mutex
}

func newWebSSHSessionStore() *webSSHSessionStore {
	return &webSSHSessionStore{sessions: map[string]*webSSHSession{}}
}

func (s *webSSHSessionStore) create(userID, proxyID uint, username string, password []byte, cols, rows int, saveCredential, savedCredential bool) (*webSSHSession, error) {
	token, err := randomWebSSHToken()
	if err != nil {
		return nil, err
	}
	session := &webSSHSession{
		token:           token,
		userID:          userID,
		proxyID:         proxyID,
		username:        username,
		password:        append([]byte(nil), password...),
		cols:            cols,
		rows:            rows,
		saveCredential:  saveCredential,
		savedCredential: savedCredential,
		expiresAt:       time.Now().Add(webSSHSessionTTL),
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.sessions[token] = session
	return session, nil
}

func (s *webSSHSessionStore) consume(token string) (*webSSHSession, bool) {
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

func (s *webSSHSessionStore) cleanupLocked(now time.Time) {
	for token, session := range s.sessions {
		if now.After(session.expiresAt) {
			delete(s.sessions, token)
			session.zero()
		}
	}
}

func (s *webSSHSession) zero() {
	for i := range s.password {
		s.password[i] = 0
	}
	s.password = nil
}

func (w *webSSHWSWriter) write(msg webSSHServerMessage) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.conn.WriteJSON(msg)
}

func (web *web) handleWebSSHTargetHTTP(w http.ResponseWriter, r *http.Request) {
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
	proxyID, err := parseWebSSHProxyID(r, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebSSHTarget(ctx, proxyID)
	if err != nil {
		status := webSSHHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": target})
}

func (web *web) handleCreateWebSSHSessionHTTP(w http.ResponseWriter, r *http.Request) {
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
	proxyID, err := parseWebSSHProxyID(r, "/session")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	var req createWebSSHSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid request body"})
		return
	}
	req.Username = strings.TrimSpace(req.Username)
	cols, rows := normalizeWebSSHSize(req.Cols, req.Rows)
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebSSHTarget(ctx, proxyID)
	if err != nil {
		status := webSSHHTTPStatus(err)
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
		if req.Username == "" {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "请选择已保存用户或输入密码"})
			return
		}
		credential, err := web.controlPlane.GetWebSSHCredentialSecret(ctx, proxyID, req.Username)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "该 SSH 用户没有保存密码，请输入密码"})
				return
			}
			status := webSSHHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
		password, err = web.decryptWebSSHPassword(credential.EncryptedPassword, credential.Nonce)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "已保存密码无法解密，请清除后重新保存"})
			return
		}
		req.Username = credential.Username
		savedCredential = true
	} else if req.Username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "SSH 用户名不能为空"})
		for i := range password {
			password[i] = 0
		}
		return
	}
	if req.Username == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "SSH 用户名不能为空"})
		for i := range password {
			password[i] = 0
		}
		return
	}
	saveCredential := req.SaveCredential && !savedCredential
	if saveCredential && web.credentialKey == nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "WebSSH 凭据加密密钥未配置"})
		for i := range password {
			password[i] = 0
		}
		return
	}
	session, err := web.webSSH.create(user.ID, proxyID, req.Username, password, cols, rows, saveCredential, savedCredential)
	for i := range password {
		password[i] = 0
	}
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "failed to create session"})
		return
	}
	req.Password = ""
	wsPath := fmt.Sprintf("/api/v1/webssh/sessions/%s/connect", session.token)
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    200,
		"message": "success",
		"data": createWebSSHSessionResponse{
			Token:     session.token,
			WSURL:     web.webSSHURL(r, wsPath),
			ExpiresAt: session.expiresAt.Format(time.RFC3339),
		},
	})
}

func (web *web) handleWebSSHCredentialHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodDelete {
		w.Header().Set("Allow", "GET, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebSSHProxyID(r, "/credential")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	if r.Method == http.MethodDelete {
		if err := web.controlPlane.DeleteWebSSHCredential(ctx, proxyID, r.URL.Query().Get("username")); err != nil {
			status := webSSHHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success"})
		return
	}
	credentials, err := web.controlPlane.GetWebSSHCredentials(ctx, proxyID)
	if err != nil {
		status := webSSHHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": credentials})
}

func (web *web) handleWebSSHHostKeyHTTP(w http.ResponseWriter, r *http.Request) {
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
	proxyID, err := parseWebSSHProxyID(r, "/host-key")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	if err := web.controlPlane.DeleteWebSSHHostKey(ctx, proxyID); err != nil {
		status := webSSHHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success"})
}

func (web *web) handleWebSSHConnectHTTP(w http.ResponseWriter, r *http.Request) {
	token := parseWebSSHSessionToken(r)
	session, ok := web.webSSH.consume(token)
	if !ok {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": http.StatusUnauthorized, "message": "invalid or expired webssh session"})
		return
	}
	defer session.zero()
	upgrader := websocket.Upgrader{
		ReadBufferSize:  webSSHWSBufferSize,
		WriteBufferSize: webSSHWSBufferSize,
		CheckOrigin:     web.checkWebSSHOrigin,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetReadLimit(webSSHMaxMessageSize)
	writer := &webSSHWSWriter{conn: conn}
	// WebSocket sessions outlive the original HTTP request timeout. The
	// connection is closed by the websocket read loop, SSH wait, or server
	// shutdown path instead of the short request context.
	clientIP, clientIPSource := remoteClientIPInfo(r)
	web.runWebSSH(context.WithoutCancel(r.Context()), writer, conn, session, clientIP, clientIPSource)
}

func (web *web) runWebSSH(ctx context.Context, writer *webSSHWSWriter, wsConn *websocket.Conn, webSession *webSSHSession, clientIP, clientIPSource string) {
	_ = writer.write(webSSHServerMessage{Type: "status", Status: "connecting"})
	log.Debugf("webssh session starting: proxy_id=%d user_id=%d", webSession.proxyID, webSession.userID)
	connectStarted := time.Now()
	targetConn, target, err := web.controlPlane.OpenWebSSHStream(ctx, webSession.proxyID)
	if err != nil {
		log.Debugf("webssh stream open failed: proxy_id=%d err=%v", webSession.proxyID, err)
		web.recordWebSSHAudit(target, webSession.userID, clientIP, clientIPSource, "open_session", webSession.username, "", false, time.Since(connectStarted).Milliseconds(), err.Error())
		_ = writer.write(webSSHServerMessage{Type: "error", Message: err.Error()})
		return
	}
	defer targetConn.Close()

	password := string(webSession.password)
	sshConfig := &ssh.ClientConfig{
		User:            webSession.username,
		Auth:            []ssh.AuthMethod{ssh.Password(password)},
		HostKeyCallback: web.webSSHHostKeyCallback(webSession.proxyID),
		Timeout:         10 * time.Second,
	}
	targetAddr := net.JoinHostPort(target.TargetHost, strconv.Itoa(target.TargetPort))
	sshConn, chans, reqs, err := ssh.NewClientConn(targetConn, targetAddr, sshConfig)
	password = ""
	if err != nil {
		webSession.zero()
		log.Debugf("webssh ssh handshake failed: proxy_id=%d err=%v", webSession.proxyID, err)
		message := webSSHConnectionError(err)
		web.recordWebSSHAudit(target, webSession.userID, clientIP, clientIPSource, "open_session", webSession.username, "", false, time.Since(connectStarted).Milliseconds(), message)
		_ = writer.write(webSSHServerMessage{Type: "error", Message: message})
		return
	}
	if webSession.saveCredential {
		encryptedPassword, nonce, err := web.encryptWebSSHPassword(webSession.password)
		sessionCtx := context.WithValue(context.Background(), "user_id", webSession.userID)
		if err != nil {
			log.Errorf("webssh credential encrypt failed: proxy_id=%d err=%v", webSession.proxyID, err)
			_ = writer.write(webSSHServerMessage{Type: "credential_error", Message: "SSH 已连接，但保存密码失败"})
		} else if err := web.controlPlane.SaveWebSSHCredential(sessionCtx, webSession.proxyID, webSession.username, encryptedPassword, nonce); err != nil {
			log.Errorf("webssh credential save failed: proxy_id=%d err=%v", webSession.proxyID, err)
			_ = writer.write(webSSHServerMessage{Type: "credential_error", Message: "SSH 已连接，但保存密码失败"})
		} else {
			_ = writer.write(webSSHServerMessage{Type: "credential_saved"})
		}
	} else if webSession.savedCredential {
		sessionCtx := context.WithValue(context.Background(), "user_id", webSession.userID)
		if err := web.controlPlane.TouchWebSSHCredential(sessionCtx, webSession.proxyID, webSession.username); err != nil {
			log.Debugf("webssh credential touch failed: proxy_id=%d err=%v", webSession.proxyID, err)
		}
	}
	webSession.zero()
	client := ssh.NewClient(sshConn, chans, reqs)
	defer client.Close()

	terminalSession, err := client.NewSession()
	if err != nil {
		_ = writer.write(webSSHServerMessage{Type: "error", Message: "SSH 会话创建失败"})
		return
	}
	defer terminalSession.Close()

	stdin, err := terminalSession.StdinPipe()
	if err != nil {
		_ = writer.write(webSSHServerMessage{Type: "error", Message: "SSH 输入通道创建失败"})
		return
	}
	stdout, err := terminalSession.StdoutPipe()
	if err != nil {
		_ = writer.write(webSSHServerMessage{Type: "error", Message: "SSH 输出通道创建失败"})
		return
	}
	stderr, err := terminalSession.StderrPipe()
	if err != nil {
		_ = writer.write(webSSHServerMessage{Type: "error", Message: "SSH 错误输出通道创建失败"})
		return
	}
	cols, rows := normalizeWebSSHSize(webSession.cols, webSession.rows)
	modes := ssh.TerminalModes{
		ssh.ECHO:          1,
		ssh.TTY_OP_ISPEED: 14400,
		ssh.TTY_OP_OSPEED: 14400,
	}
	if err := terminalSession.RequestPty("xterm-256color", rows, cols, modes); err != nil {
		_ = writer.write(webSSHServerMessage{Type: "error", Message: "SSH PTY 创建失败"})
		return
	}
	if err := terminalSession.Shell(); err != nil {
		log.Debugf("webssh shell start failed: proxy_id=%d err=%v", webSession.proxyID, err)
		web.recordWebSSHAudit(target, webSession.userID, clientIP, clientIPSource, "open_session", webSession.username, "", false, time.Since(connectStarted).Milliseconds(), "SSH Shell 启动失败")
		_ = writer.write(webSSHServerMessage{Type: "error", Message: "SSH Shell 启动失败"})
		return
	}
	log.Debugf("webssh shell started: proxy_id=%d target=%s", webSession.proxyID, targetAddr)
	_ = writer.write(webSSHServerMessage{Type: "status", Status: "connected"})
	web.recordWebSSHAudit(target, webSession.userID, clientIP, clientIPSource, "open_session", webSession.username, "", true, time.Since(connectStarted).Milliseconds(), "")
	sessionStarted := time.Now()
	closeAudited := false
	auditClose := func() {
		if closeAudited {
			return
		}
		closeAudited = true
		web.recordWebSSHAudit(target, webSession.userID, clientIP, clientIPSource, "close_session", webSession.username, "", true, time.Since(sessionStarted).Milliseconds(), "")
	}
	defer auditClose()

	commandCollector := &webSSHCommandCollector{}
	done := make(chan struct{})
	go web.copyWebSSHOutput(writer, stdout, done, commandCollector.observeOutput)
	go web.copyWebSSHOutput(writer, stderr, done, commandCollector.observeOutput)
	waitDone := make(chan error, 1)
	go func() {
		waitDone <- terminalSession.Wait()
	}()
	clientMessages := make(chan webSSHClientMessage, 16)
	clientErrors := make(chan error, 1)
	refreshWebSSHReadDeadline(wsConn)
	go func() {
		for {
			var msg webSSHClientMessage
			if err := wsConn.ReadJSON(&msg); err != nil {
				log.Debugf("webssh websocket read ended: proxy_id=%d err=%v", webSession.proxyID, err)
				clientErrors <- err
				return
			}
			refreshWebSSHReadDeadline(wsConn)
			select {
			case clientMessages <- msg:
			case <-ctx.Done():
				return
			}
		}
	}()
	recordCommand := func(command string) {
		web.recordWebSSHAudit(target, webSession.userID, clientIP, clientIPSource, "execute", webSession.username, command, true, 0, "")
	}

	for {
		select {
		case <-ctx.Done():
			log.Debugf("webssh session closing: proxy_id=%d reason=context err=%v", webSession.proxyID, ctx.Err())
			_ = terminalSession.Close()
			close(done)
			return
		case err := <-waitDone:
			log.Debugf("webssh session closing: proxy_id=%d reason=ssh_wait err=%v", webSession.proxyID, err)
			_ = writer.write(webSSHServerMessage{Type: "status", Status: "closed"})
			close(done)
			return
		case err := <-clientErrors:
			log.Debugf("webssh session closing: proxy_id=%d reason=websocket err=%v", webSession.proxyID, err)
			_ = terminalSession.Close()
			close(done)
			return
		case msg := <-clientMessages:
			switch msg.Type {
			case "input":
				if msg.Data != "" {
					if _, err := io.WriteString(stdin, msg.Data); err == nil {
						commandCollector.feed(msg.Data, recordCommand)
					} else {
						log.Debugf("webssh stdin write failed: proxy_id=%d err=%v", webSession.proxyID, err)
					}
				}
			case "resize":
				cols, rows = normalizeWebSSHSize(msg.Cols, msg.Rows)
				log.Debugf("webssh resize received: proxy_id=%d cols=%d rows=%d", webSession.proxyID, cols, rows)
				_ = terminalSession.WindowChange(rows, cols)
			case "ping":
				_ = writer.write(webSSHServerMessage{Type: "pong"})
			}
		}
	}
}

func (c *webSSHCommandCollector) feed(data string, emit func(string)) {
	if c == nil || emit == nil || data == "" {
		return
	}
	commands := make([]string, 0, 1)
	c.mu.Lock()
	for _, r := range data {
		if c.consumeEscape(r) {
			continue
		}
		switch r {
		case '\x1b':
			c.escapeState = 1
		case '\r', '\n':
			c.flushLocked(&commands)
		case '\b', '\x7f':
			if len(c.buffer) > 0 {
				c.buffer = c.buffer[:len(c.buffer)-1]
			}
		case '\x03', '\x15', '\x18', '\x1a':
			c.buffer = c.buffer[:0]
		default:
			if r >= 0x20 && r != 0x7f && len(c.buffer) < webSSHMaxAuditRunes {
				c.buffer = append(c.buffer, r)
			}
		}
	}
	c.mu.Unlock()
	for _, command := range commands {
		emit(command)
	}
}

func (c *webSSHCommandCollector) consumeEscape(r rune) bool {
	switch c.escapeState {
	case 0:
		return false
	case 1:
		if r == '[' || r == 'O' {
			c.escapeState = 2
			return true
		}
		c.escapeState = 0
		return true
	case 2:
		if r >= 0x40 && r <= 0x7e {
			c.escapeState = 0
		}
		return true
	default:
		c.escapeState = 0
		return true
	}
}

func (c *webSSHCommandCollector) observeOutput(data string) {
	if c == nil || !webSSHOutputHasSensitivePrompt(data) {
		return
	}
	c.mu.Lock()
	c.suppressNextLine = true
	c.mu.Unlock()
}

func (c *webSSHCommandCollector) flushLocked(commands *[]string) {
	command := strings.TrimSpace(string(c.buffer))
	c.buffer = c.buffer[:0]
	if c.suppressNextLine {
		c.suppressNextLine = false
		return
	}
	if command == "" {
		return
	}
	*commands = append(*commands, webSSHSanitizeAuditCommand(command))
}

func (web *web) recordWebSSHAudit(target *controlplane.WebSSHTarget, userID uint, clientIP, clientIPSource, action, username, statement string, success bool, elapsedMS int64, errText string) {
	if target == nil || userID == 0 {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "execute"
	}
	username = strings.TrimSpace(username)
	statement = strings.TrimSpace(statement)
	if statement == "" {
		statement = webSSHAuditStatement(action, username)
	}
	if err := web.controlPlane.RecordWebDataAudit(context.Background(), &controlplane.WebDataAudit{
		UserID:           userID,
		ProxyID:          target.ProxyID,
		ProxyName:        target.ProxyName,
		ApplicationID:    target.ApplicationID,
		ApplicationName:  target.ApplicationName,
		Protocol:         "ssh",
		Action:           action,
		Database:         username,
		StatementPreview: webDataStatementPreview(statement),
		StatementSHA256:  webDataStatementHash(statement),
		Success:          success,
		AffectedRows:     0,
		Error:            errText,
		ElapsedMS:        elapsedMS,
		ClientIP:         strings.TrimSpace(clientIP),
		Details:          map[string]any{"client_ip_source": strings.TrimSpace(clientIPSource)},
	}); err != nil {
		log.Warnf("webssh audit record failed: proxy_id=%d user_id=%d action=%s err=%v", target.ProxyID, userID, action, err)
	}
}

func webSSHAuditStatement(action, username string) string {
	var statement string
	switch strings.TrimSpace(action) {
	case "open_session":
		statement = "OPEN SSH SESSION"
	case "close_session":
		statement = "CLOSE SSH SESSION"
	default:
		statement = strings.ToUpper(strings.TrimSpace(action))
	}
	username = strings.TrimSpace(username)
	if username != "" {
		return statement + " AS " + username
	}
	return statement
}

func webSSHOutputHasSensitivePrompt(data string) bool {
	normalized := strings.ToLower(strings.TrimSpace(data))
	if normalized == "" {
		return false
	}
	markers := []string{
		"password:",
		"password for",
		"passphrase:",
		"verification code:",
		"one-time password:",
		"请输入密码",
		"密码:",
		"密码：",
	}
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func webSSHSanitizeAuditCommand(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	lower := strings.ToLower(command)
	markers := []string{
		"password",
		"passwd",
		"passphrase",
		"secret",
		"token",
		"api_key",
		"apikey",
		"access_key",
		"secret_key",
		"sshpass",
		"mysql_pwd",
		"pgpassword",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return "[SENSITIVE COMMAND REDACTED]"
		}
	}
	return command
}

func refreshWebSSHReadDeadline(conn *websocket.Conn) {
	_ = conn.SetReadDeadline(time.Now().Add(webSSHHeartbeatTTL))
}

func (web *web) copyWebSSHOutput(writer *webSSHWSWriter, reader io.Reader, done <-chan struct{}, observe func(string)) {
	chunks := make(chan []byte, 16)
	quit := make(chan struct{})
	go func() {
		defer close(chunks)
		buf := make([]byte, webSSHOutputReadSize)
		for {
			select {
			case <-done:
				return
			case <-quit:
				return
			default:
			}
			n, err := reader.Read(buf)
			if n > 0 {
				chunk := append([]byte(nil), buf[:n]...)
				select {
				case chunks <- chunk:
				case <-done:
					return
				case <-quit:
					return
				}
			}
			if err != nil {
				return
			}
		}
	}()

	pending := make([]byte, 0, webSSHOutputReadSize)
	var flushTimer *time.Timer
	var flushTimerC <-chan time.Time
	stopFlushTimer := func() {
		if flushTimer == nil {
			return
		}
		if !flushTimer.Stop() {
			select {
			case <-flushTimer.C:
			default:
			}
		}
		flushTimerC = nil
	}
	startFlushTimer := func() {
		if flushTimer == nil {
			flushTimer = time.NewTimer(webSSHOutputFlush)
		} else {
			flushTimer.Reset(webSSHOutputFlush)
		}
		flushTimerC = flushTimer.C
	}
	flush := func() bool {
		stopFlushTimer()
		if len(pending) == 0 {
			return true
		}
		data := string(pending)
		pending = pending[:0]
		if observe != nil {
			observe(data)
		}
		return writer.write(webSSHServerMessage{Type: "output", Data: data}) == nil
	}

	defer func() {
		close(quit)
		stopFlushTimer()
	}()
	for {
		select {
		case <-done:
			flush()
			return
		case chunk, ok := <-chunks:
			if !ok {
				flush()
				return
			}
			pending = append(pending, chunk...)
			if len(pending) >= webSSHOutputBatchMax {
				if !flush() {
					return
				}
			} else if flushTimerC == nil {
				startFlushTimer()
			}
		case <-flushTimerC:
			if !flush() {
				return
			}
		}
	}
}

func (web *web) webSSHHostKeyCallback(proxyID uint) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		target, err := web.controlPlane.GetWebSSHTarget(context.Background(), proxyID)
		if err != nil {
			return err
		}
		if target.HostKey != nil && target.HostKey.Trusted {
			if target.HostKey.FingerprintSHA256 != fingerprint {
				return fmt.Errorf("SSH 主机指纹变化，已阻止连接。请确认目标机器身份后重置信任指纹")
			}
			return nil
		}
		publicKey := base64.StdEncoding.EncodeToString(key.Marshal())
		return web.controlPlane.TrustWebSSHHostKey(context.Background(), proxyID, key.Type(), fingerprint, publicKey)
	}
}

func (web *web) webSSHURL(r *http.Request, path string) string {
	scheme := "ws"
	if r.TLS != nil || strings.EqualFold(r.Header.Get("X-Forwarded-Proto"), "https") {
		scheme = "wss"
	}
	return (&url.URL{Scheme: scheme, Host: r.Host, Path: path}).String()
}

func (web *web) checkWebSSHOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return strings.EqualFold(u.Host, r.Host)
}

func parseWebSSHProxyID(r *http.Request, suffix string) (uint, error) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webssh/proxies/")
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

func parseWebSSHSessionToken(r *http.Request) string {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webssh/sessions/")
	return strings.TrimSuffix(path, "/connect")
}

func normalizeWebSSHSize(cols, rows int) (int, int) {
	if cols < 20 {
		cols = webSSHDefaultColumns
	}
	if rows < 5 {
		rows = webSSHDefaultRows
	}
	if cols > 300 {
		cols = 300
	}
	if rows > 120 {
		rows = 120
	}
	return cols, rows
}

func randomWebSSHToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func (web *web) encryptWebSSHPassword(password []byte) (string, string, error) {
	if len(web.credentialKey) == 0 {
		return "", "", errors.New("WebSSH 凭据加密密钥未配置")
	}
	block, err := aes.NewCipher(web.credentialKey)
	if err != nil {
		return "", "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", "", err
	}
	ciphertext := gcm.Seal(nil, nonce, password, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), base64.StdEncoding.EncodeToString(nonce), nil
}

func (web *web) decryptWebSSHPassword(encryptedPassword, nonce string) ([]byte, error) {
	if len(web.credentialKey) == 0 {
		return nil, errors.New("WebSSH 凭据加密密钥未配置")
	}
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedPassword)
	if err != nil {
		return nil, err
	}
	nonceBytes, err := base64.StdEncoding.DecodeString(nonce)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(web.credentialKey)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Open(nil, nonceBytes, ciphertext, nil)
}

func webSSHHTTPStatus(err error) int {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound
	}
	if errors.Is(err, controlplane.ErrForbidden()) {
		return http.StatusForbidden
	}
	return http.StatusBadRequest
}

func webSSHConnectionError(err error) string {
	msg := err.Error()
	lower := strings.ToLower(msg)
	if strings.Contains(lower, "unable to authenticate") || strings.Contains(lower, "permission denied") || strings.Contains(lower, "no supported methods remain") {
		return "SSH 认证失败，请检查用户名或密码"
	}
	if strings.Contains(msg, "SSH 主机指纹变化") {
		return msg
	}
	return "SSH 连接失败: " + msg
}
