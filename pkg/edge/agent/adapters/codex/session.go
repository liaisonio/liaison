package codex

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	"github.com/liaisonio/liaison/pkg/edge/agent/process"
	"github.com/liaisonio/liaison/pkg/edge/agent/rpc"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
)

// Session is an explicitly owned app-server, not an attachment to an existing
// desktop process. Provider/auth and persisted model configuration are never overridden.
type Session struct {
	client         *rpc.Client
	cancel         context.CancelFunc
	done           chan struct{}
	mu             sync.Mutex
	waitErr        error
	threads        map[string]string
	directory      string
	skills         map[string]localSkill
	permissionMode string
}

func (Adapter) Launch(ctx context.Context, installation discovery.Installation, directory string) (agentruntime.Session, error) {
	return Start(ctx, installation, directory)
}

func Start(ctx context.Context, installation discovery.Installation, directory string) (*Session, error) {
	return start(ctx, installation, directory, false)
}

func start(ctx context.Context, installation discovery.Installation, directory string, experimental bool) (*Session, error) {
	if installation.Agent != "codex" || installation.Wrapper {
		return nil, errors.New("unsupported Codex installation entry point")
	}
	life, cancel := context.WithCancel(ctx)
	spec, err := launchSpec(installation, directory)
	if err != nil {
		cancel()
		return nil, err
	}
	child, err := process.Start(life, spec)
	if err != nil {
		cancel()
		return nil, err
	}
	s := &Session{client: rpc.New(child.Input, child.Output), cancel: cancel, done: make(chan struct{}), threads: make(map[string]string), directory: directory}
	go func() {
		defer close(s.done)
		defer func() {
			if recover() != nil {
				cancel()
				s.mu.Lock()
				s.waitErr = errors.New("agent process supervisor failed")
				s.mu.Unlock()
			}
		}()
		err := child.Wait()
		s.mu.Lock()
		s.waitErr = err
		s.mu.Unlock()
	}()
	// EOF/overflow/protocol failure must not leave an unattended Agent running.
	go func() {
		defer func() {
			if recover() != nil {
				cancel()
			}
		}()
		select {
		case <-s.client.Done():
			cancel()
		case <-s.done:
			cancel()
		}
	}()
	handshake, stop := context.WithTimeout(life, 10*time.Second)
	defer stop()
	if _, err = s.client.Call(handshake, "initialize", map[string]any{"clientInfo": map[string]string{"name": "liaison", "title": "Liaison", "version": "0.1.0"}, "capabilities": map[string]bool{"experimentalApi": experimental}}); err == nil {
		err = s.client.Notify(handshake, "initialized", map[string]any{})
	}
	if err != nil {
		closeErr := s.Close()
		return nil, errors.Join(errors.New("Codex handshake failed"), closeErr)
	}
	return s, nil
}

func (s *Session) Events() <-chan rpc.Message { return s.client.Events() }
func (s *Session) Done() <-chan struct{}      { return s.done }
func (s *Session) Close() error {
	s.cancel()
	if err := s.client.Close(); err != nil {
		return err
	}
	select {
	case <-s.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("agent shutdown timed out")
	}
}

type Thread = agentruntime.Thread

var ErrExternalTools = errors.New("external tools are not supported in read-only preview")

func (s *Session) NewThread(ctx context.Context) (Thread, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.threads) >= 8 {
		return Thread{}, errors.New("agent thread limit reached")
	}
	// Preview is read-only. Writes need a separately authorized launch/session
	// policy; model settings continue to come from Codex itself.
	// Read only the names of configured integrations into the policy projection.
	// This response stays inside the Edge: never log or return local configuration.
	configuration, err := s.client.Call(ctx, "config/read", map[string]any{"cwd": s.directory, "includeLayers": false})
	if err != nil {
		return Thread{}, ErrExternalTools
	}
	overrides, err := previewOverrides(configuration)
	if err != nil {
		return Thread{}, err
	}
	data, err := s.client.Call(ctx, "thread/start", map[string]any{"sandbox": "read-only", "approvalPolicy": "on-request", "ephemeral": false, "serviceName": "liaison", "config": overrides})
	if err != nil {
		return Thread{}, err
	}
	return s.acceptThread(ctx, data)
}

