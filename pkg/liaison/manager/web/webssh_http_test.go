package web

import (
	"bytes"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/config"
)

func TestValidateWebSSHSessionCredentials(t *testing.T) {
	tests := []struct {
		name    string
		req     createWebSSHSessionRequest
		wantErr string
	}{
		{
			name:    "username required",
			req:     createWebSSHSessionRequest{Password: "secret"},
			wantErr: "用户名",
		},
		{
			name:    "new connection requires password",
			req:     createWebSSHSessionRequest{Username: "root"},
			wantErr: "密码",
		},
		{
			name: "explicit saved credential may omit password",
			req: createWebSSHSessionRequest{
				Username:           "root",
				UseSavedCredential: true,
			},
		},
		{
			name: "new connection accepts entered password",
			req:  createWebSSHSessionRequest{Username: "root", Password: "secret"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWebSSHSessionCredentials(tt.req)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("validateWebSSHSessionCredentials() error = %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("validateWebSSHSessionCredentials() error = %v, want containing %q", err, tt.wantErr)
			}
		})
	}
}

func TestWebSSHCredentialEncryptDecrypt(t *testing.T) {
	key := deriveWebSSHCredentialKey(&config.Configuration{
		Manager: config.Manager{CredentialSecret: "test-webssh-credential-secret-32-bytes"},
	})
	webServer := &web{credentialKey: key}
	password := []byte("secret-password")

	encryptedPassword, nonce, err := webServer.encryptWebSSHPassword(password)
	if err != nil {
		t.Fatalf("encryptWebSSHPassword() error = %v", err)
	}
	if encryptedPassword == "" || nonce == "" {
		t.Fatal("encrypted password and nonce must be present")
	}
	if bytes.Contains([]byte(encryptedPassword), password) {
		t.Fatal("encrypted password should not contain plaintext")
	}

	decrypted, err := webServer.decryptWebSSHPassword(encryptedPassword, nonce)
	if err != nil {
		t.Fatalf("decryptWebSSHPassword() error = %v", err)
	}
	if string(decrypted) != string(password) {
		t.Fatalf("decrypted password = %q, want %q", decrypted, password)
	}
}

func TestWebSSHCredentialDecryptRejectsWrongKey(t *testing.T) {
	first := &web{credentialKey: deriveWebSSHCredentialKey(&config.Configuration{
		Manager: config.Manager{CredentialSecret: "first-webssh-credential-secret-32-bytes"},
	})}
	second := &web{credentialKey: deriveWebSSHCredentialKey(&config.Configuration{
		Manager: config.Manager{CredentialSecret: "second-webssh-credential-secret-32-bytes"},
	})}

	encryptedPassword, nonce, err := first.encryptWebSSHPassword([]byte("secret-password"))
	if err != nil {
		t.Fatalf("encryptWebSSHPassword() error = %v", err)
	}
	if _, err := second.decryptWebSSHPassword(encryptedPassword, nonce); err == nil {
		t.Fatal("decryptWebSSHPassword() with wrong key succeeded")
	}
}

func TestWebSSHCommandCollectorAuditsCompletedLines(t *testing.T) {
	var commands []string
	collector := &webSSHCommandCollector{}
	emit := func(command string) {
		commands = append(commands, command)
	}

	collector.feed("ec", emit)
	collector.feed("ho hello\r", emit)
	collector.feed("\x1b[Auname -a\n", emit)
	collector.feed("abcd\b\bXY\r", emit)

	want := []string{"echo hello", "uname -a", "abXY"}
	if len(commands) != len(want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	for i := range want {
		if commands[i] != want[i] {
			t.Fatalf("commands[%d] = %q, want %q", i, commands[i], want[i])
		}
	}
}

func TestWebSSHCommandCollectorSuppressesSensitiveInput(t *testing.T) {
	var commands []string
	collector := &webSSHCommandCollector{}
	emit := func(command string) {
		commands = append(commands, command)
	}

	collector.observeOutput("[sudo] password for root:")
	collector.feed("super-secret-password\r", emit)
	collector.feed("whoami\r", emit)
	collector.feed("mysql --password=plain-text\r", emit)

	want := []string{"whoami", "[SENSITIVE COMMAND REDACTED]"}
	if len(commands) != len(want) {
		t.Fatalf("commands = %#v, want %#v", commands, want)
	}
	for i := range want {
		if commands[i] != want[i] {
			t.Fatalf("commands[%d] = %q, want %q", i, commands[i], want[i])
		}
	}
}
