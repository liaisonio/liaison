package controlplane

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

type WebSSHHostKey struct {
	Trusted           bool   `json:"trusted"`
	Algorithm         string `json:"algorithm,omitempty"`
	FingerprintSHA256 string `json:"fingerprint_sha256,omitempty"`
}

type WebSSHCredential struct {
	Saved      bool   `json:"saved"`
	Username   string `json:"username,omitempty"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

type WebSSHCredentialSecret struct {
	Username          string
	EncryptedPassword string
	Nonce             string
}

type WebSSHTarget struct {
	ProxyID                uint                `json:"proxy_id"`
	ProxyName              string              `json:"proxy_name"`
	ApplicationID          uint                `json:"application_id"`
	ApplicationName        string              `json:"application_name"`
	TargetHost             string              `json:"target_host"`
	TargetPort             int                 `json:"target_port"`
	EffectiveStatus        string              `json:"effective_status"`
	EffectiveStatusMessage string              `json:"effective_status_message,omitempty"`
	HostKey                *WebSSHHostKey      `json:"host_key,omitempty"`
	Credentials            []*WebSSHCredential `json:"credentials,omitempty"`

	edgeID uint64
}

func (cp *controlPlane) GetWebSSHTarget(ctx context.Context, proxyID uint) (*WebSSHTarget, error) {
	target, err := cp.loadWebSSHTarget(proxyID)
	if err != nil {
		return nil, err
	}
	userID, ok := webSSHUserIDFromContext(ctx)
	if !ok {
		return target, nil
	}
	credentials, err := cp.loadWebSSHCredentials(proxyID, userID)
	if err != nil {
		return nil, err
	}
	target.Credentials = credentials
	return target, nil
}

func (cp *controlPlane) OpenWebSSHStream(ctx context.Context, proxyID uint) (net.Conn, *WebSSHTarget, error) {
	target, err := cp.loadWebSSHTarget(proxyID)
	if err != nil {
		return nil, nil, err
	}
	if target.EffectiveStatus != proxyEffectiveStatusActive {
		return nil, target, errors.New(target.EffectiveStatusMessage)
	}
	if cp.frontierBound == nil {
		return nil, target, errors.New("连接器通道未初始化")
	}
	stream, err := cp.frontierBound.OpenStream(ctx, target.edgeID)
	if err != nil {
		return nil, target, fmt.Errorf("连接器通道打开失败: %w", err)
	}
	dst := proto.Dst{
		Addr:          net.JoinHostPort(target.TargetHost, fmt.Sprintf("%d", target.TargetPort)),
		ApplicationID: target.ApplicationID,
		ProxyID:       target.ProxyID,
	}
	data, err := json.Marshal(dst)
	if err != nil {
		_ = stream.Close()
		return nil, target, err
	}
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	if _, err := stream.Write(lengthBuf); err != nil {
		_ = stream.Close()
		return nil, target, err
	}
	if _, err := stream.Write(data); err != nil {
		_ = stream.Close()
		return nil, target, err
	}
	return stream, target, nil
}

func (cp *controlPlane) TrustWebSSHHostKey(_ context.Context, proxyID uint, algorithm, fingerprintSHA256, publicKey string) error {
	if proxyID == 0 {
		return errors.New("访问 ID 不能为空")
	}
	if strings.TrimSpace(algorithm) == "" || strings.TrimSpace(fingerprintSHA256) == "" || strings.TrimSpace(publicKey) == "" {
		return errors.New("主机指纹信息不能为空")
	}
	if _, err := cp.repo.GetProxyByID(proxyID); err != nil {
		return err
	}
	return cp.repo.UpsertWebSSHHostKey(&model.WebSSHHostKey{
		ProxyID:           proxyID,
		Algorithm:         algorithm,
		FingerprintSHA256: fingerprintSHA256,
		PublicKey:         publicKey,
	})
}

func (cp *controlPlane) DeleteWebSSHHostKey(_ context.Context, proxyID uint) error {
	if proxyID == 0 {
		return errors.New("访问 ID 不能为空")
	}
	if _, err := cp.repo.GetProxyByID(proxyID); err != nil {
		return err
	}
	return cp.repo.DeleteWebSSHHostKeyByProxyID(proxyID)
}

func (cp *controlPlane) GetWebSSHCredentials(ctx context.Context, proxyID uint) ([]*WebSSHCredential, error) {
	if err := cp.validateWebSSHProxy(proxyID); err != nil {
		return nil, err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	return cp.loadWebSSHCredentials(proxyID, userID)
}

func (cp *controlPlane) GetWebSSHCredentialSecret(ctx context.Context, proxyID uint, username string) (*WebSSHCredentialSecret, error) {
	if err := cp.validateWebSSHProxy(proxyID); err != nil {
		return nil, err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return nil, errors.New("SSH 用户名不能为空")
	}
	credential, err := cp.repo.GetWebSSHCredential(proxyID, userID, username)
	if err != nil {
		return nil, err
	}
	return &WebSSHCredentialSecret{
		Username:          credential.Username,
		EncryptedPassword: credential.EncryptedPassword,
		Nonce:             credential.Nonce,
	}, nil
}

func (cp *controlPlane) SaveWebSSHCredential(ctx context.Context, proxyID uint, username, encryptedPassword, nonce string) error {
	if err := cp.validateWebSSHProxy(proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" || strings.TrimSpace(encryptedPassword) == "" || strings.TrimSpace(nonce) == "" {
		return errors.New("WebSSH 凭据信息不能为空")
	}
	return cp.repo.UpsertWebSSHCredential(&model.WebSSHCredential{
		ProxyID:           proxyID,
		UserID:            userID,
		Username:          username,
		EncryptedPassword: encryptedPassword,
		Nonce:             nonce,
	})
}

func (cp *controlPlane) TouchWebSSHCredential(ctx context.Context, proxyID uint, username string) error {
	if proxyID == 0 {
		return errors.New("访问 ID 不能为空")
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("SSH 用户名不能为空")
	}
	return cp.repo.TouchWebSSHCredential(proxyID, userID, username)
}

func (cp *controlPlane) DeleteWebSSHCredential(ctx context.Context, proxyID uint, username string) error {
	if err := cp.validateWebSSHProxy(proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("SSH 用户名不能为空")
	}
	return cp.repo.DeleteWebSSHCredential(proxyID, userID, username)
}

func (cp *controlPlane) loadWebSSHTarget(proxyID uint) (*WebSSHTarget, error) {
	if proxyID == 0 {
		return nil, errors.New("访问 ID 不能为空")
	}
	proxy, err := cp.repo.GetProxyByID(proxyID)
	if err != nil {
		return nil, err
	}
	application, err := cp.repo.GetApplicationByID(proxy.ApplicationID)
	if err != nil {
		return nil, err
	}
	proxy.Application = application
	if application.ApplicationType != model.ApplicationTypeSSH {
		return nil, errors.New("仅 SSH 应用支持 WebSSH")
	}
	effectiveStatus, effectiveStatusMessage := cp.proxyEffectiveStatus(proxy, application)
	var edgeID uint64
	if len(application.EdgeIDs) > 0 {
		edgeID = uint64(application.EdgeIDs[0])
	}
	hostKey := &WebSSHHostKey{Trusted: false}
	if saved, err := cp.repo.GetWebSSHHostKeyByProxyID(proxy.ID); err == nil {
		hostKey = &WebSSHHostKey{
			Trusted:           true,
			Algorithm:         saved.Algorithm,
			FingerprintSHA256: saved.FingerprintSHA256,
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	return &WebSSHTarget{
		ProxyID:                proxy.ID,
		ProxyName:              proxy.Name,
		ApplicationID:          application.ID,
		ApplicationName:        application.Name,
		TargetHost:             application.IP,
		TargetPort:             application.Port,
		EffectiveStatus:        effectiveStatus,
		EffectiveStatusMessage: effectiveStatusMessage,
		HostKey:                hostKey,
		edgeID:                 edgeID,
	}, nil
}

func (cp *controlPlane) validateWebSSHProxy(proxyID uint) error {
	_, err := cp.loadWebSSHTarget(proxyID)
	return err
}

func (cp *controlPlane) loadWebSSHCredentials(proxyID, userID uint) ([]*WebSSHCredential, error) {
	saved, err := cp.repo.ListWebSSHCredentialsByProxyAndUser(proxyID, userID)
	if err != nil {
		return nil, err
	}
	credentials := make([]*WebSSHCredential, 0, len(saved))
	for _, item := range saved {
		credential := &WebSSHCredential{
			Saved:    true,
			Username: item.Username,
		}
		if item.LastUsedAt != nil {
			credential.LastUsedAt = item.LastUsedAt.Format(time.RFC3339)
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

func webSSHUserIDFromContext(ctx context.Context) (uint, bool) {
	if ctx == nil {
		return 0, false
	}
	switch v := ctx.Value("user_id").(type) {
	case uint:
		return v, v > 0
	case uint64:
		if v > 0 {
			return uint(v), true
		}
	case int:
		if v > 0 {
			return uint(v), true
		}
	case int64:
		if v > 0 {
			return uint(v), true
		}
	}
	return 0, false
}

func requireWebSSHUserID(ctx context.Context) (uint, error) {
	userID, ok := webSSHUserIDFromContext(ctx)
	if !ok {
		return 0, errors.New("用户未认证")
	}
	return userID, nil
}
