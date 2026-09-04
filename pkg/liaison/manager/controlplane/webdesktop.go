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
	"github.com/liaisonio/liaison/pkg/trafficconn"
)

type WebDesktopCredential struct {
	ID         uint   `json:"id"`
	Saved      bool   `json:"saved"`
	Protocol   string `json:"protocol"`
	Username   string `json:"username,omitempty"`
	Domain     string `json:"domain,omitempty"`
	LastUsedAt string `json:"last_used_at,omitempty"`
}

type WebDesktopCredentialSecret struct {
	Protocol          string
	Username          string
	Domain            string
	EncryptedPassword string
	Nonce             string
}

type WebDesktopTarget struct {
	ProxyID                uint                    `json:"proxy_id"`
	ProxyName              string                  `json:"proxy_name"`
	ApplicationID          uint                    `json:"application_id"`
	ApplicationName        string                  `json:"application_name"`
	Protocol               string                  `json:"protocol"`
	TargetHost             string                  `json:"target_host"`
	TargetPort             int                     `json:"target_port"`
	EffectiveStatus        string                  `json:"effective_status"`
	EffectiveStatusMessage string                  `json:"effective_status_message,omitempty"`
	Credentials            []*WebDesktopCredential `json:"credentials,omitempty"`

	edgeID uint64
}

func (cp *controlPlane) GetWebDesktopTarget(ctx context.Context, proxyID uint) (*WebDesktopTarget, error) {
	target, err := cp.loadWebDesktopTarget(ctx, proxyID)
	if err != nil {
		return nil, err
	}
	userID, ok := webSSHUserIDFromContext(ctx)
	if !ok {
		return target, nil
	}
	credentials, err := cp.loadWebDesktopCredentials(proxyID, userID, target.Protocol)
	if err != nil {
		return nil, err
	}
	target.Credentials = credentials
	return target, nil
}

func (cp *controlPlane) OpenWebDesktopStream(ctx context.Context, proxyID uint) (net.Conn, *WebDesktopTarget, error) {
	target, err := cp.loadWebDesktopTarget(ctx, proxyID)
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
	meteredStream := trafficconn.TargetConn(stream, cp.trafficRecorder, target.ProxyID, target.ApplicationID)
	dst := proto.Dst{
		Addr:          net.JoinHostPort(target.TargetHost, fmt.Sprintf("%d", target.TargetPort)),
		ApplicationID: target.ApplicationID,
		ProxyID:       target.ProxyID,
	}
	data, err := json.Marshal(dst)
	if err != nil {
		_ = meteredStream.Close()
		return nil, target, err
	}
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	if _, err := meteredStream.Write(lengthBuf); err != nil {
		_ = meteredStream.Close()
		return nil, target, err
	}
	if _, err := meteredStream.Write(data); err != nil {
		_ = meteredStream.Close()
		return nil, target, err
	}
	return meteredStream, target, nil
}