// ResumeThread reopens only an ID selected by the Edge's owner-scoped binding
// store. The adapter itself never lists or discovers unrelated native threads.
func (s *Session) ResumeThread(ctx context.Context, threadID string) (Thread, error) {
	if threadID == "" {
		return Thread{}, errors.New("invalid Codex thread ID")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.threads) >= 8 {
		return Thread{}, errors.New("agent thread limit reached")
	}
	configuration, err := s.client.Call(ctx, "config/read", map[string]any{"cwd": s.directory, "includeLayers": false})
	if err != nil {
		return Thread{}, ErrExternalTools
	}
	overrides, err := previewOverrides(configuration)
	if err != nil {
		return Thread{}, err
	}
	data, err := s.client.Call(ctx, "thread/resume", map[string]any{"threadId": threadID, "cwd": s.directory, "sandbox": "read-only", "approvalPolicy": "on-request", "config": overrides})
	if err != nil {
		return Thread{}, err
	}
	return s.acceptThread(ctx, data)
}

// acceptThread is called with s.mu held.
func (s *Session) acceptThread(ctx context.Context, data []byte) (Thread, error) {
	var response struct {
		Thread struct {
			Version string `json:"cliVersion"`
			ID      string `json:"id"`
			Model   string `json:"model"`
			Name    string `json:"name"`
		} `json:"thread"`
		Model string `json:"model"`
	}
	if json.Unmarshal(data, &response) != nil || response.Thread.ID == "" {
		return Thread{}, errors.New("invalid Codex thread response")
	}
	// Filesystem sandboxing does not constrain external MCP side effects.
	// Fail closed before any inference when this thread has external servers.
	// Do not expose their inventory or mutate the user's Codex configuration.
	inventory, err := s.client.Call(ctx, "mcpServerStatus/list", map[string]any{"threadId": response.Thread.ID, "limit": 256})
	if err != nil {
		return Thread{}, errors.New("cannot verify preview tool boundary")
	}
	if !emptyExternalToolInventory(inventory) {
		return Thread{}, ErrExternalTools
	}
	s.threads[response.Thread.ID] = ""
	model := response.Model
	if model == "" {
		model = response.Thread.Model
	}
	return Thread{Version: response.Thread.Version, ID: response.Thread.ID, Model: model, Name: response.Thread.Name}, nil
}

func previewOverrides(data []byte) (map[string]any, error) {
	var response struct {
		Config *struct {
			MCP     map[string]json.RawMessage `json:"mcp_servers"`
			Plugins map[string]json.RawMessage `json:"plugins"`
		} `json:"config"`
	}
	if json.Unmarshal(data, &response) != nil || response.Config == nil {
		return nil, ErrExternalTools
	}
	result := map[string]any{"features.apps": false}
	for table, entries := range map[string]map[string]json.RawMessage{"mcp_servers": response.Config.MCP, "plugins": response.Config.Plugins} {
		if len(entries) > 256 {
			return nil, ErrExternalTools
		}
		if len(entries) == 0 {
			continue
		}
		disabled := make(map[string]any, len(entries))
		for name := range entries {
			disabled[name] = map[string]any{"enabled": false}
		}
		result[table] = disabled
	}
	return result, nil
}

