package bridge

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/stretchr/testify/require"
)

func fileFixture(t *testing.T) (*Bridge, proto.EdgeAgentRPCRequest) {
	t.Helper()
	b, _, start := setup(t)
	start.AccessID = strings.Repeat("a", 32)
	input := proto.EdgeAgentRPCRequest{Version: 1, ActorID: "owner", ProjectRoot: start.Project, Request: start}
	out := b.Handle(context.Background(), input)
	require.Equal(t, "ok", out.Status)
	input.Request = proto.EdgeAgentRequest{AccessID: start.AccessID, SessionID: out.SessionID}
	return b, input
}
func fileCall(b *Bridge, input proto.EdgeAgentRPCRequest, action string, f proto.AgentFileOperation) proto.EdgeAgentResult {
	input.Request.Action = action
	input.Request.File = &f
	return b.Handle(context.Background(), input)
}
func uploadTestFile(t *testing.T, b *Bridge, input proto.EdgeAgentRPCRequest, name string, data []byte) proto.AgentFile {
	t.Helper()
	begun := fileCall(b, input, "file_begin", proto.AgentFileOperation{Name: name, Size: int64(len(data))})
	require.Equal(t, "ok", begun.Status)
	for offset := 0; offset < len(data); offset += proto.AgentFileChunk {
		out := fileCall(b, input, "file_write", proto.AgentFileOperation{TransferID: begun.TransferID, Offset: int64(offset), Data: data[offset:min(offset+proto.AgentFileChunk, len(data))]})
		require.Equal(t, "ok", out.Status)
	}
	committed := fileCall(b, input, "file_commit", proto.AgentFileOperation{TransferID: begun.TransferID})
	require.Equal(t, "ok", committed.Status)
	require.NotNil(t, committed.File)
	return *committed.File
}
func TestFileRoundTripAndDistinctNames(t *testing.T) {
	b, input := fileFixture(t)
	payload := bytes.Repeat([]byte("abc"), 100000)
	first := uploadTestFile(t, b, input, "report.txt", payload)
	second := uploadTestFile(t, b, input, "report.txt", []byte("new"))
	require.NotEqual(t, first.Path, second.Path)
	out := fileCall(b, input, "file_read", proto.AgentFileOperation{Path: first.Path})
	var got []byte
	for {
		require.Equal(t, "ok", out.Status)
		got = append(got, out.FileData...)
		if out.FileDone {
			break
		}
		out = fileCall(b, input, "file_read", proto.AgentFileOperation{TransferID: out.TransferID, Offset: out.FileOffset})
	}
	require.Equal(t, payload, got)
	listing := fileCall(b, input, "file_list", proto.AgentFileOperation{Path: filepath.ToSlash(filepath.Dir(first.Path))})
	require.Equal(t, "ok", listing.Status)
	require.Len(t, listing.Files, 2)
	empty := uploadTestFile(t, b, input, "empty", nil)
	out = fileCall(b, input, "file_read", proto.AgentFileOperation{Path: empty.Path})
	require.True(t, out.FileDone)
	require.Empty(t, out.FileData)
}
func TestFilesRejectTraversalAndForeignScopes(t *testing.T) {
	b, input := fileFixture(t)
	outside := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("private"), 0600))
	require.NoError(t, os.Symlink(outside, filepath.Join(input.ProjectRoot, "escape")))
	for _, p := range []string{"../secret", "/etc/passwd", "escape/secret", "..\\secret"} {
		t.Run(p, func(t *testing.T) {
			out := fileCall(b, input, "file_read", proto.AgentFileOperation{Path: p})
			require.NotEqual(t, "ok", out.Status)
			require.Empty(t, out.FileData)
		})
	}
	f := uploadTestFile(t, b, input, "hello", []byte("owned"))
	foreign := input
	foreign.ActorID = "other"
	require.Equal(t, "not_found", fileCall(b, foreign, "file_read", proto.AgentFileOperation{Path: f.Path}).Status)
	foreign = input
	foreign.Request.AccessID = strings.Repeat("f", 32)
	require.Equal(t, "not_found", fileCall(b, foreign, "file_read", proto.AgentFileOperation{Path: f.Path}).Status)
	foreign = input
	foreign.Request.SessionID = strings.Repeat("e", 32)
	require.Equal(t, "not_found", fileCall(b, foreign, "file_read", proto.AgentFileOperation{Path: f.Path}).Status)
}
func TestIncompleteUploadsAreNotCommittedAndExpire(t *testing.T) {
	b, input := fileFixture(t)
	begin := func() string {
		r := fileCall(b, input, "file_begin", proto.AgentFileOperation{Name: "partial", Size: 10})
		require.Equal(t, "ok", r.Status)
		return r.TransferID
	}
	id := begin()
	partial := filepath.ToSlash(filepath.Join(".liaison-attachments", input.Request.SessionID, ".upload-"+id))
	require.Equal(t, "invalid_request", fileCall(b, input, "file_read", proto.AgentFileOperation{Path: partial}).Status)
	require.Equal(t, "invalid_request", fileCall(b, input, "file_commit", proto.AgentFileOperation{TransferID: id}).Status)
	id = begin()
	require.Equal(t, "ok", fileCall(b, input, "file_cancel", proto.AgentFileOperation{TransferID: id}).Status)
	id = begin()
	require.Equal(t, "invalid_request", fileCall(b, input, "file_write", proto.AgentFileOperation{TransferID: id, Offset: 2, Data: []byte("x")}).Status)
	id = begin()
	b.expireFiles(time.Now().Add(3*time.Minute), false)
	require.Equal(t, "not_found", fileCall(b, input, "file_commit", proto.AgentFileOperation{TransferID: id}).Status)
	entries, err := os.ReadDir(filepath.Join(input.ProjectRoot, ".liaison-attachments", input.Request.SessionID))
	require.NoError(t, err)
	require.Empty(t, entries)
}
func TestImageInputsArePrivateSnapshots(t *testing.T) {
	b, input := fileFixture(t)
	var encoded bytes.Buffer
	require.NoError(t, png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))))
	f := uploadTestFile(t, b, input, "image.png", encoded.Bytes())
	s := b.sessions[input.Request.SessionID]
	display, inputs, temp, err := b.prepareAttachments(context.Background(), s, []string{f.Path})
	require.NoError(t, err)
	defer removeImageInputs(temp)
	require.Equal(t, "image/png", display[0].MediaType)
	require.NotEqual(t, inputs[0].Path, inputs[0].ImagePath)
	require.NoError(t, os.WriteFile(inputs[0].Path, []byte("changed"), 0600))
	actual, err := os.ReadFile(inputs[0].ImagePath)
	require.NoError(t, err)
	require.Equal(t, encoded.Bytes(), actual)
	_, _, _, err = b.prepareAttachments(context.Background(), s, []string{f.Path})
	require.Error(t, err)
	_, _, _, err = b.prepareAttachments(context.Background(), s, []string{"README.md"})
	require.Error(t, err)
}

func TestImageUploadRejectsInvalidContentBeforePublishing(t *testing.T) {
	b, input := fileFixture(t)
	require.Equal(t, "invalid_request", fileCall(b, input, "file_begin", proto.AgentFileOperation{Name: "large.png", Size: (4 << 20) + 1}).Status)
	data := []byte("not an image")
	begin := fileCall(b, input, "file_begin", proto.AgentFileOperation{Name: "invalid.png", Size: int64(len(data))})
	require.Equal(t, "ok", begin.Status)
	require.Equal(t, "ok", fileCall(b, input, "file_write", proto.AgentFileOperation{TransferID: begin.TransferID, Data: data}).Status)
	require.Equal(t, "invalid_request", fileCall(b, input, "file_commit", proto.AgentFileOperation{TransferID: begin.TransferID}).Status)
	entries, err := os.ReadDir(filepath.Join(input.ProjectRoot, ".liaison-attachments", input.Request.SessionID))
	require.NoError(t, err)
	require.Empty(t, entries)
}
