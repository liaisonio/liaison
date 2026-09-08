package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

var (
	ErrExecutorNotFound = errors.New("protocol executor not found")
	ErrOperationDenied  = errors.New("protocol operation not supported")
)

type Operation string

const (
	OperationTerminalRead    Operation = "terminal.read"
	OperationTerminalExecute Operation = "terminal.execute"
	OperationDataSchema      Operation = "data.schema"
	OperationDataQuery       Operation = "data.query"
	OperationDesktopInfo     Operation = "desktop.session_info"
	OperationDesktopCapture  Operation = "desktop.capture"
	OperationDesktopInput    Operation = "desktop.input"
)

type Request struct {
	Invocation tool.ToolInvocation
	Operation  Operation
	Input      json.RawMessage
}

type Result struct {
	Kind       tool.OutputKind
	IsError    bool
	Content    json.RawMessage
	Truncated  bool
	ArtifactID string
}

type ProtocolExecutor interface {
	Protocols() []tool.Protocol
	Operations() []Operation
	Execute(ctx context.Context, request Request) (Result, error)
}

// Router owns protocol executor registration. Runtime sessions remain owned by
// the protocol implementations; the Agent only routes a bound operation.
type Router struct {
	mu        sync.RWMutex
	executors map[tool.Protocol]ProtocolExecutor
}

func NewRouter() *Router {
	return &Router{executors: make(map[tool.Protocol]ProtocolExecutor)}
}

func (router *Router) Register(executor ProtocolExecutor) error {
	if executor == nil {
		return errors.New("protocol executor is required")
	}
	protocols := executor.Protocols()
	if len(protocols) == 0 || len(executor.Operations()) == 0 {
		return errors.New("protocol executor requires protocols and operations")
	}
	router.mu.Lock()
	defer router.mu.Unlock()
	for _, protocol := range protocols {
		if protocol == "" || protocol == tool.ProtocolAny {
			return fmt.Errorf("invalid concrete executor protocol %q", protocol)
		}
		if _, exists := router.executors[protocol]; exists {
			return fmt.Errorf("protocol %s already has an executor", protocol)
		}
	}
	for _, protocol := range protocols {
		router.executors[protocol] = executor
	}
	return nil
}

func (router *Router) Unregister(executor ProtocolExecutor) {
	if executor == nil {
		return
	}
	router.mu.Lock()
	defer router.mu.Unlock()
	for protocol, registered := range router.executors {
		if registered == executor {
			delete(router.executors, protocol)
		}
	}
}

func (router *Router) Execute(ctx context.Context, request Request) (Result, error) {
	protocol := request.Invocation.Binding.Attachment.Protocol
	router.mu.RLock()
	executor, ok := router.executors[protocol]
	router.mu.RUnlock()
	if !ok {
		return Result{}, fmt.Errorf("%w: %s", ErrExecutorNotFound, protocol)
	}
	if !supportsOperation(executor.Operations(), request.Operation) {
		return Result{}, fmt.Errorf("%w: %s on %s", ErrOperationDenied, request.Operation, protocol)
	}
	result, err := executor.Execute(ctx, request)
	if err != nil {
		return Result{}, fmt.Errorf("execute %s on %s: %w", request.Operation, protocol, err)
	}
	if len(result.Content) == 0 || !json.Valid(result.Content) {
		return Result{}, errors.New("protocol executor returned invalid JSON content")
	}
	return result, nil
}

func supportsOperation(operations []Operation, wanted Operation) bool {
	for _, operation := range operations {
		if operation == wanted {
			return true
		}
	}
	return false
}
