package proto

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAgentSearchValidationAndReplyTokens(t *testing.T) {
	request := EdgeAgentRequest{Action: "sessions", EdgeID: 1, AccessID: strings.Repeat("a", 32), HistorySearch: "项目"}
	require.True(t, request.Valid())
	request.HistorySearch = strings.Repeat("字", 101)
	require.False(t, request.Valid())
	request.HistorySearch = "line\nbreak"
	require.False(t, request.Valid())
	request.HistorySearch = "valid"
	request.Action = "stop"
	require.False(t, request.Valid())
	messages := []EdgeAgentMessage{{Role: "user", Text: "hello"}}
	require.Empty(t, AgentReplyToken(1, messages))
	messages = append(messages, EdgeAgentMessage{Role: "assistant", Text: "answer"})
	first := AgentReplyToken(1, messages)
	messages[0].Text = "another user prompt"
	require.Equal(t, first, AgentReplyToken(1, messages))
	messages[1].Text += " more"
	require.NotEqual(t, first, AgentReplyToken(1, messages))
	require.NotEqual(t, AgentReplyToken(1, messages), AgentReplyToken(2, messages))
}
