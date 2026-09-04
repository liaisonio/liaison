package sshgateway

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/entry/firewall"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/proto"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

const (
	upstreamConnectTimeout = 15 * time.Second
	auditTimeout           = 2 * time.Second
	maxAuditCommandRunes   = 4096
)

var errAgentForwardingRequired = errors.New("SSH agent forwarding is required for public-key authentication")

type TargetProvider interface {
	OpenWebSSHStream(ctx context.Context, proxyID uint) (net.Conn, *controlplane.WebSSHTarget, error)
	TrustWebSSHHostKey(ctx context.Context, proxyID uint, algorithm, fingerprintSHA256, publicKey string) error
	RecordWebDataAudit(ctx context.Context, audit *controlplane.WebDataAudit) error
}

type Gateway struct {
	mu          sync.Mutex
	proxies     map[int]net.Listener
	ports       map[int]int
	targets     TargetProvider
	hostKey     ssh.Signer
	firewall    *firewall.Manager
	idleTimeout time.Duration
	maxDuration time.Duration
}

type connectionAuth struct {
	mu          sync.Mutex
	method      string
	publicKey   ssh.PublicKey
	fingerprint string
	upstream    *ssh.Client
	target      *controlplane.WebSSHTarget
	opened      bool
	started     time.Time
	agentOnce   sync.Once
	agentErr    error
}

type shellCommandCollector struct {
	mu               sync.Mutex
	enabled          bool
	buffer           []rune
	escapeState      int
	suppressNextLine bool
	outputTail       string
}

func New(hostKeyFile string, targets TargetProvider, idleTimeout, maxDuration time.Duration) (*Gateway, error) {
	hostKey, err := loadOrCreateHostKey(filepath.Clean(hostKeyFile))
	if err != nil {
		return nil, err
	}
	return &Gateway{
		proxies:     make(map[int]net.Listener),
		ports:       make(map[int]int),
		targets:     targets,
		hostKey:     hostKey,
		idleTimeout: idleTimeout,
		maxDuration: maxDuration,
	}, nil
}

func (g *Gateway) SetFirewall(manager *firewall.Manager) { g.firewall = manager }

