package tool

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDisclosure_WhenToolsDifferByProtocol_OnlyExposesMatchingAttachment(t *testing.T) {
	engine := NewEngine(nil, nil)
	ssh := testRegistration(testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment), "ssh")
	mysql := testRegistration(testDescriptor("mysql", "query", "1.0.0", DisclosureAttachment), "mysql")
	require.NoError(t, engine.Register(context.Background(), ssh))
	require.NoError(t, engine.Register(context.Background(), mysql))

	snapshot, err := engine.BuildSnapshot(context.Background(), DisclosureRequest{
		Attachments: []AttachmentSnapshot{{ID: "ssh-1", Protocol: ProtocolWebSSH, Capabilities: []Capability{"terminal.execute"}, Generation: 1}},
	})
	require.NoError(t, err)
	require.Len(t, snapshot.Tools, 1)
	assert.Equal(t, ssh.Descriptor.ID, snapshot.Tools[0].Descriptor.ID)
}

func TestDisclosure_WhenToolIsDeferred_RequiresQueryOrPromotion(t *testing.T) {
	engine := NewEngine(nil, nil)
	descriptor := testDescriptor("ssh", "upload_file", "1.0.0", DisclosureDeferred)
	descriptor.Tags = []string{"upload", "file"}
	require.NoError(t, engine.Register(context.Background(), testRegistration(descriptor, "uploaded")))
	request := DisclosureRequest{Attachments: []AttachmentSnapshot{{ID: "ssh-1", Protocol: ProtocolWebSSH, Capabilities: []Capability{"terminal.execute"}, Generation: 1}}}

	snapshot, err := engine.BuildSnapshot(context.Background(), request)
	require.NoError(t, err)
	assert.Empty(t, snapshot.Tools)

	results, err := engine.Search(context.Background(), request, "upload")
	require.NoError(t, err)
	require.Len(t, results, 1)
	assert.Equal(t, descriptor.ID, results[0].ID)

	request.Promoted = []ToolID{descriptor.ID}
	snapshot, err = engine.BuildSnapshot(context.Background(), request)
	require.NoError(t, err)
	require.Len(t, snapshot.Tools, 1)
	assert.Equal(t, descriptor.ID, snapshot.Tools[0].Descriptor.ID)
}

func TestDisclosure_WhenBudgetIsSmall_PrefersPromotedAndLowerRiskTools(t *testing.T) {
	engine := NewEngine(nil, nil)
	lowRisk := testDescriptor("mysql", "select", "1.0.0", DisclosureAttachment)
	highRisk := testDescriptor("mysql", "drop_table", "1.0.0", DisclosureAttachment)
	highRisk.Risk = RiskCritical
	promoted := testDescriptor("mysql", "explain", "1.0.0", DisclosureDeferred)
	for _, descriptor := range []ToolDescriptor{lowRisk, highRisk, promoted} {
		require.NoError(t, engine.Register(context.Background(), testRegistration(descriptor, descriptor.ID.Name)))
	}

	snapshot, err := engine.BuildSnapshot(context.Background(), DisclosureRequest{
		Attachments: []AttachmentSnapshot{{ID: "db", Protocol: ProtocolMySQL, Capabilities: []Capability{"sql.query"}, Generation: 1}},
		Promoted:    []ToolID{promoted.ID},
		Budget:      DisclosureBudget{MaxAlwaysVisible: 1, MaxCandidates: 1, MaxSchemas: 2, MaxTokens: 10000},
	})
	require.NoError(t, err)
	require.Len(t, snapshot.Tools, 2)
	assert.Equal(t, promoted.ID, snapshot.Tools[0].Descriptor.ID)
	assert.Equal(t, lowRisk.ID, snapshot.Tools[1].Descriptor.ID)
}

func TestEngineExecute_WhenToolWasNotExposed_ReturnsError(t *testing.T) {
	engine := NewEngine(nil, nil)
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	require.NoError(t, engine.Register(context.Background(), testRegistration(descriptor, "ok")))
	snapshot := ToolSetSnapshot{ID: "snapshot", AttachmentGeneration: map[string]uint64{}}

	_, err := engine.Execute(context.Background(), snapshot, ToolInvocation{
		ID:      "invocation",
		Call:    ToolCall{ID: descriptor.ID, Input: json.RawMessage(`{}`)},
		Binding: ToolBinding{ToolSnapshotID: snapshot.ID},
	})
	assert.ErrorIs(t, err, ErrToolNotExposed)
}

