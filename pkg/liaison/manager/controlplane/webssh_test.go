package controlplane

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

func TestWebSSHTargetRequiresSSHApplication(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	proxy := &model.Proxy{
		Name:          "proxy-1",
		ApplicationID: application.ID,
		Port:          39022,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	if _, err := cp.GetWebSSHTarget(context.Background(), proxy.ID); err == nil || !strings.Contains(err.Error(), "SSH") {
		t.Fatalf("GetWebSSHTarget error = %v, want SSH-only error", err)
	}
}

func TestWebSSHCredentialsAreScopedByUserAndUsername(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeSSH
	application.Port = 22
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "ssh-proxy",
		ApplicationID: application.ID,
		Port:          39022,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	ctxUser1 := context.WithValue(context.Background(), "user_id", uint(1))
	ctxUser2 := context.WithValue(context.Background(), "user_id", uint(2))
	if err := cp.SaveWebSSHCredential(ctxUser1, proxy.ID, "root", "enc-user1-root", "nonce-1"); err != nil {
		t.Fatalf("save user1 root credential: %v", err)
	}
	if err := cp.SaveWebSSHCredential(ctxUser1, proxy.ID, "admin", "enc-user1-admin", "nonce-2"); err != nil {
		t.Fatalf("save user1 admin credential: %v", err)
	}
	if err := cp.SaveWebSSHCredential(ctxUser2, proxy.ID, "root", "enc-user2-root", "nonce-3"); err != nil {
		t.Fatalf("save user2 root credential: %v", err)
	}

	user1Credentials, err := cp.GetWebSSHCredentials(ctxUser1, proxy.ID)
	if err != nil {
		t.Fatalf("GetWebSSHCredentials user1: %v", err)
	}
	if len(user1Credentials) != 2 {
		t.Fatalf("user1 credential count = %d, want 2", len(user1Credentials))
	}
	user2Credentials, err := cp.GetWebSSHCredentials(ctxUser2, proxy.ID)
	if err != nil {
		t.Fatalf("GetWebSSHCredentials user2: %v", err)
	}
	if len(user2Credentials) != 1 || user2Credentials[0].Username != "root" {
		t.Fatalf("user2 credentials = %+v, want only root", user2Credentials)
	}

	user1Root, err := cp.GetWebSSHCredentialSecret(ctxUser1, proxy.ID, "root")
	if err != nil {
		t.Fatalf("GetWebSSHCredentialSecret user1 root: %v", err)
	}
	if user1Root.EncryptedPassword != "enc-user1-root" {
		t.Fatalf("user1 root encrypted password = %q", user1Root.EncryptedPassword)
	}
	user2Root, err := cp.GetWebSSHCredentialSecret(ctxUser2, proxy.ID, "root")
	if err != nil {
		t.Fatalf("GetWebSSHCredentialSecret user2 root: %v", err)
	}
	if user2Root.EncryptedPassword != "enc-user2-root" {
		t.Fatalf("user2 root encrypted password = %q", user2Root.EncryptedPassword)
	}

	if err := cp.DeleteWebSSHCredential(ctxUser1, proxy.ID, " "); err == nil {
		t.Fatal("DeleteWebSSHCredential with empty username succeeded")
	}
	user1Credentials, err = cp.GetWebSSHCredentials(ctxUser1, proxy.ID)
	if err != nil {
		t.Fatalf("GetWebSSHCredentials user1 after empty delete: %v", err)
	}
	if len(user1Credentials) != 2 {
		t.Fatalf("user1 credential count after empty delete = %d, want 2", len(user1Credentials))
	}

	if err := cp.DeleteWebSSHCredential(ctxUser1, proxy.ID, "root"); err != nil {
		t.Fatalf("DeleteWebSSHCredential user1 root: %v", err)
	}
	if _, err := cp.GetWebSSHCredentialSecret(ctxUser1, proxy.ID, "root"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("user1 root after delete error = %v, want record not found", err)
	}
	if _, err := cp.GetWebSSHCredentialSecret(ctxUser2, proxy.ID, "root"); err != nil {
		t.Fatalf("user2 root should remain after user1 delete: %v", err)
	}
}

func TestWebSSHTargetReportsStatusAndHostKey(t *testing.T) {
	cp, r := newTestControlPlane(t)
	defer r.Close()
	_, application := createTestEdgeApplication(t, r)
	application.ApplicationType = model.ApplicationTypeSSH
	application.Port = 22
	if err := r.UpdateApplication(application); err != nil {
		t.Fatalf("update application: %v", err)
	}
	proxy := &model.Proxy{
		Name:          "ssh-proxy",
		ApplicationID: application.ID,
		Port:          39022,
		Status:        model.ProxyStatusRunning,
	}
	if err := r.CreateProxy(proxy); err != nil {
		t.Fatalf("create proxy: %v", err)
	}

	target, err := cp.GetWebSSHTarget(context.Background(), proxy.ID)
	if err != nil {
		t.Fatalf("GetWebSSHTarget: %v", err)
	}
	if target.EffectiveStatus != proxyEffectiveStatusActive {
		t.Fatalf("effective status = %q, want %q", target.EffectiveStatus, proxyEffectiveStatusActive)
	}
	if target.HostKey == nil || target.HostKey.Trusted {
		t.Fatalf("host key = %+v, want untrusted", target.HostKey)
	}

	if err := cp.TrustWebSSHHostKey(context.Background(), proxy.ID, "ssh-ed25519", "SHA256:test", "public-key"); err != nil {
		t.Fatalf("TrustWebSSHHostKey: %v", err)
	}
	target, err = cp.GetWebSSHTarget(context.Background(), proxy.ID)
	if err != nil {
		t.Fatalf("GetWebSSHTarget after trust: %v", err)
	}
	if target.HostKey == nil || !target.HostKey.Trusted || target.HostKey.FingerprintSHA256 != "SHA256:test" {
		t.Fatalf("host key = %+v, want trusted SHA256:test", target.HostKey)
	}

	if err := cp.DeleteWebSSHHostKey(context.Background(), proxy.ID); err != nil {
		t.Fatalf("DeleteWebSSHHostKey: %v", err)
	}
	target, err = cp.GetWebSSHTarget(context.Background(), proxy.ID)
	if err != nil {
		t.Fatalf("GetWebSSHTarget after delete: %v", err)
	}
	if target.HostKey == nil || target.HostKey.Trusted {
		t.Fatalf("host key = %+v, want untrusted after reset", target.HostKey)
	}
}
