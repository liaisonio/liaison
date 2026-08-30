package sshgateway

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

func loadOrCreateHostKey(path string) (ssh.Signer, error) {
	if path == "" {
		return nil, fmt.Errorf("SSH host key path is empty")
	}
	data, err := os.ReadFile(path)
	if err == nil {
		return ssh.ParsePrivateKey(data)
	}
	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read SSH host key: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, fmt.Errorf("create SSH host key directory: %w", err)
	}
	_, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate SSH host key: %w", err)
	}
	block, err := ssh.MarshalPrivateKey(privateKey, "liaison SSH gateway")
	if err != nil {
		return nil, fmt.Errorf("marshal SSH host key: %w", err)
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".ssh-host-key-*")
	if err != nil {
		return nil, fmt.Errorf("create temporary SSH host key: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(0600); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := pem.Encode(tmp, block); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}
	if err := os.Rename(tmpName, path); err != nil {
		if data, readErr := os.ReadFile(path); readErr == nil {
			return ssh.ParsePrivateKey(data)
		}
		return nil, fmt.Errorf("persist SSH host key: %w", err)
	}
	return ssh.NewSignerFromKey(privateKey)
}
