package aigateway

import (
	"context"
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type interruptedStream struct{ err error }

func (r interruptedStream) Read([]byte) (int, error) { return 0, r.err }

func TestStreamInterruptionRetainsObservedUsage(t *testing.T) {
	for _, protocol := range []string{"openai-compatible", "anthropic"} {
		for _, interruption := range []error{context.Canceled, context.DeadlineExceeded, io.ErrUnexpectedEOF} {
			t.Run(protocol+"/"+interruption.Error(), func(t *testing.T) {
				frame := `{"choices":[],"usage":{"prompt_tokens":12}}`
				if protocol == "anthropic" {
					frame = `{"type":"message_start","message":{"id":"fixture","usage":{"input_tokens":12}}}`
				}
				reader := io.MultiReader(strings.NewReader("data: "+frame+"\n\n"), interruptedStream{interruption})
				var output strings.Builder
				var usage Usage
				err := RelaySSE(reader, protocol, "public", func(b []byte) error { _, e := output.Write(b); return e }, &usage)
				require.ErrorIs(t, err, interruption)
				require.False(t, usage.Complete)
				require.NotNil(t, usage.Input)
				require.EqualValues(t, 12, *usage.Input)
				require.Nil(t, usage.Output)
				require.NotContains(t, output.String(), "[DONE]")
			})
		}
	}
}

func TestStreamTerminalWriteFailureIsNotComplete(t *testing.T) {
	stream := "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":12,\"completion_tokens\":3}}\n\ndata: [DONE]\n\n"
	var usage Usage
	err := RelaySSE(strings.NewReader(stream), "openai-compatible", "public", func(b []byte) error {
		if strings.Contains(string(b), "[DONE]") {
			return io.ErrClosedPipe
		}
		return nil
	}, &usage)
	require.ErrorIs(t, err, io.ErrClosedPipe)
	require.False(t, usage.Complete)
	require.EqualValues(t, 12, *usage.Input)
	require.EqualValues(t, 3, *usage.Output)
}