func (g *Gateway) CreateProxy(ctx context.Context, definition *proto.Proxy) error {
	if definition == nil || definition.ID <= 0 {
		return errors.New("invalid SSH access definition")
	}
	listener, err := net.Listen("tcp", net.JoinHostPort("", fmt.Sprintf("%d", definition.ProxyPort)))
	if err != nil {
		return fmt.Errorf("listen for SSH access %d: %w", definition.ID, err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port

	g.mu.Lock()
	if old := g.proxies[definition.ID]; old != nil {
		_ = old.Close()
	}
	g.proxies[definition.ID] = listener
	g.ports[definition.ID] = actualPort
	g.mu.Unlock()
	definition.ProxyPort = actualPort

	// A proxy listener outlives the control-plane request that created it.
	go g.serve(context.Background(), listener, *definition)
	return nil
}

func (g *Gateway) DeleteProxy(_ context.Context, id int) error {
	g.mu.Lock()
	listener := g.proxies[id]
	delete(g.proxies, id)
	delete(g.ports, id)
	g.mu.Unlock()
	if listener != nil {
		return listener.Close()
	}
	return nil
}

func (g *Gateway) Close() error {
	g.mu.Lock()
	listeners := make([]net.Listener, 0, len(g.proxies))
	for _, listener := range g.proxies {
		listeners = append(listeners, listener)
	}
	g.proxies = make(map[int]net.Listener)
	g.ports = make(map[int]int)
	g.mu.Unlock()
	var first error
	for _, listener := range listeners {
		if err := listener.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (g *Gateway) serve(ctx context.Context, listener net.Listener, definition proto.Proxy) {
	for {
		raw, err := listener.Accept()
		if err != nil {
			return
		}
		if g.firewall != nil && !g.firewall.CheckAddr(definition.ID, raw.RemoteAddr()) {
			_ = raw.Close()
			continue
		}
		go g.handleConnection(ctx, raw, definition)
	}
}

func (g *Gateway) handleConnection(parent context.Context, raw net.Conn, definition proto.Proxy) {
	defer raw.Close()
	if g.idleTimeout > 0 {
		raw = &idleConn{Conn: raw, timeout: g.idleTimeout}
	}
	ctx := parent
	cancel := func() {}
	if g.maxDuration > 0 {
		ctx, cancel = context.WithTimeout(parent, g.maxDuration)
	} else {
		ctx, cancel = context.WithCancel(parent)
	}
	defer cancel()

	state := &connectionAuth{started: time.Now()}
	config := g.serverConfig(ctx, &definition, raw.RemoteAddr(), state)
	serverConn, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		state.close()
		return
	}
	defer serverConn.Close()
	go func() {
		<-ctx.Done()
		_ = serverConn.Close()
	}()
	defer func() {
		if state.wasOpened() {
			g.recordAudit(ctx, &definition, state.targetValue(), serverConn.User(), state.methodValue(), state.fingerprintValue(), "close_session", "", true, "", raw.RemoteAddr(), state.started)
		}
		state.close()
	}()

	go rejectGlobalRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(ssh.Prohibited, "port forwarding and auxiliary channels are disabled")
			continue
		}
		go g.bridgeSession(ctx, &definition, serverConn, state, channel, raw.RemoteAddr())
	}
}

func (g *Gateway) serverConfig(ctx context.Context, definition *proto.Proxy, remote net.Addr, state *connectionAuth) *ssh.ServerConfig {
	config := &ssh.ServerConfig{
		ServerVersion: "SSH-2.0-Liaison",
		PasswordCallback: func(meta ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
			upstream, target, err := g.openUpstream(ctx, definition, meta.User(), ssh.Password(string(password)))
			for index := range password {
				password[index] = 0
			}
			if err != nil {
				g.recordAudit(ctx, definition, target, meta.User(), "password", "", "open_session", "", false, "target SSH authentication failed", remote, time.Now())
				return nil, errors.New("target SSH authentication failed")
			}
			state.setAuthenticated("password", nil, "", upstream, target)
			g.markOpenAndAudit(ctx, definition, state, meta.User(), remote)
			return nil, nil
		},
		PublicKeyCallback: func(meta ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			fingerprint := ssh.FingerprintSHA256(key)
			state.setAuthenticated("agent", key, fingerprint, nil, nil)
			return &ssh.Permissions{Extensions: map[string]string{
				"permit-agent-forwarding": "true",
				"public-key-fingerprint":  fingerprint,
			}}, nil
		},
	}
	config.AddHostKey(g.hostKey)
	return config
}

func (g *Gateway) openUpstream(ctx context.Context, definition *proto.Proxy, username string, auth ssh.AuthMethod) (*ssh.Client, *controlplane.WebSSHTarget, error) {
	raw, target, err := g.targets.OpenWebSSHStream(ctx, uint(definition.ID))
	if err != nil {
		return nil, target, err
	}
	config := &ssh.ClientConfig{
		User:            username,
		Auth:            []ssh.AuthMethod{auth},
		HostKeyCallback: g.hostKeyCallback(ctx, uint(definition.ID), target),
		Timeout:         upstreamConnectTimeout,
		ClientVersion:   "SSH-2.0-Liaison-Gateway",
	}
	connection, channels, requests, err := ssh.NewClientConn(raw, net.JoinHostPort(target.TargetHost, fmt.Sprintf("%d", target.TargetPort)), config)
	if err != nil {
		_ = raw.Close()
		return nil, target, err
	}
	return ssh.NewClient(connection, channels, requests), target, nil
}

func (g *Gateway) hostKeyCallback(ctx context.Context, proxyID uint, target *controlplane.WebSSHTarget) ssh.HostKeyCallback {
	return func(_ string, _ net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)
		if target != nil && target.HostKey != nil && target.HostKey.Trusted {
			if target.HostKey.FingerprintSHA256 != fingerprint {
				return fmt.Errorf("target SSH host key changed: got %s", fingerprint)
			}
			return nil
		}
		return g.targets.TrustWebSSHHostKey(ctx, proxyID, key.Type(), fingerprint, strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))))
	}
}

