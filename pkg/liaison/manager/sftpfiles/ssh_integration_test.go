package sftpfiles

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

// Exercises SSH negotiation and the SFTP subsystem over a real loopback socket.
// No deployed credentials, connector, or existing remote files are used.
func TestFilesSSHSubsystemRoundTrip(t *testing.T) {
	_, private, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)
	signer, err := ssh.NewSignerFromKey(private)
	require.NoError(t, err)
	config := &ssh.ServerConfig{PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if !bytes.Equal(key.Marshal(), signer.PublicKey().Marshal()) {
			return nil, errors.New("unknown fixture key")
		}
		return &ssh.Permissions{}, nil
	}}
	config.AddHostKey(signer)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		server, channels, requests, err := ssh.NewServerConn(conn, config)
		if err != nil {
			return
		}
		defer server.Close()
		go ssh.DiscardRequests(requests)
		for incoming := range channels {
			if incoming.ChannelType() != "session" {
				_ = incoming.Reject(ssh.UnknownChannelType, "unsupported fixture channel")
				continue
			}
			channel, requests, err := incoming.Accept()
			if err != nil {
				return
			}
			for req := range requests {
				var subsystem struct{ Name string }
				accepted := req.Type == "subsystem" && ssh.Unmarshal(req.Payload, &subsystem) == nil && subsystem.Name == "sftp"
				_ = req.Reply(accepted, nil) // client failure is handled by Serve/connection cleanup
				if !accepted {
					continue
				}
				s, err := sftp.NewServer(channel)
				if err == nil {
					_ = s.Serve()
					_ = s.Close()
				} // closing the client ends the fixture
				break
			}
			_ = channel.Close() // best-effort fixture cleanup
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close() // unblock Accept on setup failure
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("SSH fixture failed to stop")
		}
	})
	client, err := ssh.Dial("tcp", listener.Addr().String(), &ssh.ClientConfig{
		User: "fixture", Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)}, HostKeyCallback: ssh.FixedHostKey(signer.PublicKey()), Timeout: 5 * time.Second,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = client.Close() })
	f, err := New(context.Background(), client, true)
	require.NoError(t, err)
	t.Cleanup(func() { _ = f.Close() })
	require.True(t, f.CanUpload())
	p := filepath.Join(t.TempDir(), "transfer.txt")
	payload := bytes.Repeat([]byte("isolated transfer\n"), 4096)
	n, err := f.Upload(context.Background(), p, bytes.NewReader(payload), int64(len(payload)))
	require.NoError(t, err)
	require.EqualValues(t, len(payload), n)
	var downloaded bytes.Buffer
	n, err = f.Download(context.Background(), p, &downloaded)
	require.NoError(t, err)
	require.EqualValues(t, len(payload), n)
	require.Equal(t, payload, downloaded.Bytes())
}
