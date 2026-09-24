// Package bridge exposes a bounded, owner-scoped Agent preview over the existing
// authenticated Manager tunnel. It does not accept arbitrary RPC or executable paths.
package bridge

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/edge/agent/adapters/codex"
	"github.com/liaisonio/liaison/pkg/edge/agent/discovery"
	agentruntime "github.com/liaisonio/liaison/pkg/edge/agent/runtime"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/singchia/geminio"
)

const maxOutput = 256 << 10

type discoverFunc func(context.Context) (discovery.Result, discovery.Environment, error)
type Bridge struct {
	ctx         context.Context
	cancel      context.CancelFunc
	registry    *agentruntime.Registry
	adapter     agentruntime.Adapter
	discover    discoverFunc
	mu          sync.Mutex
	sessions    map[string]*session
	bindings    map[string]nativeBinding
	bindingPath string
	resuming    map[string]bool
	starting    int
	slots       chan struct{}
	done        chan struct{}
}
type session struct {
	window                        uint64
	truncated                     bool
	version                       string
	mu                            sync.Mutex
	owner, id, thread, turn       string
	agent                         agentruntime.Session
	messages                      []proto.EdgeAgentMessage
	running, closed               bool
	status                        string
	created, touched, turnStarted time.Time
	updated, closedAt             time.Time
	runtimeStarted                time.Time
	bytes                         int
	access, model, project        string
	skills                        []proto.AgentSkill
	skillsAvailable               bool
	revision                      uint64
	changed                       chan struct{}
	activities                    []proto.AgentActivity
	title                         string
	renamed                       bool
	models                        []proto.AgentModel
	modelsAvailable               bool
	selectedModel                 string
	permissionMode                string
	approvals                     []pendingApproval
	inputs                        []pendingInput
}
type registrar interface {
	RegisterRPCHandler(string, func(context.Context, geminio.Request, geminio.Response)) error
}

func localDiscovery(ctx context.Context) (discovery.Result, discovery.Environment, error) {
	env, err := discovery.CurrentEnvironment()
	if err != nil {
		return discovery.Result{}, env, err
	}
	platform, err := discovery.NewNative(env)
	if err != nil {
		return discovery.Result{}, env, err
	}
	result, err := discovery.Find(ctx, platform, env, codex.Adapter{}, "")
	return result, env, err
}
func New(ctx context.Context, bindingPath string) (*Bridge, error) {
	return newBridge(ctx, codex.Adapter{}, localDiscovery, bindingPath)
}
func newBridge(ctx context.Context, adapter agentruntime.Adapter, discover discoverFunc, bindingPath string) (*Bridge, error) {
	if !filepath.IsAbs(bindingPath) {
		return nil, errors.New("agent binding path must be absolute")
	}
	bindings, err := loadBindings(bindingPath)
	if err != nil {
		return nil, err
	}
	life, cancel := context.WithCancel(ctx)
	r, err := agentruntime.New(life, 4)
	if err != nil {
		cancel()
		return nil, err
	}
	b := &Bridge{ctx: life, cancel: cancel, registry: r, adapter: adapter, discover: discover, sessions: make(map[string]*session), bindings: bindings, bindingPath: bindingPath, resuming: make(map[string]bool), slots: make(chan struct{}, 8), done: make(chan struct{})}
	go b.reap()
	return b, nil
}
func (b *Bridge) Close() error { b.cancel(); <-b.done; return b.registry.Close() }
func (b *Bridge) Register(fb registrar) error {
	return fb.RegisterRPCHandler(proto.RPCEdgeAgent, func(ctx context.Context, req geminio.Request, rsp geminio.Response) {
		if len(req.Data()) > 512<<10 {
			rsp.SetError(errors.New("invalid agent request"))
			return
		}
		var input proto.EdgeAgentRPCRequest
		d := json.NewDecoder(bytes.NewReader(req.Data()))
		d.DisallowUnknownFields()
		if d.Decode(&input) != nil || d.Decode(new(any)) != io.EOF {
			rsp.SetError(errors.New("invalid agent request"))
			return
		}
		result := b.Handle(ctx, input)
		data, err := json.Marshal(result)
		if err != nil {
			rsp.SetError(errors.New("agent response unavailable"))
			return
		}
		rsp.SetData(data)
	})
}
func result(status string) proto.EdgeAgentResult {
	return proto.EdgeAgentResult{Version: 1, Status: status}
}
func installationID(i discovery.Installation) string {
	sum := sha256.Sum256([]byte(i.Agent + "\x00" + i.Path + "\x00" + i.ResolvedPath))
	return hex.EncodeToString(sum[:16])
}

