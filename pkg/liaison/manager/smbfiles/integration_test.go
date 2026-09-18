//go:build integration

package smbfiles

import (
	"context"
	"net"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// The share must contain a UTF-8 file named hello.txt in an isolated fixture.
func TestSMB_RealShare(t *testing.T) {
	address := os.Getenv("TEST_SMB_ADDRESS")
	if address == "" {
		t.Skip("set TEST_SMB_ADDRESS for an isolated share")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
	require.NoError(t, err)
	host, _, err := net.SplitHostPort(address)
	require.NoError(t, err)
	f, err := New(ctx, conn, host, os.Getenv("TEST_SMB_USER"), os.Getenv("TEST_SMB_PASSWORD"), "", os.Getenv("TEST_SMB_SHARE"))
	require.NoError(t, err)
	defer f.Close()
	entries, err := f.List(ctx, "/")
	require.NoError(t, err)
	require.NotEmpty(t, entries)
	value, err := f.Preview(ctx, "/hello.txt")
	require.NoError(t, err)
	require.Contains(t, value, "Liaison")
	data, err := f.Read(ctx, "/hello.txt", TransferLimit)
	require.NoError(t, err)
	require.Equal(t, value, string(data))
	_, err = f.Read(ctx, "/hello.txt", 1)
	require.ErrorIs(t, err, ErrLimit)
	for _, path := range []string{"/../hello.txt", "//other/share/hello.txt", `C:\hello.txt`, "/hello.txt:stream"} {
		_, err = f.Read(ctx, path, TransferLimit)
		require.ErrorIs(t, err, ErrInvalid, path)
	}

	t.Run("wrong_password", func(t *testing.T) {
		conn, err := (&net.Dialer{}).DialContext(ctx, "tcp", address)
		require.NoError(t, err)
		// New owns and closes the connection even when authentication fails.
		invalid, err := New(ctx, conn, host, os.Getenv("TEST_SMB_USER"), os.Getenv("TEST_SMB_PASSWORD")+"-invalid", "", os.Getenv("TEST_SMB_SHARE"))
		if invalid != nil {
			require.NoError(t, invalid.Close())
		}
		require.ErrorIs(t, err, ErrRead)
	})
}
