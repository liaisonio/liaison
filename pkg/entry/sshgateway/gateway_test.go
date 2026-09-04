package sshgateway

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/proto"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

type testError string

func (e testError) Error() string { return string(e) }

const errTestAuth = testError("unknown key")

type testTargetProvider struct {
	targetSigner ssh.Signer
	clientKey    ssh.PublicKey
	mu           sync.Mutex
	trusted      string
	actions      []string
	statements   []string
}

func (p *testTargetProvider) OpenWebSSHStream(context.Context, uint) (net.Conn, *controlplane.WebSSHTarget, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, err
	}
	go func() {
		defer listener.Close()
		server, acceptErr := listener.Accept()
		if acceptErr == nil {
			serveTestTarget(server, p.targetSigner, p.clientKey)
		}
	}()
	client, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		_ = listener.Close()
		return nil, nil, err
	}
	return client, &controlplane.WebSSHTarget{
		ProxyID: 1, ProxyName: "native-ssh", ApplicationID: 2, ApplicationName: "target",
		TargetHost: "127.0.0.1", TargetPort: 22, HostKey: &controlplane.WebSSHHostKey{Trusted: p.trusted != "", FingerprintSHA256: p.trusted},
	}, nil
}

func (p *testTargetProvider) TrustWebSSHHostKey(_ context.Context, _ uint, _ string, fingerprint, _ string) error {
	p.mu.Lock()
	p.trusted = fingerprint
	p.mu.Unlock()
	return nil
}

func (p *testTargetProvider) RecordWebDataAudit(_ context.Context, audit *controlplane.WebDataAudit) error {
	p.mu.Lock()
	p.actions = append(p.actions, audit.Action)
	if audit.StatementPreview != "" {
		p.statements = append(p.statements, audit.StatementPreview)
	}
	p.mu.Unlock()
	return nil
}

func TestShellCommandCollectorNormalizesInputAndSuppressesSecrets(t *testing.T) {
	collector := &shellCommandCollector{}
	var commands []string
	emit := func(command string) { commands = append(commands, command) }
	collector.feed([]byte("ignored\n"), emit)
	collector.enable()
	collector.feed([]byte("upx\x7ftime\r\n"), emit)
	collector.feed([]byte("echo foo\x1b[Dbar\n"), emit)
	collector.observeOutput([]byte("[sudo] pass"))
	collector.observeOutput([]byte("word for root: "))
	collector.feed([]byte("do-not-store\n"), emit)
	collector.feed([]byte("export API_TOKEN=value\n"), emit)

	want := []string{"uptime", "echo foobar", "[SENSITIVE COMMAND REDACTED]"}
	if len(commands) != len(want) {
		t.Fatalf("commands = %q, want %q", commands, want)
	}
	for i := range want {
		if commands[i] != want[i] {
			t.Fatalf("commands[%d] = %q, want %q", i, commands[i], want[i])
		}
	}
}

