package tool

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTimeoutMiddleware_ToolDeadlineReturnsModelReadableFailure(t *testing.T) {
	handler := TimeoutMiddleware(time.Millisecond).Wrap(func(ctx context.Context, _ ToolInvocation) (ToolResult, error) {
		<-ctx.Done()
		return ToolResult{}, ctx.Err()
	})
	result, err := handler(context.Background(), ToolInvocation{})
	require.NoError(t, err)
	require.True(t, result.IsError)
	require.Equal(t, OutputError, result.Kind)
	require.Contains(t, string(result.Content), "tool_timeout")
	require.Contains(t, string(result.Content), "automatically retry")
}

func TestTimeoutMiddleware_ParentCancellationStillStopsRuntime(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	handler := TimeoutMiddleware(time.Minute).Wrap(func(ctx context.Context, _ ToolInvocation) (ToolResult, error) { return ToolResult{}, ctx.Err() })
	_, err := handler(ctx, ToolInvocation{})
	require.ErrorIs(t, err, context.Canceled)
}