func (g *Gateway) bridgeSession(ctx context.Context, definition *proto.Proxy, serverConn *ssh.ServerConn, state *connectionAuth, incoming ssh.NewChannel, remote net.Addr) {
	clientChannel, clientRequests, err := incoming.Accept()
	if err != nil {
		return
	}
	defer clientChannel.Close()

	if state.upstreamValue() == nil {
		if state.methodValue() != "agent" {
			writeSessionFailure(clientChannel, "SSH authentication state is unavailable")
			return
		}
		var ready bool
		for request := range clientRequests {
			if request.Type != "auth-agent-req@openssh.com" {
				writeSessionFailure(clientChannel, "public-key login requires agent forwarding; reconnect with ssh -A")
				if request.WantReply {
					_ = request.Reply(false, nil)
				}
				return
			}
			err = state.authenticateWithAgent(func() error {
				return g.openUpstreamWithAgent(ctx, definition, serverConn, state)
			})
			if request.WantReply {
				_ = request.Reply(err == nil, nil)
			}
			if err != nil {
				g.recordAudit(ctx, definition, state.targetValue(), serverConn.User(), "agent", state.fingerprintValue(), "open_session", "", false, err.Error(), remote, state.started)
				writeSessionFailure(clientChannel, "agent authentication to the target SSH server failed")
				return
			}
			g.markOpenAndAudit(ctx, definition, state, serverConn.User(), remote)
			ready = true
			break
		}
		if !ready {
			return
		}
	}

	upstream := state.upstreamValue()
	if upstream == nil {
		writeSessionFailure(clientChannel, "target SSH connection is unavailable")
		return
	}
	targetChannel, targetRequests, err := upstream.OpenChannel("session", nil)
	if err != nil {
		writeSessionFailure(clientChannel, "unable to open target SSH session")
		return
	}
	defer targetChannel.Close()

	sessionCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	targetRequestsDone := make(chan struct{})
	go func() {
		defer close(targetRequestsDone)
		forwardTargetRequests(sessionCtx, targetRequests, clientChannel)
	}()
	clientRequestsDone := make(chan struct{})
	commandCollector := &shellCommandCollector{}
	go func() {
		defer close(clientRequestsDone)
		g.forwardSessionRequests(sessionCtx, definition, state, clientRequests, targetChannel, serverConn.User(), remote, commandCollector)
	}()
	clientInputDone := make(chan struct{}, 1)
	targetOutputDone := make(chan struct{}, 1)
	go copyAndCloseWrite(targetChannel, clientChannel, func(data []byte) {
		commandCollector.feed(data, func(command string) {
			g.recordAudit(sessionCtx, definition, state.targetValue(), serverConn.User(), state.methodValue(), state.fingerprintValue(), "execute", command, true, "", remote, time.Now())
		})
	}, clientInputDone)
	go copyAndCloseWrite(clientChannel, targetChannel, commandCollector.observeOutput, targetOutputDone)

	// Client stdin commonly reaches EOF immediately for one-shot exec calls.
	// Closing the whole target channel at that point races with stdout and the
	// exit-status request. The target output side is authoritative for normal
	// completion; a closed client request stream or context still aborts it.
	select {
	case <-targetOutputDone:
	case <-clientRequestsDone:
		_ = targetChannel.Close()
		<-targetOutputDone
	case <-ctx.Done():
		_ = targetChannel.Close()
		<-targetOutputDone
	}
	_ = targetChannel.Close()
	cancel()
	<-targetRequestsDone
	select {
	case <-clientRequestsDone:
	default:
	}
}

func (g *Gateway) openUpstreamWithAgent(ctx context.Context, definition *proto.Proxy, serverConn *ssh.ServerConn, state *connectionAuth) error {
	channel, requests, err := serverConn.OpenChannel("auth-agent@openssh.com", nil)
	if err != nil {
		return errAgentForwardingRequired
	}
	defer channel.Close()
	go ssh.DiscardRequests(requests)
	signers, err := agent.NewClient(channel).Signers()
	if err != nil {
		return fmt.Errorf("read forwarded SSH agent: %w", err)
	}
	fingerprint := state.fingerprintValue()
	var matching ssh.Signer
	for _, signer := range signers {
		if ssh.FingerprintSHA256(signer.PublicKey()) == fingerprint {
			matching = signer
			break
		}
	}
	if matching == nil {
		return fmt.Errorf("forwarded SSH agent does not contain the login key %s", fingerprint)
	}
	upstream, target, err := g.openUpstream(ctx, definition, serverConn.User(), ssh.PublicKeys(matching))
	if err != nil {
		state.setTarget(target)
		return fmt.Errorf("target SSH public-key authentication failed: %w", err)
	}
	state.setUpstream(upstream, target)
	return nil
}

func (g *Gateway) forwardSessionRequests(ctx context.Context, definition *proto.Proxy, state *connectionAuth, requests <-chan *ssh.Request, target ssh.Channel, username string, remote net.Addr, collector *shellCommandCollector) {
	for {
		select {
		case <-ctx.Done():
			return
		case request, ok := <-requests:
			if !ok {
				return
			}
			if request.Type == "auth-agent-req@openssh.com" {
				if request.WantReply {
					_ = request.Reply(state.methodValue() == "agent", nil)
				}
				continue
			}
			allowed, command := allowedSessionRequest(request)
			if !allowed {
				if request.WantReply {
					_ = request.Reply(false, nil)
				}
				continue
			}
			success, err := target.SendRequest(request.Type, request.WantReply, request.Payload)
			if request.WantReply {
				_ = request.Reply(success && err == nil, nil)
			}
			if request.Type == "shell" && success && err == nil && collector != nil {
				collector.enable()
			}
			if request.Type == "exec" {
				errorText := ""
				if err != nil {
					errorText = err.Error()
				}
				g.recordAudit(ctx, definition, state.targetValue(), username, state.methodValue(), state.fingerprintValue(), "execute", command, success && err == nil, errorText, remote, time.Now())
			}
		}
	}
}