func TestGatewayPasswordAuditsInteractiveShellCommands(t *testing.T) {
	provider := &testTargetProvider{targetSigner: newTestSigner(t)}
	gateway, err := New(filepath.Join(t.TempDir(), "ssh_host_ed25519_key"), provider, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	definition := &proto.Proxy{ID: 1, Name: "native-ssh", ApplicationID: 2, AccessProtocol: "ssh"}
	if err := gateway.CreateProxy(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", testPort(definition.ProxyPort)), &ssh.ClientConfig{
		User: "root", Auth: []ssh.AuthMethod{ssh.Password("target-secret")}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	stdin, err := session.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := session.RequestPty("xterm", 24, 80, ssh.TerminalModes{}); err != nil {
		t.Fatal(err)
	}
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	if _, err := stdin.Write([]byte("uptime\n")); err != nil {
		t.Fatal(err)
	}
	_ = stdin.Close()
	_ = session.Wait()

	provider.mu.Lock()
	defer provider.mu.Unlock()
	if !contains(provider.statements, "uptime") {
		t.Fatalf("audit statements = %q, want interactive command", provider.statements)
	}
}

func TestGatewayPasswordBridgesExecAndRejectsBypassChannels(t *testing.T) {
	targetSigner := newTestSigner(t)
	provider := &testTargetProvider{targetSigner: targetSigner}
	gateway, err := New(filepath.Join(t.TempDir(), "ssh_host_ed25519_key"), provider, time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	definition := &proto.Proxy{ID: 1, Name: "native-ssh", ProxyPort: 0, ApplicationID: 2, AccessProtocol: "ssh"}
	if err := gateway.CreateProxy(context.Background(), definition); err != nil {
		t.Fatalf("CreateProxy() error = %v", err)
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", testPort(definition.ProxyPort)), &ssh.ClientConfig{
		User:            "root",
		Auth:            []ssh.AuthMethod{ssh.Password("target-secret")},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // isolated in-memory test gateway
		Timeout:         5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ssh.Dial() error = %v", err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	output, err := session.CombinedOutput("printf integration-ok")
	if err != nil {
		t.Fatalf("CombinedOutput() error = %v, output = %q", err, output)
	}
	if strings.TrimSpace(string(output)) != "integration-ok" {
		t.Fatalf("CombinedOutput() = %q", output)
	}

	if _, _, err := client.OpenChannel("direct-tcpip", nil); err == nil {
		t.Fatal("direct-tcpip channel was accepted")
	}
	sftp, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession(sftp) error = %v", err)
	}
	if err := sftp.RequestSubsystem("sftp"); err == nil {
		t.Fatal("sftp subsystem was accepted")
	}
	_ = sftp.Close()
	scp, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession(scp) error = %v", err)
	}
	if err := scp.Run("scp -t /tmp/file"); err == nil {
		t.Fatal("scp exec was accepted")
	}
	_ = scp.Close()

	provider.mu.Lock()
	defer provider.mu.Unlock()
	if provider.trusted != ssh.FingerprintSHA256(targetSigner.PublicKey()) {
		t.Fatalf("trusted fingerprint = %q", provider.trusted)
	}
	if !contains(provider.actions, "open_session") || !contains(provider.actions, "execute") {
		t.Fatalf("audit actions = %v", provider.actions)
	}
}

func TestGatewayAgentForwardsOnlyLoginKey(t *testing.T) {
	clientSigner, clientPrivateKey := newTestKey(t)
	otherSigner, otherPrivateKey := newTestKey(t)
	provider := &testTargetProvider{targetSigner: newTestSigner(t), clientKey: clientSigner.PublicKey()}
	gateway, err := New(filepath.Join(t.TempDir(), "ssh_host_ed25519_key"), provider, time.Minute, time.Hour)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	definition := &proto.Proxy{ID: 1, Name: "native-ssh", ApplicationID: 2, AccessProtocol: "ssh"}
	if err := gateway.CreateProxy(context.Background(), definition); err != nil {
		t.Fatalf("CreateProxy() error = %v", err)
	}

	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", testPort(definition.ProxyPort)), &ssh.ClientConfig{
		User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatalf("ssh.Dial() error = %v", err)
	}
	defer client.Close()
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: otherPrivateKey}); err != nil {
		t.Fatal(err)
	}
	if err := keyring.Add(agent.AddedKey{PrivateKey: clientPrivateKey}); err != nil {
		t.Fatal(err)
	}
	if err := agent.ForwardToAgent(client, keyring); err != nil {
		t.Fatalf("ForwardToAgent() error = %v", err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatalf("NewSession() error = %v", err)
	}
	if err := agent.RequestAgentForwarding(session); err != nil {
		t.Fatalf("RequestAgentForwarding() error = %v", err)
	}
	output, err := session.CombinedOutput("printf integration-ok")
	if err != nil || strings.TrimSpace(string(output)) != "integration-ok" {
		t.Fatalf("CombinedOutput() error = %v, output = %q", err, output)
	}
	_ = otherSigner // the keyring intentionally contains an unrelated key first

	provider.mu.Lock()
	defer provider.mu.Unlock()
	if !contains(provider.actions, "open_session") || !contains(provider.actions, "execute") {
		t.Fatalf("audit actions = %v", provider.actions)
	}
}

func TestGatewayRejectsWrongTargetPasswordDuringHandshake(t *testing.T) {
	provider := &testTargetProvider{targetSigner: newTestSigner(t)}
	gateway, err := New(filepath.Join(t.TempDir(), "ssh_host_ed25519_key"), provider, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	definition := &proto.Proxy{ID: 1, Name: "native-ssh", ApplicationID: 2, AccessProtocol: "ssh"}
	if err := gateway.CreateProxy(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", testPort(definition.ProxyPort)), &ssh.ClientConfig{
		User: "root", Auth: []ssh.AuthMethod{ssh.Password("wrong-password")}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
	})
	if err == nil {
		_ = client.Close()
		t.Fatal("SSH handshake unexpectedly accepted a password rejected by the target")
	}
	provider.mu.Lock()
	defer provider.mu.Unlock()
	if !contains(provider.actions, "open_session") {
		t.Fatalf("failed authentication audit actions = %v", provider.actions)
	}
}

