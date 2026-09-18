//go:build integration

package smbfiles

import (
	"context"
	"github.com/stretchr/testify/require"
	"net"
	"os"
	"testing"
	"time"
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
	_, err = f.Read(ctx, "/../hello.txt", TransferLimit)
	require.ErrorIs(t, err, ErrInvalid)
}