func (b *Bridge) Handle(ctx context.Context, input proto.EdgeAgentRPCRequest) proto.EdgeAgentResult {
	if input.ResetRevision != 0 && input.Request.Action != "send" {
		return result("invalid_request")
	}
	if input.Version != 1 || input.ActorID == "" || len(input.ActorID) > 32 || !input.Request.Valid() || (input.Resume != nil) != (input.Request.Action == "resume") {
		return result("invalid_request")
	}
	select {
	case <-b.ctx.Done():
		return result("unavailable")
	default:
	}
	select {
	case b.slots <- struct{}{}:
		defer func() { <-b.slots }()
	default:
		return result("busy")
	}
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	req := input.Request
	if req.Action == "sessions" {
		return b.listSessions(input.ActorID, req.AccessID)
	}
	if req.Action == "directories" {
		env, err := discovery.CurrentEnvironment()
		if err != nil {
			return result("unavailable")
		}
		if env.AccountID == "0" || env.OS == "windows" {
			return result("unsupported_user")
		}
		return browseDirectories(ctx, env.Home, input.ProjectRoot, req.Directory)
	}
	if req.Action == "discover" || req.Action == "start" || req.Action == "resume" {
		found, env, err := b.discover(ctx)
		if err != nil {
			return result("unavailable")
		}
		if env.OS == "windows" || env.AccountID == "0" || strings.HasSuffix(env.AccountID, "-18") {
			return result("unsupported_user")
		}
		if req.Action == "discover" {
			r := result("ok")
			r.DefaultProject = env.Home
			r.Truncated = found.Truncated
			for _, i := range found.Installations {
				if !i.Wrapper {
					r.Installations = append(r.Installations, proto.AgentInstallation{ID: installationID(i), Kind: i.Agent, Path: i.Path, Source: i.Source})
				}
			}
			return r
		}
		if req.Action == "resume" {
			return b.resume(ctx, input, found)
		}
		if !filepath.IsAbs(req.Project) || strings.ContainsRune(req.Project, 0) {
			return result("invalid_request")
		}
		project, err := filepath.EvalSymlinks(req.Project)
		if err != nil {
			return result("invalid_request")
		}
		accessProject := project
		if req.WorkingDirectory != "" {
			_, selected, ok := resolveDirectory(directoryRoots(env.Home, input.ProjectRoot), req.WorkingDirectory)
			if !ok {
				return result("invalid_request")
			}
			project = selected
		}
		for _, i := range found.Installations {
			if installationID(i) == req.InstallationID && !i.Wrapper {
				return b.start(ctx, input.ActorID, project, accessProject, i, req.AccessID)
			}
		}
		return result("not_found")
	}
	b.mu.Lock()
	s := b.sessions[req.SessionID]
	b.mu.Unlock()
	if s == nil || s.owner != input.ActorID || s.access != req.AccessID {
		if req.Action == "delete" && len(req.AccessID) == 32 {
			key := bindingKey(input.ActorID, req.AccessID, req.SessionID)
			b.mu.Lock()
			_, exists := b.bindings[key]
			if exists {
				delete(b.bindings, key)
				if err := saveBindings(b.bindingPath, b.bindings); err != nil {
					b.mu.Unlock()
					return result("unavailable")
				}
			}
			b.mu.Unlock()
			if exists {
				return result("ok")
			}
		}
		return result("not_found")
	}
	if req.Action == "discard" {
		return b.discardEmpty(s)
	}
	// Background history synchronization must not renew the idle lease.
	if req.Action == "snapshot" {
		return s.snapshot()
	}
	s.mu.Lock()
	s.touched = time.Now()
	closed := s.closed
	s.mu.Unlock()
	if req.Action == "stop" {
		s.stop("session_closed")
		snapshot := s.snapshot()
		if s.access == "" {
			b.mu.Lock()
			delete(b.sessions, s.id)
			b.mu.Unlock()
		}
		return snapshot
	}
	if req.Action == "delete" {
		if !closed {
			return result("busy")
		}
		b.mu.Lock()
		delete(b.sessions, s.id)
		delete(b.bindings, bindingKey(s.owner, s.access, s.id))
		if err := saveBindings(b.bindingPath, b.bindings); err != nil {
			b.mu.Unlock()
			return result("unavailable")
		}
		b.mu.Unlock()
		return result("ok")
	}
	if req.Action == "rename" {
		s.mu.Lock()
		s.title = strings.TrimSpace(req.Title)
		s.renamed = true
		s.changedLocked()
		s.mu.Unlock()
		return s.snapshot()
	}
	if req.Action == "models" || req.Action == "model" {
		return s.manageModel(ctx, req)
	}
	if req.Action == "approve" || req.Action == "permissions" {
		return s.managePermissions(ctx, req)
	}
	if req.Action == "answer" {
		return s.answerInput(ctx, req)
	}
	if closed || req.Action == "poll" {
		return s.snapshot()
	}
	if req.Action == "watch" {
		revision, err := strconv.ParseUint(req.Cursor, 10, 64)
		if err != nil {
			return result("invalid_request")
		}
		return s.watch(ctx, revision)
	}
	switch req.Action {
	case "send":
		var skillAgent agentruntime.SkillSession
		if req.SkillID != "" {
			var ok bool
			skillAgent, ok = s.agent.(agentruntime.SkillSession)
			found := false
			for _, skill := range s.skills {
				if skill.ID == req.SkillID {
					found = true
					break
				}
			}
			if !ok || !found {
				return result("invalid_request")
			}
		}
		if strings.TrimSpace(req.Text) == "" {
			return result("invalid_request")
		}
		s.mu.Lock()
		if s.running || s.closed {
			s.mu.Unlock()
			return result("busy")
		}
		if input.ResetRevision != 0 {
			if s.access == "" || s.revision != input.ResetRevision {
				s.mu.Unlock()
				return result("busy")
			}
			if s.title == "" {
				s.title = s.displayTitleLocked()
			}
			s.window++
			s.messages = nil
			s.activities = nil
			s.bytes = 0
			s.truncated = false
		}
		s.messages = append(s.messages, proto.EdgeAgentMessage{Role: "user", Text: req.Text}, proto.EdgeAgentMessage{Role: "assistant"})
		s.bytes += len(req.Text)
		s.boundDisplayLocked()
		s.running = true
		s.status = "ok"
		s.turn = ""
		s.turnStarted = time.Now()
		s.changedLocked()
		selectedModel := s.selectedModel
		s.mu.Unlock()
		var turn string
		var err error
		if capable, ok := s.agent.(agentruntime.ModelSession); ok && selectedModel != "" {
			turn, err = capable.SendModel(ctx, s.thread, req.Text, req.SkillID, selectedModel)
		} else if skillAgent != nil {
			turn, err = skillAgent.SendSkill(ctx, s.thread, req.Text, req.SkillID)
		} else {
			turn, err = s.agent.Send(ctx, s.thread, req.Text)
		}
		if err != nil {
			s.stop("turn_failed")
			return s.snapshot()
		}
		s.mu.Lock()
		if s.running {
			s.turn = turn
			s.changedLocked()
		}
		s.mu.Unlock()
	case "interrupt":
		s.mu.Lock()
		turn, running := s.turn, s.running
		s.mu.Unlock()
		if running && turn != "" {
			if err := s.agent.Interrupt(ctx, s.thread, turn); err != nil {
				s.stop("turn_failed")
			}
		}
	}
	return s.snapshot()
}