func allowedSessionRequest(request *ssh.Request) (bool, string) {
	switch request.Type {
	case "pty-req", "shell", "window-change", "signal":
		return true, ""
	case "env":
		var payload struct{ Name, Value string }
		if ssh.Unmarshal(request.Payload, &payload) != nil {
			return false, ""
		}
		return payload.Name == "TERM" || payload.Name == "LANG" || strings.HasPrefix(payload.Name, "LC_"), ""
	case "exec":
		var payload struct{ Command string }
		if ssh.Unmarshal(request.Payload, &payload) != nil || forbiddenExec(payload.Command) {
			return false, payload.Command
		}
		return true, payload.Command
	default:
		return false, ""
	}
}

func forbiddenExec(command string) bool {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return false
	}
	name := filepath.Base(fields[0])
	return name == "scp" || name == "sftp-server" || strings.Contains(command, "/sftp-server")
}

func forwardTargetRequests(ctx context.Context, requests <-chan *ssh.Request, client ssh.Channel) {
	for {
		select {
		case <-ctx.Done():
			return
		case request, ok := <-requests:
			if !ok {
				return
			}
			success, err := client.SendRequest(request.Type, request.WantReply, request.Payload)
			if request.WantReply {
				_ = request.Reply(success && err == nil, nil)
			}
		}
	}
}

func rejectGlobalRequests(requests <-chan *ssh.Request) {
	for request := range requests {
		if request.WantReply {
			_ = request.Reply(false, nil)
		}
	}
}

func copyAndCloseWrite(destination, source ssh.Channel, observe func([]byte), done chan<- struct{}) {
	buffer := make([]byte, 32*1024)
	for {
		count, readErr := source.Read(buffer)
		if count > 0 {
			data := buffer[:count]
			if observe != nil {
				observe(data)
			}
			if _, writeErr := destination.Write(data); writeErr != nil {
				break
			}
		}
		if readErr != nil {
			break
		}
	}
	_ = destination.CloseWrite()
	done <- struct{}{}
}

func (c *shellCommandCollector) enable() {
	if c == nil {
		return
	}
	c.mu.Lock()
	c.enabled = true
	c.mu.Unlock()
}

func (c *shellCommandCollector) feed(data []byte, emit func(string)) {
	if c == nil || emit == nil || len(data) == 0 {
		return
	}
	commands := make([]string, 0, 1)
	c.mu.Lock()
	if !c.enabled {
		c.mu.Unlock()
		return
	}
	for _, r := range string(data) {
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
			if r >= 0x20 && r != 0x7f && len(c.buffer) < maxAuditCommandRunes {
				c.buffer = append(c.buffer, r)
			}
		}
	}
	c.mu.Unlock()
	for _, command := range commands {
		emit(command)
	}
}

