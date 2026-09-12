package web

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"golang.org/x/crypto/ssh"
)

const agentTerminalOutputLimit = 1024 * 1024

type webDataAgentHandle struct {
	web     *web
	session *webDataSession
}

type webDesktopAgentHandle struct {
	session   *webDesktopSession
	startedAt time.Time
}

func (handle *webDesktopAgentHandle) SessionInfo(ctx context.Context) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if handle == nil || handle.session == nil || handle.session.target == nil {
		return nil, errors.New("desktop session is unavailable")
	}
	target := handle.session.target
	return json.Marshal(map[string]any{
		"connected":        true,
		"protocol":         handle.session.protocol,
		"application_name": target.ApplicationName,
		"access_name":      target.ProxyName,
		"target":           fmt.Sprintf("%s:%d", target.TargetHost, target.TargetPort),
		"remote_user":      strings.TrimSpace(handle.session.username),
		"domain":           strings.TrimSpace(handle.session.domain),
		"width":            handle.session.width,
		"height":           handle.session.height,
		"dpi":              handle.session.dpi,
		"connected_at":     handle.startedAt.UTC().Format(time.RFC3339),
	})
}

func (web *web) registerWebDesktopAgentSession(session *webDesktopSession) (func(), error) {
	if session == nil || session.target == nil {
		return nil, errors.New("webdesktop session target is required")
	}
	descriptor, unregister, err := web.accessSessions.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{
			ID: session.token, UserID: session.userID, AccessID: session.proxyID,
			ApplicationID: session.target.ApplicationID, Protocol: accesssession.Protocol(session.protocol),
			Capabilities: []string{"desktop.session_info"},
		},
		Desktop: &webDesktopAgentHandle{session: session, startedAt: time.Now()},
	})
	if err != nil {
		return nil, fmt.Errorf("register webdesktop agent session: %w", err)
	}
	log.Debugf("webdesktop agent session registered: proxy_id=%d protocol=%s generation=%d", session.proxyID, session.protocol, descriptor.Generation)
	return unregister, nil
}

func (handle *webDataAgentHandle) Schema(ctx context.Context, path []string) (json.RawMessage, error) {
	if err := handle.web.ensureWebDataSessionActive(ctx, handle.session); err != nil {
		return nil, err
	}
	handle.session.mu.Lock()
	defer handle.session.mu.Unlock()

	query := webDataMetadataRequest{}
	if len(path) > 0 {
		query.NodeType = strings.TrimSpace(path[0])
	}
	if len(path) > 1 {
		query.Database = strings.TrimSpace(path[1])
	}
	if len(path) > 2 {
		query.Schema = strings.TrimSpace(path[2])
	}
	if len(path) > 3 {
		query.Name = strings.TrimSpace(path[3])
	}
	// Object inspection reuses the console's bounded, read-only metadata path.
	if query.NodeType == "table" || query.NodeType == "collection" || query.NodeType == "key" || query.NodeType == "index" {
		detail, err := handle.session.objectDetails(ctx, webDataObjectRequest{
			ObjectType: query.NodeType, Database: query.Database, Schema: query.Schema, Name: query.Name, Key: query.Name,
		})
		if err != nil {
			return nil, err
		}
		return json.Marshal(detail)
	}
	var (
		nodes []webDataMetadataNode
		err   error
	)
	if query.NodeType == "" {
		nodes, err = handle.session.metadata(ctx)
	} else {
		nodes, err = handle.session.metadataChildren(ctx, query)
	}
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{"nodes": nodes, "protocol": handle.session.protocol, "current_database": webDataAuditSessionDatabase(handle.session)})
}