func TestEngineExecute_WhenAttachmentGenerationChanged_ReturnsStaleSnapshot(t *testing.T) {
	engine := NewEngine(nil, nil)
	registration := testRegistration(testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment), "ok")
	require.NoError(t, engine.Register(context.Background(), registration))
	snapshot, err := engine.BuildSnapshot(context.Background(), DisclosureRequest{
		Attachments: []AttachmentSnapshot{{ID: "ssh-1", Protocol: ProtocolWebSSH, Capabilities: []Capability{"terminal.execute"}, Generation: 1}},
	})
	require.NoError(t, err)

	_, err = engine.Execute(context.Background(), snapshot, ToolInvocation{
		ID:   "invocation",
		Call: ToolCall{ID: registration.Descriptor.ID, Input: json.RawMessage(`{}`)},
		Binding: ToolBinding{
			ToolSnapshotID: snapshot.ID,
			Attachment:     AttachmentSnapshot{ID: "ssh-1", Generation: 2},
		},
	})
	assert.ErrorIs(t, err, ErrStaleSnapshot)
}

func TestEngineExecute_WhenApprovalRequired_DoesNotBindExecutor(t *testing.T) {
	factory := &recordingFactory{executor: &staticExecutor{content: "never"}}
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	descriptor.Approval = ApprovalAlways
	engine := NewEngine(nil, nil)
	require.NoError(t, engine.Register(context.Background(), ToolRegistration{Descriptor: descriptor, Factory: factory}))
	snapshot := requireSnapshot(t, engine, descriptor)

	_, err := engine.Execute(context.Background(), snapshot, invocationFor(snapshot, descriptor.ID))
	assert.ErrorIs(t, err, ErrApprovalRequired)
	assert.Equal(t, 0, factory.bindCount())
}

func TestEngineExecuteApproved_OnlyAcceptsExactUnexpiredGrant(t *testing.T) {
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	descriptor.Approval = ApprovalAlways
	engine := NewEngine(nil, nil)
	require.NoError(t, engine.Register(context.Background(), testRegistration(descriptor, "approved")))
	snapshot := requireSnapshot(t, engine, descriptor)
	invocation := invocationFor(snapshot, descriptor.ID)
	digest := sha256.Sum256(invocation.Call.Input)
	grant := ApprovalGrant{ApprovalID: "approval", InvocationID: invocation.ID, ToolID: descriptor.ID,
		ToolSnapshotID: snapshot.ID, InputSHA256: hex.EncodeToString(digest[:]),
		AttachmentGeneration: invocation.Binding.Attachment.Generation, ExpiresAt: time.Now().Add(time.Minute)}

	result, err := engine.ExecuteApproved(context.Background(), snapshot, invocation, grant)
	require.NoError(t, err)
	assert.JSONEq(t, `"approved"`, string(result.Content))

	grant.InputSHA256 = "different"
	_, err = engine.ExecuteApproved(context.Background(), snapshot, invocation, grant)
	assert.ErrorIs(t, err, ErrApprovalRequired)
}

func TestEngineExecute_AppliesMiddlewareInDeclaredOrder(t *testing.T) {
	var mu sync.Mutex
	order := make([]string, 0, 5)
	middleware := func(name string) ToolMiddleware {
		return ToolMiddlewareFunc(func(next ToolHandler) ToolHandler {
			return func(ctx context.Context, invocation ToolInvocation) (ToolResult, error) {
				mu.Lock()
				order = append(order, name+":before")
				mu.Unlock()
				result, err := next(ctx, invocation)
				mu.Lock()
				order = append(order, name+":after")
				mu.Unlock()
				return result, err
			}
		})
	}
	executor := executorFunc(func(context.Context, json.RawMessage) (ToolResult, error) {
		mu.Lock()
		order = append(order, "execute")
		mu.Unlock()
		return ToolResult{Kind: OutputText, Content: json.RawMessage(`"ok"`)}, nil
	})
	engine := NewEngine(nil, nil, middleware("auth"), middleware("audit"))
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	require.NoError(t, engine.Register(context.Background(), ToolRegistration{Descriptor: descriptor, Factory: staticFactory{executor: executor}}))
	snapshot := requireSnapshot(t, engine, descriptor)

	_, err := engine.Execute(context.Background(), snapshot, invocationFor(snapshot, descriptor.ID))
	require.NoError(t, err)
	assert.Equal(t, []string{"auth:before", "audit:before", "execute", "audit:after", "auth:after"}, order)
}

func TestEngineReplace_InvalidatesOldSnapshotAndExposesNewVersion(t *testing.T) {
	engine := NewEngine(nil, nil)
	v1 := testRegistration(testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment), "v1")
	require.NoError(t, engine.Register(context.Background(), v1))
	oldSnapshot := requireSnapshot(t, engine, v1.Descriptor)

	v2 := testRegistration(testDescriptor("ssh", "execute", "2.0.0", DisclosureAttachment), "v2")
	require.NoError(t, engine.Replace(context.Background(), v2))
	_, err := engine.Execute(context.Background(), oldSnapshot, invocationFor(oldSnapshot, v1.Descriptor.ID))
	assert.ErrorIs(t, err, ErrToolUnavailable)

	newSnapshot := requireSnapshot(t, engine, v2.Descriptor)
	result, err := engine.Execute(context.Background(), newSnapshot, invocationFor(newSnapshot, v2.Descriptor.ID))
	require.NoError(t, err)
	assert.JSONEq(t, `"v2"`, string(result.Content))
}