func TestGatewayPublicKeyRequiresMatchingForwardedAgent(t *testing.T) {
	clientSigner, _ := newTestKey(t)
	_, otherPrivateKey := newTestKey(t)
	provider := &testTargetProvider{targetSigner: newTestSigner(t), clientKey: clientSigner.PublicKey()}
	gateway, err := New(filepath.Join(t.TempDir(), "ssh_host_ed25519_key"), provider, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	definition := &proto.Proxy{ID: 1, Name: "native-ssh", ApplicationID: 2, AccessProtocol: "ssh"}
	if err := gateway.CreateProxy(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", testPort(definition.ProxyPort)), &ssh.ClientConfig{
		User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	keyring := agent.NewKeyring()
	if err := keyring.Add(agent.AddedKey{PrivateKey: otherPrivateKey}); err != nil {
		t.Fatal(err)
	}
	if err := agent.ForwardToAgent(client, keyring); err != nil {
		t.Fatal(err)
	}
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	if err := agent.RequestAgentForwarding(session); err == nil {
		t.Fatal("agent forwarding unexpectedly succeeded without the login key")
	}
}

func TestGatewayPublicKeyWithoutAgentFailsSession(t *testing.T) {
	clientSigner := newTestSigner(t)
	provider := &testTargetProvider{targetSigner: newTestSigner(t), clientKey: clientSigner.PublicKey()}
	gateway, err := New(filepath.Join(t.TempDir(), "ssh_host_ed25519_key"), provider, time.Minute, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = gateway.Close() })
	definition := &proto.Proxy{ID: 1, Name: "native-ssh", ApplicationID: 2, AccessProtocol: "ssh"}
	if err := gateway.CreateProxy(context.Background(), definition); err != nil {
		t.Fatal(err)
	}
	client, err := ssh.Dial("tcp", net.JoinHostPort("127.0.0.1", testPort(definition.ProxyPort)), &ssh.ClientConfig{
		User: "root", Auth: []ssh.AuthMethod{ssh.PublicKeys(clientSigner)}, HostKeyCallback: ssh.InsecureIgnoreHostKey(), Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.CombinedOutput("true")
	if err == nil {
		t.Fatalf("CombinedOutput() error = %v, output = %q", err, output)
	}
}

func serveTestTarget(raw net.Conn, hostKey ssh.Signer, acceptedKey ssh.PublicKey) {
	config := &ssh.ServerConfig{PasswordCallback: func(_ ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if string(password) != "target-secret" {
			return nil, errTestAuth
		}
		return nil, nil
	}, PublicKeyCallback: func(_ ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
		if acceptedKey == nil || ssh.FingerprintSHA256(key) != ssh.FingerprintSHA256(acceptedKey) {
			return nil, errTestAuth
		}
		return nil, nil
	}}
	config.AddHostKey(hostKey)
	connection, channels, requests, err := ssh.NewServerConn(raw, config)
	if err != nil {
		_ = raw.Close()
		return
	}
	defer connection.Close()
	go ssh.DiscardRequests(requests)
	for channel := range channels {
		if channel.ChannelType() != "session" {
			_ = channel.Reject(ssh.UnknownChannelType, "unsupported")
			continue
		}
		stream, streamRequests, err := channel.Accept()
		if err != nil {
			continue
		}
		go func() {
			defer stream.Close()
			for request := range streamRequests {
				switch request.Type {
				case "pty-req":
					if request.WantReply {
						_ = request.Reply(true, nil)
					}
					continue
				case "shell":
					if request.WantReply {
						_ = request.Reply(true, nil)
					}
					buffer := make([]byte, 256)
					_, _ = stream.Read(buffer)
					_, _ = stream.Write([]byte("integration-ok\n"))
					_, _ = stream.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
					return
				case "exec":
				default:
					if request.WantReply {
						_ = request.Reply(false, nil)
					}
					continue
				}
				var payload struct{ Command string }
				if ssh.Unmarshal(request.Payload, &payload) != nil {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				time.Sleep(20 * time.Millisecond)
				_, _ = stream.Write([]byte("integration-ok\n"))
				_, _ = stream.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				return
			}
		}()
	}
}

func newTestSigner(t *testing.T) ssh.Signer {
	t.Helper()
	signer, _ := newTestKey(t)
	return signer
}

func newTestKey(t *testing.T) (ssh.Signer, ed25519.PrivateKey) {
	t.Helper()
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(privateKey)
	if err != nil {
		t.Fatal(err)
	}
	return signer, privateKey
}

func testPort(port int) string { return fmt.Sprintf("%d", port) }

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
