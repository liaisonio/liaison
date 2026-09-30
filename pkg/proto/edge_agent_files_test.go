package proto

import (
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestAgentFileValidation(t *testing.T) {
	base := EdgeAgentRequest{Action: "file_begin", AccessID: strings.Repeat("a", 32), SessionID: strings.Repeat("b", 32)}
	for _, name := range []string{"../a", "a/b", "a\\b", "..", "a\x00b", "a\nb", "a:b"} {
		r := base
		r.File = &AgentFileOperation{Name: name}
		require.False(t, r.Valid(), name)
	}
	base.File = &AgentFileOperation{Name: "report.txt", Size: AgentFileLimit}
	require.True(t, base.Valid())
	base.File.Size++
	require.False(t, base.Valid())
	base.Action = "file_write"
	base.File = &AgentFileOperation{TransferID: strings.Repeat("c", 32), Data: make([]byte, AgentFileChunk+1)}
	require.False(t, base.Valid())
	base.File.Data = base.File.Data[:AgentFileChunk]
	require.True(t, base.Valid())
	base.Action = "send"
	base.File = nil
	base.Attachments = []string{".liaison-attachments/test/file"}
	require.True(t, base.Valid())
	base.Attachments = append(base.Attachments, base.Attachments[0])
	require.False(t, base.Valid())
	base.Action = "poll"
	require.False(t, base.Valid())
}