func (b *Bridge) resume(ctx context.Context, input proto.EdgeAgentRPCRequest, found discovery.Result) proto.EdgeAgentResult {
	req, seed := input.Request, input.Resume
	if seed == nil || seed.SessionID != req.SessionID || len(req.AccessID) != 32 || seed.Revision == 0 || len(seed.Messages) > 96 || len(seed.Activities) > 128 {
		return result("invalid_request")
	}
	bytes := 0
	for _, message := range seed.Messages {
		if message.Role != "user" && message.Role != "assistant" {
			return result("invalid_request")
		}
		bytes += len(message.Text)
	}
	if bytes > maxOutput {
		return result("invalid_request")
	}
	detailBytes := 0
	for _, activity := range seed.Activities {
		detailBytes += activitySize(activity)
	}
	if detailBytes > maxActivityDetails {
		return result("invalid_request")
	}
	key := bindingKey(input.ActorID, req.AccessID, req.SessionID)
	b.mu.Lock()
	if current := b.sessions[req.SessionID]; current != nil {
		if current.owner != input.ActorID || current.access != req.AccessID {
			b.mu.Unlock()
			return result("resume_unavailable")
		}
		current.mu.Lock()
		closed := current.closed
		current.mu.Unlock()
		if !closed {
			b.mu.Unlock()
			return current.snapshot()
		}
		delete(b.sessions, req.SessionID)
	}
	binding, ok := b.bindings[key]
	if !ok {
		b.mu.Unlock()
		return result("resume_unavailable")
	}
	if b.resuming[key] {
		b.mu.Unlock()
		return result("busy")
	}
	b.resuming[key] = true
	b.mu.Unlock()
	defer func() { b.mu.Lock(); delete(b.resuming, key); b.mu.Unlock() }()

	accessProject, err := filepath.EvalSymlinks(input.ProjectRoot)
	if input.ProjectRoot == "" || err != nil || binding.AccessProject != accessProject {
		return result("resume_unavailable")
	}
	project, err := filepath.EvalSymlinks(binding.Project)
	if err != nil || project != binding.Project {
		return result("resume_unavailable")
	}
	var installation discovery.Installation
	for _, item := range found.Installations {
		if !item.Wrapper && installationID(item) == binding.InstallationID {
			installation = item
			break
		}
	}
	if installation.Agent == "" {
		return result("resume_unavailable")
	}
	scope := agentruntime.Scope{Owner: input.ActorID, Access: "edge-agent-preview", Project: project}
	runtimeID, err := b.registry.Open(ctx, scope, b.adapter, installation)
	if err != nil {
		return result("launch_failed")
	}
	agent, err := b.registry.Lookup(scope, runtimeID)
	if err != nil {
		return result("launch_failed")
	}
	ready, err := agent.CheckAuthentication(ctx)
	if err != nil || !ready {
		if closeErr := agent.Close(); closeErr != nil {
			return result("unavailable")
		}
		return result("authentication_required")
	}
	capable, ok := agent.(agentruntime.ResumableSession)
	if !ok {
		if closeErr := agent.Close(); closeErr != nil {
			return result("unavailable")
		}
		return result("resume_unavailable")
	}
	thread, err := capable.ResumeThread(ctx, binding.Thread)
	if err != nil {
		if closeErr := agent.Close(); closeErr != nil {
			return result("unavailable")
		}
		if errors.Is(err, codex.ErrExternalTools) {
			return result("external_tools")
		}
		return result("resume_unavailable")
	}
	created := time.Now()
	if parsed, parseErr := time.Parse(time.RFC3339, seed.StartedAt); parseErr == nil {
		created = parsed
	}
	model := seed.Model
	if model == "" {
		model = thread.Model
	}
	s := &session{truncated: seed.Truncated, window: seed.Window, owner: input.ActorID, access: req.AccessID, id: req.SessionID, thread: thread.ID, agent: agent, status: "ok", created: created, touched: time.Now(), updated: time.Now(), revision: seed.Revision + 1, changed: make(chan struct{}), project: project, title: seed.Title, model: model, selectedModel: model, messages: append([]proto.EdgeAgentMessage{}, seed.Messages...), activities: append([]proto.AgentActivity{}, seed.Activities...), bytes: bytes}
	s.runtimeStarted = time.Now()
	s.version = thread.Version
	if capable, ok := agent.(agentruntime.SkillSession); ok {
		if skills, skillsErr := capable.Skills(ctx); skillsErr == nil {
			s.skillsAvailable = true
			for _, skill := range skills {
				s.skills = append(s.skills, proto.AgentSkill{ID: skill.ID, Name: skill.Name, Description: skill.Description})
			}
		}
	}
	b.mu.Lock()
	if b.sessions[s.id] != nil {
		b.mu.Unlock()
		if closeErr := agent.Close(); closeErr != nil {
			return result("unavailable")
		}
		return result("busy")
	}
	b.sessions[s.id] = s
	b.mu.Unlock()
	go b.consume(s)
	return s.snapshot()
}