func TestEngineUnload_ForceCancelsInFlightCall(t *testing.T) {
	started := make(chan struct{})
	finished := make(chan error, 1)
	executor := executorFunc(func(ctx context.Context, _ json.RawMessage) (ToolResult, error) {
		close(started)
		<-ctx.Done()
		return ToolResult{}, ctx.Err()
	})
	engine := NewEngine(nil, nil)
	descriptor := testDescriptor("ssh", "execute", "1.0.0", DisclosureAttachment)
	require.NoError(t, engine.Register(context.Background(), ToolRegistration{Descriptor: descriptor, Factory: staticFactory{executor: executor}}))
	snapshot := requireSnapshot(t, engine, descriptor)
	go func() {
		_, err := engine.Execute(context.Background(), snapshot, invocationFor(snapshot, descriptor.ID))
		finished <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("tool did not start")
	}

	complete, err := engine.Unload(context.Background(), descriptor.ID, true)
	require.NoError(t, err)
	assert.False(t, complete)
	select {
	case executeErr := <-finished:
		assert.ErrorIs(t, executeErr, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("force unload did not cancel the tool")
	}
	_, _, _, exists := engine.Status(descriptor.ID)
	assert.False(t, exists)
}

func testDescriptor(namespace, name, version string, disclosure DisclosureMode) ToolDescriptor {
	protocols := []Protocol{ProtocolWebSSH, ProtocolSSH}
	capabilities := []Capability{"terminal.execute"}
	if namespace == "mysql" {
		protocols = []Protocol{ProtocolMySQL}
		capabilities = []Capability{"sql.query"}
	}
	return ToolDescriptor{
		ID:             ToolID{Namespace: namespace, Name: name, Version: version},
		DisplayName:    name,
		Description:    "Test " + name,
		WhenToUse:      "Use for " + name,
		InputSchema:    json.RawMessage(`{"type":"object"}`),
		OutputKinds:    []OutputKind{OutputText},
		Protocols:      protocols,
		Capabilities:   capabilities,
		Risk:           RiskReadOnly,
		Approval:       ApprovalNever,
		Disclosure:     disclosure,
		DefaultTimeout: time.Second,
		Source:         ToolSourceRef{ID: "test", Kind: "builtin", Trust: TrustBuiltin},
	}
}

func testRegistration(descriptor ToolDescriptor, output string) ToolRegistration {
	return ToolRegistration{Descriptor: descriptor, Factory: staticFactory{executor: &staticExecutor{content: output}}}
}

func requireSnapshot(t *testing.T, engine *Engine, descriptor ToolDescriptor) ToolSetSnapshot {
	t.Helper()
	protocol := descriptor.Protocols[0]
	snapshot, err := engine.BuildSnapshot(context.Background(), DisclosureRequest{
		Attachments: []AttachmentSnapshot{{ID: "attachment", Protocol: protocol, Capabilities: append([]Capability(nil), descriptor.Capabilities...), Generation: 1}},
		Promoted:    []ToolID{descriptor.ID},
	})
	require.NoError(t, err)
	require.NotEmpty(t, snapshot.Tools)
	return snapshot
}

func invocationFor(snapshot ToolSetSnapshot, id ToolID) ToolInvocation {
	return ToolInvocation{
		ID:   "invocation",
		Call: ToolCall{ID: id, Input: json.RawMessage(`{}`)},
		Binding: ToolBinding{
			ToolSnapshotID: snapshot.ID,
			Attachment:     AttachmentSnapshot{ID: "attachment", Generation: 1},
		},
	}
}

type staticFactory struct {
	executor ToolExecutor
}

func (factory staticFactory) Bind(context.Context, ToolBinding) (ToolExecutor, error) {
	return factory.executor, nil
}

type recordingFactory struct {
	mu       sync.Mutex
	bound    int
	executor ToolExecutor
}

func (factory *recordingFactory) Bind(context.Context, ToolBinding) (ToolExecutor, error) {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	factory.bound++
	return factory.executor, nil
}

func (factory *recordingFactory) bindCount() int {
	factory.mu.Lock()
	defer factory.mu.Unlock()
	return factory.bound
}

type staticExecutor struct {
	content string
}

func (executor *staticExecutor) Execute(context.Context, json.RawMessage) (ToolResult, error) {
	content, err := json.Marshal(executor.content)
	if err != nil {
		return ToolResult{}, err
	}
	return ToolResult{Kind: OutputText, Content: content}, nil
}

type executorFunc func(context.Context, json.RawMessage) (ToolResult, error)

func (fn executorFunc) Execute(ctx context.Context, input json.RawMessage) (ToolResult, error) {
	return fn(ctx, input)
}