func (c *shellCommandCollector) consumeEscape(r rune) bool {
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

func (c *shellCommandCollector) observeOutput(data []byte) {
	if c == nil || len(data) == 0 {
		return
	}
	c.mu.Lock()
	combined := c.outputTail + string(data)
	if len(combined) > 256 {
		c.outputTail = combined[len(combined)-256:]
	} else {
		c.outputTail = combined
	}
	if outputHasSensitivePrompt(combined) {
		c.suppressNextLine = true
		c.outputTail = ""
	}
	c.mu.Unlock()
}

func (c *shellCommandCollector) flushLocked(commands *[]string) {
	command := strings.TrimSpace(string(c.buffer))
	c.buffer = c.buffer[:0]
	if c.suppressNextLine {
		c.suppressNextLine = false
		return
	}
	if command != "" {
		*commands = append(*commands, sanitizeAuditCommand(command))
	}
}

func outputHasSensitivePrompt(data string) bool {
	normalized := strings.ToLower(strings.TrimSpace(data))
	for _, marker := range []string{"password:", "password for", "passphrase:", "verification code:", "one-time password:", "请输入密码", "密码:", "密码："} {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func sanitizeAuditCommand(command string) string {
	command = strings.TrimSpace(command)
	lower := strings.ToLower(command)
	for _, marker := range []string{"password", "passwd", "passphrase", "secret", "token", "api_key", "apikey", "access_key", "secret_key", "sshpass", "mysql_pwd", "pgpassword"} {
		if strings.Contains(lower, marker) {
			return "[SENSITIVE COMMAND REDACTED]"
		}
	}
	return command
}

func writeSessionFailure(channel ssh.Channel, message string) {
	_, _ = channel.Stderr().Write([]byte("liaison: " + message + "\n"))
	_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{255}))
}

func (g *Gateway) markOpenAndAudit(ctx context.Context, definition *proto.Proxy, state *connectionAuth, username string, remote net.Addr) {
	if !state.markOpened() {
		return
	}
	g.recordAudit(ctx, definition, state.targetValue(), username, state.methodValue(), state.fingerprintValue(), "open_session", "", true, "", remote, state.started)
}

func (g *Gateway) recordAudit(ctx context.Context, definition *proto.Proxy, target *controlplane.WebSSHTarget, username, authMethod, fingerprint, action, command string, success bool, errorText string, remote net.Addr, started time.Time) {
	if g.targets == nil || definition == nil {
		return
	}
	details := map[string]any{"auth_method": authMethod}
	if fingerprint != "" {
		details["public_key_fingerprint"] = fingerprint
	}
	if command != "" {
		details["command"] = command
	}
	proxyName := definition.Name
	applicationID := definition.ApplicationID
	applicationName := ""
	if target != nil {
		proxyName = target.ProxyName
		applicationID = target.ApplicationID
		applicationName = target.ApplicationName
	}
	clientIP := ""
	if remote != nil {
		clientIP = remote.String()
		if host, _, err := net.SplitHostPort(clientIP); err == nil {
			clientIP = host
		}
	}
	hash := ""
	if command != "" {
		digest := sha256.Sum256([]byte(command))
		hash = hex.EncodeToString(digest[:])
	}
	auditCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), auditTimeout)
	defer cancel()
	_ = g.targets.RecordWebDataAudit(auditCtx, &controlplane.WebDataAudit{
		ProxyID:          uint(definition.ID),
		ProxyName:        proxyName,
		ApplicationID:    applicationID,
		ApplicationName:  applicationName,
		Protocol:         "ssh",
		Action:           action,
		Database:         username,
		StatementPreview: command,
		StatementSHA256:  hash,
		Success:          success,
		Error:            errorText,
		ElapsedMS:        time.Since(started).Milliseconds(),
		ClientIP:         clientIP,
		Details:          details,
	})
}

func (state *connectionAuth) setAuthenticated(method string, key ssh.PublicKey, fingerprint string, upstream *ssh.Client, target *controlplane.WebSSHTarget) {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.upstream != nil && state.upstream != upstream {
		_ = state.upstream.Close()
	}
	state.method = method
	state.publicKey = key
	state.fingerprint = fingerprint
	state.upstream = upstream
	state.target = target
}

func (state *connectionAuth) setUpstream(upstream *ssh.Client, target *controlplane.WebSSHTarget) {
	state.mu.Lock()
	state.upstream = upstream
	state.target = target
	state.mu.Unlock()
}

func (state *connectionAuth) setTarget(target *controlplane.WebSSHTarget) {
	state.mu.Lock()
	state.target = target
	state.mu.Unlock()
}

func (state *connectionAuth) authenticateWithAgent(authenticate func() error) error {
	state.agentOnce.Do(func() { state.agentErr = authenticate() })
	return state.agentErr
}

func (state *connectionAuth) markOpened() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	if state.opened {
		return false
	}
	state.opened = true
	return true
}

func (state *connectionAuth) wasOpened() bool {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.opened
}

func (state *connectionAuth) close() {
	state.mu.Lock()
	upstream := state.upstream
	state.upstream = nil
	state.mu.Unlock()
	if upstream != nil {
		_ = upstream.Close()
	}
}

func (state *connectionAuth) upstreamValue() *ssh.Client {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.upstream
}

func (state *connectionAuth) targetValue() *controlplane.WebSSHTarget {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.target
}

func (state *connectionAuth) methodValue() string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.method
}

func (state *connectionAuth) fingerprintValue() string {
	state.mu.Lock()
	defer state.mu.Unlock()
	return state.fingerprint
}

type idleConn struct {
	net.Conn
	timeout time.Duration
}

func (connection *idleConn) Read(data []byte) (int, error) {
	_ = connection.Conn.SetReadDeadline(time.Now().Add(connection.timeout))
	return connection.Conn.Read(data)
}

func (connection *idleConn) Write(data []byte) (int, error) {
	_ = connection.Conn.SetWriteDeadline(time.Now().Add(connection.timeout))
	return connection.Conn.Write(data)
}
