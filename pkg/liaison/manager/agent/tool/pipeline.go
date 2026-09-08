package tool

import (
	"context"
	"encoding/json"
	"errors"
	"time"
)

func Chain(handler ToolHandler, middleware ...ToolMiddleware) ToolHandler {
	for index := len(middleware) - 1; index >= 0; index-- {
		handler = middleware[index].Wrap(handler)
	}
	return handler
}

func TimeoutMiddleware(timeout time.Duration) ToolMiddleware {
	return ToolMiddlewareFunc(func(next ToolHandler) ToolHandler {
		return func(ctx context.Context, invocation ToolInvocation) (ToolResult, error) {
			if timeout <= 0 {
				return next(ctx, invocation)
			}
			timedContext, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			result, err := next(timedContext, invocation)
			// A tool deadline is an execution outcome, not a failed chat request.
			// Parent cancellation must still stop the runtime immediately.
			if ctx.Err() == nil && errors.Is(timedContext.Err(), context.DeadlineExceeded) && errors.Is(err, context.DeadlineExceeded) {
				content, encodeErr := json.Marshal(map[string]any{
					"code": "tool_timeout", "timeout_seconds": timeout.Seconds(),
					"message": "Tool execution exceeded its time limit. Remote completion is unknown. Do not claim success or automatically retry. Explain the timeout and ask before any further execution; prefer a narrower scope.",
				})
				if encodeErr != nil {
					return ToolResult{}, encodeErr
				}
				return ToolResult{Kind: OutputError, IsError: true, Content: content}, nil
			}
			return result, err
		}
	})
}