func emptyExternalToolInventory(data []byte) bool {
	var status struct {
		Data []struct {
			RuntimeStatus string                     `json:"runtimeStatus"`
			Tools         map[string]json.RawMessage `json:"tools"`
		} `json:"data"`
		NextCursor *string `json:"nextCursor"`
	}
	if json.Unmarshal(data, &status) != nil || status.Data == nil || status.NextCursor != nil {
		return false
	}
	for _, server := range status.Data {
		if server.RuntimeStatus != "disabled" || len(server.Tools) != 0 {
			return false
		}
	}
	return true
}
func (s *Session) Send(ctx context.Context, threadID, text string) (string, error) {
	return s.send(ctx, threadID, text, "")
}
func (s *Session) SendSkill(ctx context.Context, threadID, text, skillID string) (string, error) {
	return s.send(ctx, threadID, text, skillID)
}
func (s *Session) send(ctx context.Context, threadID, text, skillID string, selectedModel ...string) (string, error) {
	s.mu.Lock()
	_, owned := s.threads[threadID]
	skill, known := s.skills[skillID]
	mode := s.permissionMode
	s.mu.Unlock()
	if !owned {
		return "", errors.New("agent thread unavailable")
	}
	if threadID == "" || len(text) == 0 || len(text) > 64<<10 {
		return "", errors.New("invalid agent message")
	}
	inputs := []map[string]string{{"type": "text", "text": text}}
	if skillID != "" {
		if !known {
			return "", errors.New("skill unavailable")
		}
		inputs = append(inputs, map[string]string{"type": "skill", "name": skill.Name, "path": skill.Path})
	}
	params := map[string]any{"threadId": threadID, "input": inputs}
	params["approvalPolicy"] = "on-request"
	params["sandboxPolicy"] = map[string]any{"type": "readOnly"}
	if mode == "workspace-write" {
		params["sandboxPolicy"] = map[string]any{"type": "workspaceWrite", "writableRoots": []string{s.directory}, "networkAccess": false, "excludeTmpdirEnvVar": true, "excludeSlashTmp": true}
	}
	if len(selectedModel) > 0 && selectedModel[0] != "" {
		params["model"] = selectedModel[0]
	}
	data, err := s.client.Call(ctx, "turn/start", params)
	if err != nil {
		return "", err
	}
	var response struct {
		Turn struct {
			ID string `json:"id"`
		} `json:"turn"`
	}
	if json.Unmarshal(data, &response) != nil || response.Turn.ID == "" {
		return "", errors.New("invalid Codex turn response")
	}
	s.mu.Lock()
	s.threads[threadID] = response.Turn.ID
	s.mu.Unlock()
	return response.Turn.ID, nil
}
func (s *Session) Interrupt(ctx context.Context, threadID, turnID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if turnID == "" || s.threads[threadID] != turnID {
		return errors.New("agent turn unavailable")
	}
	_, err := s.client.Call(ctx, "turn/interrupt", map[string]string{"threadId": threadID, "turnId": turnID})
	return err
}

func (s *Session) SetPermissionMode(mode string) error {
	if mode != "read-only" && mode != "workspace-write" {
		return errors.New("invalid permission mode")
	}
	s.mu.Lock()
	s.permissionMode = mode
	s.mu.Unlock()
	return nil
}

// Only one command is approved; no exec-policy amendment or session-wide grant.
func (s *Session) ApproveCommand(ctx context.Context, message rpc.Message) error {
	if len(message.ID) == 0 || message.Method != "item/commandExecution/requestApproval" {
		return errors.New("unsupported approval")
	}
	return s.client.Respond(ctx, message.ID, map[string]string{"decision": "accept"})
}

// Unsupported permission requests fail closed rather than broadening access.
func (s *Session) AnswerInput(ctx context.Context, message rpc.Message, answers map[string][]string) error {
	if len(message.ID) == 0 || message.Method != "item/tool/requestUserInput" {
		return errors.New("unsupported input request")
	}
	result := make(map[string]any, len(answers))
	for id, values := range answers {
		result[id] = map[string]any{"answers": values}
	}
	return s.client.Respond(ctx, message.ID, map[string]any{"answers": result})
}

func (s *Session) RejectRequest(ctx context.Context, message rpc.Message) error {
	if len(message.ID) == 0 {
		return errors.New("not an agent request")
	}
	switch message.Method {
	case "item/commandExecution/requestApproval", "item/fileChange/requestApproval":
		return s.client.Respond(ctx, message.ID, map[string]string{"decision": "decline"})
	case "item/permissions/requestApproval":
		return s.client.Respond(ctx, message.ID, map[string]any{"permissions": map[string]any{}})
	default:
		return errors.New("unsupported agent request; stop this session")
	}
}

// CheckAuthentication returns status only, never account identifiers or tokens.
func (s *Session) CheckAuthentication(ctx context.Context) (bool, error) {
	data, err := s.client.Call(ctx, "account/read", map[string]bool{"refreshToken": false})
	if err != nil {
		return false, err
	}
	var response struct {
		Account  json.RawMessage `json:"account"`
		Requires bool            `json:"requiresOpenaiAuth"`
	}
	if json.Unmarshal(data, &response) != nil {
		return false, errors.New("invalid Codex account response")
	}
	return !response.Requires || (len(response.Account) > 0 && string(response.Account) != "null"), nil
}