func (handle *webDataAgentHandle) Query(ctx context.Context, statement string) (json.RawMessage, error) {
	if err := handle.web.ensureWebDataSessionActive(ctx, handle.session); err != nil {
		return nil, err
	}
	statement = strings.TrimSpace(statement)
	if statement == "" {
		return nil, errors.New("data statement is required")
	}
	if len(statement) > webDataStatementMaxSize {
		return nil, errors.New("data statement is too large")
	}

	handle.session.mu.Lock()
	defer handle.session.mu.Unlock()
	started := time.Now()
	execCtx, cancel := context.WithTimeout(ctx, webDataExecuteTimeout)
	defer cancel()
	result, execErr := handle.session.execute(execCtx, statement)
	elapsed := time.Since(started).Milliseconds()
	if result == nil {
		result = &webDataExecuteResponse{Type: "message"}
	}
	result.ElapsedMS = elapsed
	result.AffectedRows = normalizeAffectedRows(result.AffectedRows)
	if execErr != nil {
		result.Error = execErr.Error()
	}
	handle.auditQuery(ctx, statement, result, execErr)
	// Database errors are tool results the model can explain, not runtime failures.
	// Cancellation still aborts the turn; a timeout must not be silently retried.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	return json.Marshal(result)
}

func (handle *webDataAgentHandle) auditQuery(ctx context.Context, statement string, result *webDataExecuteResponse, execErr error) {
	if !webDataShouldAuditExecute(handle.session.protocol, statement) || handle.session.target == nil {
		return
	}
	if err := handle.web.controlPlane.RecordWebDataAudit(context.WithoutCancel(ctx), &controlplane.WebDataAudit{
		UserID:           handle.session.userID,
		ProxyID:          handle.session.proxyID,
		ApplicationID:    handle.session.target.ApplicationID,
		Protocol:         handle.session.protocol,
		Action:           "agent_execute",
		Database:         webDataAuditSessionDatabase(handle.session),
		StatementPreview: webDataStatementPreview(statement),
		StatementSHA256:  webDataStatementHash(statement),
		Success:          execErr == nil,
		AffectedRows:     result.AffectedRows,
		Error:            result.Error,
		ElapsedMS:        result.ElapsedMS,
		Details:          map[string]interface{}{"source": "agent"},
	}); err != nil {
		log.Warnf("agent webdata audit record failed: proxy_id=%d user_id=%d err=%v", handle.session.proxyID, handle.session.userID, err)
	}
}

func (web *web) registerWebDataAgentSession(session *webDataSession) error {
	if session == nil || session.target == nil {
		return errors.New("webdata session target is required")
	}
	descriptor, unregister, err := web.accessSessions.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{
			ID:            session.token,
			UserID:        session.userID,
			AccessID:      session.proxyID,
			ApplicationID: session.target.ApplicationID,
			Protocol:      accesssession.Protocol(session.protocol),
			Capabilities:  []string{"data.schema", "data.query"},
		},
		Data: &webDataAgentHandle{web: web, session: session},
	})
	if err != nil {
		return fmt.Errorf("register webdata agent session: %w", err)
	}
	session.agentGeneration = descriptor.Generation
	session.agentUnregister = unregister
	return nil
}

type webSSHAgentHandle struct {
	client    *ssh.Client
	onExecute func(command string, success bool, elapsedMS int64, errText string)

	mu        sync.Mutex
	recent    []byte
	truncated bool
	shell     shellContext
}

func (handle *webSSHAgentHandle) observe(output string) {
	if output == "" {
		return
	}
	handle.mu.Lock()
	defer handle.mu.Unlock()
	handle.shell.observe(output)
	handle.recent = append(handle.recent, output...)
	if len(handle.recent) > agentTerminalOutputLimit {
		overflow := len(handle.recent) - agentTerminalOutputLimit
		copy(handle.recent, handle.recent[overflow:])
		handle.recent = handle.recent[:agentTerminalOutputLimit]
		handle.truncated = true
	}
}

func (handle *webSSHAgentHandle) Read(ctx context.Context, maxLines int) (json.RawMessage, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	handle.mu.Lock()
	output := append([]byte(nil), handle.recent...)
	truncated := handle.truncated
	shell := handle.shell.snapshot(true)
	handle.mu.Unlock()
	if maxLines <= 0 {
		maxLines = 100
	}
	lines := bytes.Split(output, []byte("\n"))
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
		truncated = true
	}
	result := map[string]interface{}{"output": string(bytes.Join(lines, []byte("\n"))), "truncated": truncated}
	if string(shell) != `{"quality":"unavailable"}` {
		result["shell_context"] = shell
	}
	return json.Marshal(result)
}