func (b *Bridge) start(ctx context.Context, owner, project, accessProject string, i discovery.Installation, access ...string) proto.EdgeAgentResult {
	// Keep a bounded in-memory history without consuming live process slots.
	b.mu.Lock()
	if len(access) > 0 && access[0] != "" && len(b.bindings) >= maxNativeBindings {
		b.mu.Unlock()
		return result("unavailable")
	}
	active := 0
	var oldest *session
	for _, item := range b.sessions {
		item.mu.Lock()
		if !item.closed {
			active++
		} else if oldest == nil || item.closedAt.Before(oldest.closedAt) {
			oldest = item
		}
		item.mu.Unlock()
	}
	if len(b.sessions)+b.starting >= 32 && oldest != nil {
		delete(b.sessions, oldest.id)
	}
	full := active+b.starting >= 4
	if !full {
		b.starting++
	}
	b.mu.Unlock()
	if full {
		return result("busy")
	}
	defer func() { b.mu.Lock(); b.starting--; b.mu.Unlock() }()
	scope := agentruntime.Scope{Owner: owner, Access: "edge-agent-preview", Project: project}
	id, err := b.registry.Open(ctx, scope, b.adapter, i)
	if err != nil {
		return result("launch_failed")
	}
	agent, err := b.registry.Lookup(scope, id)
	if err != nil {
		return result("launch_failed")
	}
	ready, err := agent.CheckAuthentication(ctx)
	if err != nil || !ready {
		if closeErr := agent.Close(); closeErr != nil {
			return result("unavailable")
		}
		return result("authentication_required")
	}
	thread, err := agent.NewThread(ctx)
	if err != nil {
		if closeErr := agent.Close(); closeErr != nil {
			return result("unavailable")
		}
		if errors.Is(err, codex.ErrExternalTools) {
			return result("external_tools")
		}
		return result("launch_failed")
	}
	s := &session{owner: owner, id: id, thread: thread.ID, agent: agent, status: "ok", created: time.Now(), touched: time.Now(), updated: time.Now(), revision: 1, changed: make(chan struct{})}
	s.version = thread.Version
	s.model = thread.Model
	s.title = thread.Name
	s.project = project
	if len(access) > 0 {
		s.access = access[0]
	}
	if s.access != "" {
		binding := nativeBinding{Owner: owner, Access: s.access, Session: s.id, Thread: thread.ID, Project: project, AccessProject: accessProject, InstallationID: installationID(i)}
		b.mu.Lock()
		b.bindings[bindingKey(owner, s.access, s.id)] = binding
		if err = saveBindings(b.bindingPath, b.bindings); err != nil {
			delete(b.bindings, bindingKey(owner, s.access, s.id))
			b.mu.Unlock()
			if closeErr := agent.Close(); closeErr != nil {
				return result("unavailable")
			}
			return result("unavailable")
		}
		b.mu.Unlock()
	}
	if capable, ok := agent.(agentruntime.SkillSession); ok {
		if skills, err := capable.Skills(ctx); err == nil {
			s.skillsAvailable = true
			for _, skill := range skills {
				s.skills = append(s.skills, proto.AgentSkill{ID: skill.ID, Name: skill.Name, Description: skill.Description})
			}
		}
	}
	b.mu.Lock()
	b.sessions[id] = s
	b.mu.Unlock()
	go b.consume(s)
	return s.snapshot()
}
func (s *session) snapshot() proto.EdgeAgentResult {
	s.mu.Lock()
	defer s.mu.Unlock()
	approvals, permissions, mode := s.permissionSnapshotLocked()
	return boundedSnapshot(proto.EdgeAgentResult{Window: s.window, HistoryWindowing: s.access != "", Truncated: s.truncated, InputRequests: s.inputSnapshotLocked(), Approvals: approvals, PermissionsAvailable: permissions, PermissionMode: mode, Version: 1, Status: s.status, SessionID: s.id, ThreadID: s.thread, AgentVersion: s.version, Model: s.model, Project: s.project, StartedAt: s.created.UTC().Format(time.RFC3339), Skills: append([]proto.AgentSkill{}, s.skills...), SkillsAvailable: s.skillsAvailable, Running: s.running, Closed: s.closed, Messages: append([]proto.EdgeAgentMessage{}, s.messages...), Revision: s.revision, Activities: append([]proto.AgentActivity{}, s.activities...), Title: s.displayTitleLocked(), SessionManagement: true, ModelsAvailable: s.modelsAvailable, Models: append([]proto.AgentModel{}, s.models...)})
}
func (s *session) stop(status string) {
	s.mu.Lock()
	already := s.closed
	s.closed = true
	s.running = false
	s.approvals = nil
	s.inputs = nil
	for i := range s.activities {
		if s.activities[i].Status == "running" {
			s.activities[i].Status = "ended"
		}
	}
	if !already {
		s.closedAt = time.Now()
		s.status = status
		s.changedLocked()
	}
	s.mu.Unlock()
	if !already {
		if err := s.agent.Close(); err != nil {
			s.mu.Lock()
			s.status = "unavailable"
			s.mu.Unlock()
		}
	}
}
func (b *Bridge) consume(s *session) {
	defer func() {
		if recover() != nil {
			s.stop("unavailable")
		}
	}()
	for {
		select {
		case <-b.ctx.Done():
			s.stop("session_closed")
			return
		case event, ok := <-s.agent.Events():
			if !ok {
				s.stop("session_closed")
				return
			}
			if len(event.ID) > 0 {
				if s.queueInput(event) {
					continue
				}
				if s.queueApproval(event) {
					continue
				}
				ctx, cancel := context.WithTimeout(b.ctx, 5*time.Second)
				err := s.agent.RejectRequest(ctx, event)
				cancel()
				if err != nil {
					s.stop("approval_declined")
					return
				}
				s.mu.Lock()
				s.status = "approval_declined"
				s.changedLocked()
				s.mu.Unlock()
				continue
			}
			var p struct {
				ThreadID   string          `json:"threadId"`
				TurnID     string          `json:"turnId"`
				Delta      string          `json:"delta"`
				ItemID     string          `json:"itemId"`
				ThreadName string          `json:"threadName"`
				Item       nativeActivity  `json:"item"`
				Diff       string          `json:"diff"`
				RequestID  json.RawMessage `json:"requestId"`
				Turn       struct {
					ID     string `json:"id"`
					Status string `json:"status"`
				} `json:"turn"`
			}
			if json.Unmarshal(event.Params, &p) != nil || p.ThreadID != s.thread {
				continue
			}
			s.mu.Lock()
			if s.closed {
				s.mu.Unlock()
				return
			}
			switch event.Method {
			case "serverRequest/resolved":
				s.resolveRequestLocked(p.RequestID)
			case "thread/name/updated":
				if !s.renamed && validNativeTitle(p.ThreadName) {
					s.title = p.ThreadName
					s.changedLocked()
				}
			case "item/agentMessage/delta":
				if s.running && len(s.messages) > 0 && (s.turn == "" || p.TurnID == s.turn) {
					last := len(s.messages) - 1
					if p.ItemID != "" && s.messages[last].ItemID != "" && s.messages[last].ItemID != p.ItemID {
						s.messages = append(s.messages, proto.EdgeAgentMessage{Role: "assistant"})
						last++
					}
					s.messages[last].ItemID = p.ItemID
					s.messages[len(s.messages)-1].Text += p.Delta
					s.bytes += len(p.Delta)
					s.boundDisplayLocked()
					s.changedLocked()
				}
			case "item/started", "item/completed":
				if s.running && (s.turn == "" || p.TurnID == s.turn) {
					s.recordActivityLocked(p.Item.ID, p.Item.Type, p.Item.Status, p.Item.DurationMS, event.Method == "item/completed")
					s.recordActivityDetailsLocked(p.Item)
				}
			case "item/commandExecution/outputDelta":
				if s.running && (s.turn == "" || p.TurnID == s.turn) {
					s.recordActivityOutputLocked(p.ItemID, p.Delta, true)
				}
			case "turn/diff/updated":
				if s.running && p.TurnID != "" && (s.turn == "" || p.TurnID == s.turn) {
					id := "turn-diff:" + p.TurnID
					s.recordActivityLocked(id, "turnDiff", "", 0, false)
					s.recordActivityOutputLocked(id, p.Diff, false)
				}
			case "turn/completed":
				if s.running && (s.turn == "" || s.turn == p.Turn.ID) {
					s.running = false
					s.approvals = nil
					s.inputs = nil
					for i := range s.activities {
						if s.activities[i].Status == "running" {
							s.activities[i].Status = "ended"
						}
					}
					if p.Turn.Status == "failed" {
						s.status = "turn_failed"
					}
					s.changedLocked()
				}
			}
			s.mu.Unlock()
		}
	}
}
func (b *Bridge) reap() {
	defer close(b.done)
	defer func() {
		if recover() != nil {
			b.cancel()
		}
	}()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-b.ctx.Done():
			return
		case now := <-ticker.C:
			b.mu.Lock()
			list := make([]*session, 0, len(b.sessions))
			for _, s := range b.sessions {
				list = append(list, s)
			}
			b.mu.Unlock()
			for _, s := range list {
				s.mu.Lock()
				expired, forget := s.expirationLocked(now)
				s.mu.Unlock()
				if expired {
					s.stop("expired")
				}
				if forget {
					b.mu.Lock()
					delete(b.sessions, s.id)
					b.mu.Unlock()
				}
			}
		}
	}
}
