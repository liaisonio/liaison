package executor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
)

type SessionResolver interface {
	Resolve(ctx context.Context, request accesssession.ResolveRequest) (accesssession.Handle, error)
}

type SessionExecutor struct {
	resolver SessionResolver
}

func NewSessionExecutor(resolver SessionResolver) (*SessionExecutor, error) {
	if resolver == nil {
		return nil, errors.New("access session resolver is required")
	}
	return &SessionExecutor{resolver: resolver}, nil
}

func (*SessionExecutor) Protocols() []tool.Protocol {
	return []tool.Protocol{
		tool.ProtocolWebSSH,
		tool.ProtocolMySQL,
		tool.ProtocolMariaDB,
		tool.ProtocolSQLServer,
		tool.ProtocolOracle,
		tool.ProtocolClickHouse,
		tool.ProtocolElasticsearch,
		tool.ProtocolOpenSearch,
		tool.ProtocolPostgreSQL,
		tool.ProtocolRedis,
		tool.ProtocolMongoDB,
		tool.ProtocolRDP,
		tool.ProtocolVNC,
	}
}

func (*SessionExecutor) Operations() []Operation {
	return []Operation{
		OperationTerminalRead,
		OperationTerminalExecute,
		OperationDataSchema,
		OperationDataQuery,
		OperationDesktopInfo,
	}
}

func (executor *SessionExecutor) Execute(ctx context.Context, request Request) (Result, error) {
	attachment := request.Invocation.Binding.Attachment
	handle, err := executor.resolver.Resolve(ctx, accesssession.ResolveRequest{
		ID:            attachment.AccessHandleID,
		UserID:        request.Invocation.Binding.Principal.UserID,
		AccessID:      attachment.AccessID,
		ApplicationID: attachment.ApplicationID,
		Protocol:      accesssession.Protocol(attachment.Protocol),
		Generation:    attachment.Generation,
	})
	if err != nil {
		return Result{}, fmt.Errorf("resolve attached access session: %w", err)
	}

	switch request.Operation {
	case OperationTerminalRead:
		if handle.Terminal == nil {
			return Result{}, ErrOperationDenied
		}
		var input terminalReadInput
		if err := decodeInput(request.Input, &input); err != nil {
			return Result{}, err
		}
		content, err := handle.Terminal.Read(ctx, input.MaxLines)
		return protocolResult(tool.OutputText, content, err)
	case OperationTerminalExecute:
		if handle.Terminal == nil {
			return Result{}, ErrOperationDenied
		}
		var input terminalExecuteInput
		if err := decodeInput(request.Input, &input); err != nil {
			return Result{}, err
		}
		input.Command = strings.TrimSpace(input.Command)
		if input.Command == "" {
			return Result{}, errors.New("terminal command is required")
		}
		content, err := handle.Terminal.Execute(ctx, input.Command)
		return protocolResult(tool.OutputText, content, err)
	case OperationDataSchema:
		if handle.Data == nil {
			return Result{}, ErrOperationDenied
		}
		var input dataSchemaInput
		if err := decodeInput(request.Input, &input); err != nil {
			return Result{}, err
		}
		content, err := handle.Data.Schema(ctx, input.Path)
		return protocolResult(tool.OutputFacts, content, err)
	case OperationDataQuery:
		if handle.Data == nil {
			return Result{}, ErrOperationDenied
		}
		var input dataQueryInput
		if err := decodeInput(request.Input, &input); err != nil {
			return Result{}, err
		}
		input.Statement = strings.TrimSpace(input.Statement)
		if input.Statement == "" {
			return Result{}, errors.New("data statement is required")
		}
		content, err := handle.Data.Query(ctx, input.Statement)
		return protocolResult(tool.OutputTable, content, err)
	case OperationDesktopInfo:
		if handle.Desktop == nil {
			return Result{}, ErrOperationDenied
		}
		content, err := handle.Desktop.SessionInfo(ctx)
		return protocolResult(tool.OutputFacts, content, err)
	default:
		return Result{}, fmt.Errorf("%w: %s", ErrOperationDenied, request.Operation)
	}
}

type terminalReadInput struct {
	MaxLines int `json:"max_lines"`
}

type terminalExecuteInput struct {
	Command string `json:"command"`
}

type dataSchemaInput struct {
	Path []string `json:"path"`
}

type dataQueryInput struct {
	Statement string `json:"statement"`
}

func decodeInput(input json.RawMessage, destination interface{}) error {
	if len(input) == 0 {
		input = json.RawMessage(`{}`)
	}
	if err := json.Unmarshal(input, destination); err != nil {
		return fmt.Errorf("decode protocol tool input: %w", err)
	}
	return nil
}

func protocolResult(kind tool.OutputKind, content json.RawMessage, err error) (Result, error) {
	if err != nil {
		return Result{}, err
	}
	if len(content) == 0 || !json.Valid(content) {
		return Result{}, errors.New("access session returned invalid JSON content")
	}
	var envelope struct {
		Error     string `json:"error"`
		Truncated bool   `json:"truncated"`
	}
	if err := json.Unmarshal(content, &envelope); err == nil {
		if envelope.Error != "" {
			return Result{Kind: tool.OutputError, Content: content, IsError: true, Truncated: envelope.Truncated}, nil
		}
		return Result{Kind: kind, Content: content, Truncated: envelope.Truncated}, nil
	}
	return Result{Kind: kind, Content: content}, nil
}