func (handle *webSSHAgentHandle) Execute(ctx context.Context, command string) (json.RawMessage, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil, errors.New("terminal command is required")
	}
	started := time.Now()
	record := func(err error) {
		if handle.onExecute == nil {
			return
		}
		errText := ""
		if err != nil {
			errText = err.Error()
		}
		handle.onExecute(command, err == nil, time.Since(started).Milliseconds(), errText)
	}
	session, err := handle.client.NewSession()
	if err != nil {
		err = fmt.Errorf("create SSH command session: %w", err)
		record(err)
		return nil, err
	}
	defer session.Close()
	output := &boundedOutput{limit: agentTerminalOutputLimit}
	session.Stdout = output
	session.Stderr = output
	if err := session.Start(command); err != nil {
		err = fmt.Errorf("start SSH command: %w", err)
		record(err)
		return nil, err
	}
	wait := make(chan error, 1)
	go func() {
		defer func() {
			if recover() != nil {
				wait <- errors.New("SSH command wait panicked")
			}
		}()
		wait <- session.Wait()
	}()
	select {
	case <-ctx.Done():
		// The context cancellation is the caller-visible cause. Closing the
		// channel only interrupts ssh.Session.Wait and cannot be recovered.
		_ = session.Close()
		err = ctx.Err()
		record(err)
		return nil, err
	case err := <-wait:
		exitCode := 0
		if err != nil {
			var exitError *ssh.ExitError
			if !errors.As(err, &exitError) {
				err = fmt.Errorf("wait for SSH command: %w", err)
				record(err)
				return nil, err
			}
			exitCode = exitError.ExitStatus()
		}
		content, marshalErr := json.Marshal(map[string]interface{}{
			"output": output.String(), "exit_code": exitCode, "truncated": output.Truncated(),
		})
		if marshalErr != nil {
			record(marshalErr)
			return nil, marshalErr
		}
		record(nil)
		return content, nil
	}
}

func (web *web) registerWebSSHAgentSession(client *ssh.Client, session *webSSHSession, target *controlplane.WebSSHTarget, clientIP, clientIPSource string) (*webSSHAgentHandle, func(), error) {
	if client == nil || session == nil || target == nil {
		return nil, nil, errors.New("webssh agent session is incomplete")
	}
	handle := &webSSHAgentHandle{client: client}
	handle.onExecute = func(command string, success bool, elapsedMS int64, errText string) {
		web.recordWebSSHAudit(target, session.userID, clientIP, clientIPSource, "agent_execute", session.username, command, success, elapsedMS, errText)
	}
	_, unregister, err := web.accessSessions.Register(accesssession.Handle{
		Descriptor: accesssession.Descriptor{
			ID:            session.token,
			UserID:        session.userID,
			AccessID:      session.proxyID,
			ApplicationID: target.ApplicationID,
			Protocol:      accesssession.ProtocolWebSSH,
			Capabilities:  []string{"terminal.read", "terminal.execute"},
		},
		Terminal: handle,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("register webssh agent session: %w", err)
	}
	return handle, unregister, nil
}

type boundedOutput struct {
	mu        sync.Mutex
	buffer    bytes.Buffer
	limit     int
	truncated bool
}

func (output *boundedOutput) Write(value []byte) (int, error) {
	output.mu.Lock()
	defer output.mu.Unlock()
	written := len(value)
	remaining := output.limit - output.buffer.Len()
	if remaining <= 0 {
		output.truncated = true
		return written, nil
	}
	if len(value) > remaining {
		value = value[:remaining]
		output.truncated = true
	}
	_, err := output.buffer.Write(value)
	return written, err
}

func (output *boundedOutput) String() string {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.buffer.String()
}

func (output *boundedOutput) Truncated() bool {
	output.mu.Lock()
	defer output.mu.Unlock()
	return output.truncated
}