func (cp *controlPlane) GetWebDesktopCredentialSecret(ctx context.Context, proxyID uint, protocol, username, domain string) (*WebDesktopCredentialSecret, error) {
	if err := cp.validateWebDesktopProxy(ctx, proxyID); err != nil {
		return nil, err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	protocol, username, domain = normalizeWebDesktopCredentialIdentity(protocol, username, domain)
	credential, err := cp.repo.GetWebDesktopCredential(proxyID, userID, protocol, username, domain)
	if err != nil {
		return nil, err
	}
	return &WebDesktopCredentialSecret{
		Protocol:          credential.Protocol,
		Username:          credential.Username,
		Domain:            credential.Domain,
		EncryptedPassword: credential.EncryptedPassword,
		Nonce:             credential.Nonce,
	}, nil
}

func (cp *controlPlane) SaveWebDesktopCredential(ctx context.Context, proxyID uint, protocol, username, domain, encryptedPassword, nonce string) error {
	if err := cp.validateWebDesktopProxy(ctx, proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	protocol, username, domain = normalizeWebDesktopCredentialIdentity(protocol, username, domain)
	if !isWebDesktopProtocol(protocol) || strings.TrimSpace(encryptedPassword) == "" || strings.TrimSpace(nonce) == "" {
		return errors.New("WebDesktop 凭据信息不能为空")
	}
	return cp.repo.UpsertWebDesktopCredential(&model.WebDesktopCredential{
		ProxyID:           proxyID,
		UserID:            userID,
		Protocol:          protocol,
		Username:          username,
		Domain:            domain,
		EncryptedPassword: encryptedPassword,
		Nonce:             nonce,
	})
}

func (cp *controlPlane) TouchWebDesktopCredential(ctx context.Context, proxyID uint, protocol, username, domain string) error {
	if proxyID == 0 {
		return errors.New("访问 ID 不能为空")
	}
	if err := cp.validateWebDesktopProxy(ctx, proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	protocol, username, domain = normalizeWebDesktopCredentialIdentity(protocol, username, domain)
	return cp.repo.TouchWebDesktopCredential(proxyID, userID, protocol, username, domain)
}

func (cp *controlPlane) DeleteWebDesktopCredential(ctx context.Context, proxyID uint, protocol, username, domain string) error {
	if err := cp.validateWebDesktopProxy(ctx, proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	protocol, username, domain = normalizeWebDesktopCredentialIdentity(protocol, username, domain)
	return cp.repo.DeleteWebDesktopCredential(proxyID, userID, protocol, username, domain)
}

func (cp *controlPlane) loadWebDesktopTarget(ctx context.Context, proxyID uint) (*WebDesktopTarget, error) {
	if proxyID == 0 {
		return nil, errors.New("访问 ID 不能为空")
	}
	if err := requireVisibleResource(ctx, cp.repo, resourceAccess, uint64(proxyID)); err != nil {
		return nil, err
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
	protocol := strings.ToLower(string(application.ApplicationType))
	if !isWebDesktopProtocol(protocol) {
		return nil, errors.New("仅 RDP/VNC 应用支持 WebDesktop")
	}
	effectiveStatus, effectiveStatusMessage := cp.proxyEffectiveStatus(proxy, application)
	var edgeID uint64
	if len(application.EdgeIDs) > 0 {
		edgeID = uint64(application.EdgeIDs[0])
	}
	return &WebDesktopTarget{
		ProxyID:                proxy.ID,
		ProxyName:              proxy.Name,
		ApplicationID:          application.ID,
		ApplicationName:        application.Name,
		Protocol:               protocol,
		TargetHost:             application.IP,
		TargetPort:             application.Port,
		EffectiveStatus:        effectiveStatus,
		EffectiveStatusMessage: effectiveStatusMessage,
		edgeID:                 edgeID,
	}, nil
}

func (cp *controlPlane) validateWebDesktopProxy(ctx context.Context, proxyID uint) error {
	_, err := cp.loadWebDesktopTarget(ctx, proxyID)
	return err
}

func (cp *controlPlane) loadWebDesktopCredentials(proxyID, userID uint, protocol string) ([]*WebDesktopCredential, error) {
	saved, err := cp.repo.ListWebDesktopCredentialsByProxyAndUser(proxyID, userID, protocol)
	if err != nil {
		return nil, err
	}
	credentials := make([]*WebDesktopCredential, 0, len(saved))
	for _, item := range saved {
		credential := &WebDesktopCredential{
			ID:       item.ID,
			Saved:    true,
			Protocol: item.Protocol,
			Username: item.Username,
			Domain:   item.Domain,
		}
		if item.LastUsedAt != nil {
			credential.LastUsedAt = item.LastUsedAt.Format(time.RFC3339)
		}
		credentials = append(credentials, credential)
	}
	return credentials, nil
}

func isWebDesktopProtocol(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "rdp", "vnc":
		return true
	default:
		return false
	}
}

func normalizeWebDesktopCredentialIdentity(protocol, username, domain string) (string, string, string) {
	return strings.ToLower(strings.TrimSpace(protocol)), strings.TrimSpace(username), strings.TrimSpace(domain)
}
