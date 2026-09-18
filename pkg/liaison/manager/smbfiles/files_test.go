package smbfiles

import (
	"context"
	"github.com/stretchr/testify/require"
	"net"
	"testing"
	"time"
)

func TestNormalize_RejectsEscapes(t *testing.T) {
	for _, value := range []string{"../secret", "/a/../b", "//host/share", "C:/file", "/a:stream", "/a\\b", "/a//b", "/a/./b", "/trailing.", "/trailing ", "/a\x00b", "/a?"} {
		t.Run(value, func(t *testing.T) { _, err := Normalize(value); require.ErrorIs(t, err, ErrInvalid) })
	}
	for input, want := range map[string]string{"/": ".", "": ".", "/目录/file.txt": "目录\\file.txt", "/a/": "a"} {
		got, err := Normalize(input)
		require.NoError(t, err)
		require.Equal(t, want, got)
	}
}
func TestShare_RejectsAlternateTargets(t *testing.T) {
	for _, value := range []string{"", "..", "//host/share", "share/sub", "share\\sub", "C:", "share\n"} {
		require.False(t, ValidShare(value))
	}
	require.True(t, ValidShare("Team documents"))
}
func TestNew_CanceledHandshakeClosesTunnel(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	_, err := New(ctx, client, "test.invalid", "user", "unused-test-value", "", "share")
	require.Error(t, err)
	require.Error(t, client.SetDeadline(time.Now()))
}
