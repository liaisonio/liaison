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

	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/liaisonio/liaison/pkg/trafficconn"
)

type WebDataCredential struct {
	ID               uint   `json:"id"`
	Saved            bool   `json:"saved"`
	Name             string `json:"name,omitempty"`
	Protocol         string `json:"protocol"`
	Username         string `json:"username,omitempty"`
	Database         string `json:"database,omitempty"`
	AuthDatabase     string `json:"auth_database,omitempty"`
	RedisDB          int    `json:"redis_db"`
	TLSMode          string `json:"tls_mode,omitempty"`
	Schema           string `json:"schema,omitempty"`
	AuthMechanism    string `json:"auth_mechanism,omitempty"`
	DirectConnection bool   `json:"direct_connection"`
	ConnectionParams string `json:"connection_params,omitempty"`
	LastUsedAt       string `json:"last_used_at,omitempty"`
}

type WebDataCredentialSecret struct {
	ID                uint
	Name              string
	Protocol          string
	Username          string
	Database          string
	AuthDatabase      string
	RedisDB           int
	TLSMode           string
	Schema            string
	AuthMechanism     string
	DirectConnection  bool
	ConnectionParams  string
	EncryptedPassword string
	Nonce             string
}

type WebDataCredentialProfile struct {
	ID                uint
	Name              string
	Protocol          string
	Username          string
	Database          string
	AuthDatabase      string
	RedisDB           int
	TLSMode           string
	Schema            string
	AuthMechanism     string
	DirectConnection  bool
	ConnectionParams  string
	EncryptedPassword string
	Nonce             string
	PasswordChanged   bool
}

type WebDataTarget struct {
	ProxyID                uint                 `json:"proxy_id"`
	ProxyName              string               `json:"proxy_name"`
	ApplicationID          uint                 `json:"application_id"`
	ApplicationName        string               `json:"application_name"`
	Protocol               string               `json:"protocol"`
	ApplicationType        string               `json:"application_type"`
	TargetHost             string               `json:"target_host"`
	TargetPort             int                  `json:"target_port"`
	EffectiveStatus        string               `json:"effective_status"`
	EffectiveStatusMessage string               `json:"effective_status_message,omitempty"`
	Credentials            []*WebDataCredential `json:"credentials,omitempty"`

	edgeID uint64
}

type WebDataAudit struct {
	UserID           uint
	UserEmail        string
	ProxyID          uint
	ProxyName        string
	ApplicationID    uint
	ApplicationName  string
	Protocol         string
	Action           string
	Database         string
	StatementPreview string
	StatementSHA256  string
	Success          bool
	AffectedRows     int64
	Error            string
	ElapsedMS        int64
	ClientIP         string
	Details          map[string]any
}

type WebDataAuditEntry struct {
	ID               uint           `json:"id"`
	UserID           uint           `json:"user_id"`
	UserEmail        string         `json:"user_email,omitempty"`
	ProxyID          uint           `json:"proxy_id"`
	ProxyName        string         `json:"proxy_name,omitempty"`
	ApplicationID    uint           `json:"application_id"`
	ApplicationName  string         `json:"application_name,omitempty"`
	Protocol         string         `json:"protocol"`
	Action           string         `json:"action"`
	StatementPreview string         `json:"statement_preview"`
	StatementSHA256  string         `json:"statement_sha256"`
	Success          bool           `json:"success"`
	Error            string         `json:"error,omitempty"`
	ElapsedMS        int64          `json:"elapsed_ms"`
	Details          map[string]any `json:"details,omitempty"`
	CreatedAt        string         `json:"created_at"`
}

type WebDataAuditListQuery struct {
	ProxyID   uint
	Protocol  string
	Action    string
	Success   *bool
	Keyword   string
	StartTime *time.Time
	EndTime   *time.Time
	Page      int
	PageSize  int
	Limit     int
}

type WebDataAuditList struct {
	Items    []*WebDataAuditEntry `json:"items"`
	Total    int64                `json:"total"`
	Page     int                  `json:"page"`
	PageSize int                  `json:"page_size"`
}

func (cp *controlPlane) GetWebDataTarget(ctx context.Context, proxyID uint) (*WebDataTarget, error) {
	target, err := cp.loadWebDataTarget(proxyID)
	if err != nil {
		return nil, err
	}
	userID, ok := webSSHUserIDFromContext(ctx)
	if !ok {
		return target, nil
	}
	credentials, err := cp.loadWebDataCredentials(proxyID, userID, target.Protocol)
	if err != nil {
		return nil, err
	}
	target.Credentials = credentials
	return target, nil
}

func (cp *controlPlane) OpenWebDataStream(ctx context.Context, proxyID uint) (net.Conn, *WebDataTarget, error) {
	target, err := cp.loadWebDataTarget(proxyID)
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

func (cp *controlPlane) GetWebDataCredentialSecret(ctx context.Context, proxyID uint, protocol, username, database, authDatabase string) (*WebDataCredentialSecret, error) {
	if err := cp.validateWebDataProxy(proxyID); err != nil {
		return nil, err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	protocol, username, database, authDatabase = normalizeWebDataCredentialIdentity(protocol, username, database, authDatabase)
	credential, err := cp.repo.GetWebDataCredential(proxyID, userID, protocol, username, database, authDatabase)
	if err != nil {
		return nil, err
	}
	return webDataCredentialSecretFromModel(credential), nil
}

func (cp *controlPlane) GetWebDataCredentialSecretByID(ctx context.Context, proxyID, credentialID uint) (*WebDataCredentialSecret, error) {
	if credentialID == 0 {
		return nil, errors.New("连接 ID 不能为空")
	}
	target, err := cp.loadWebDataTarget(proxyID)
	if err != nil {
		return nil, err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	credential, err := cp.repo.GetWebDataCredentialByID(credentialID, proxyID, userID)
	if err != nil {
		return nil, err
	}
	if normalizeWebDataProtocol(credential.Protocol) != target.Protocol {
		return nil, errors.New("保存连接的协议类型与应用类型不匹配")
	}
	return webDataCredentialSecretFromModel(credential), nil
}

func (cp *controlPlane) SaveWebDataCredential(ctx context.Context, proxyID uint, protocol, username, database, authDatabase, encryptedPassword, nonce string) error {
	_, err := cp.SaveWebDataCredentialProfile(ctx, proxyID, &WebDataCredentialProfile{
		Protocol:          protocol,
		Username:          username,
		Database:          database,
		AuthDatabase:      authDatabase,
		EncryptedPassword: encryptedPassword,
		Nonce:             nonce,
		PasswordChanged:   true,
	})
	return err
}

func (cp *controlPlane) SaveWebDataCredentialProfile(ctx context.Context, proxyID uint, profile *WebDataCredentialProfile) (*WebDataCredential, error) {
	target, err := cp.loadWebDataTarget(proxyID)
	if err != nil {
		return nil, err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	if profile == nil {
		return nil, errors.New("WebData 凭据信息不能为空")
	}
	protocol, username, database, authDatabase := normalizeWebDataCredentialIdentity(profile.Protocol, profile.Username, profile.Database, profile.AuthDatabase)
	if !isWebDataProtocol(protocol) {
		return nil, errors.New("WebData 协议类型不支持")
	}
	if protocol != target.Protocol {
		return nil, errors.New("保存连接的协议类型与应用类型不匹配")
	}
	updatePassword := profile.PasswordChanged || profile.ID == 0
	if updatePassword && (strings.TrimSpace(profile.EncryptedPassword) == "" || strings.TrimSpace(profile.Nonce) == "") {
		return nil, errors.New("WebData 凭据密码不能为空")
	}
	credential := &model.WebDataCredential{
		ProxyID:           proxyID,
		UserID:            userID,
		Name:              strings.TrimSpace(profile.Name),
		Protocol:          protocol,
		Username:          username,
		Database:          database,
		AuthDatabase:      authDatabase,
		RedisDB:           profile.RedisDB,
		TLSMode:           normalizeWebDataTLSMode(profile.TLSMode),
		Schema:            strings.TrimSpace(profile.Schema),
		AuthMechanism:     normalizeWebDataAuthMechanism(profile.AuthMechanism),
		DirectConnection:  profile.DirectConnection,
		ConnectionParams:  normalizeWebDataConnectionParams(profile.ConnectionParams),
		EncryptedPassword: profile.EncryptedPassword,
		Nonce:             profile.Nonce,
	}
	credential.ID = profile.ID
	if credential.Name == "" {
		credential.Name = defaultWebDataCredentialName(credential)
	}
	if credential.ID > 0 {
		if err := cp.repo.UpdateWebDataCredential(credential, updatePassword); err != nil {
			return nil, err
		}
		saved, err := cp.repo.GetWebDataCredentialByID(credential.ID, proxyID, userID)
		if err != nil {
			return nil, err
		}
		return webDataCredentialFromModel(saved), nil
	}
	if err := cp.repo.UpsertWebDataCredential(credential); err != nil {
		return nil, err
	}
	saved, err := cp.repo.GetWebDataCredential(proxyID, userID, credential.Protocol, credential.Username, credential.Database, credential.AuthDatabase)
	if err != nil {
		return nil, err
	}
	return webDataCredentialFromModel(saved), nil
}

func (cp *controlPlane) TouchWebDataCredential(ctx context.Context, proxyID uint, protocol, username, database, authDatabase string) error {
	if proxyID == 0 {
		return errors.New("访问 ID 不能为空")
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	protocol, username, database, authDatabase = normalizeWebDataCredentialIdentity(protocol, username, database, authDatabase)
	return cp.repo.TouchWebDataCredential(proxyID, userID, protocol, username, database, authDatabase)
}

func (cp *controlPlane) TouchWebDataCredentialByID(ctx context.Context, proxyID, credentialID uint) error {
	if proxyID == 0 || credentialID == 0 {
		return errors.New("访问 ID 和连接 ID 不能为空")
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	return cp.repo.TouchWebDataCredentialByID(credentialID, proxyID, userID)
}

func (cp *controlPlane) DeleteWebDataCredential(ctx context.Context, proxyID uint, protocol, username, database, authDatabase string) error {
	if err := cp.validateWebDataProxy(proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	protocol, username, database, authDatabase = normalizeWebDataCredentialIdentity(protocol, username, database, authDatabase)
	return cp.repo.DeleteWebDataCredential(proxyID, userID, protocol, username, database, authDatabase)
}

func (cp *controlPlane) DeleteWebDataCredentialByID(ctx context.Context, proxyID, credentialID uint) error {
	if err := cp.validateWebDataProxy(proxyID); err != nil {
		return err
	}
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return err
	}
	if credentialID == 0 {
		return errors.New("连接 ID 不能为空")
	}
	return cp.repo.DeleteWebDataCredentialByID(credentialID, proxyID, userID)
}

func (cp *controlPlane) RecordWebDataAudit(_ context.Context, audit *WebDataAudit) error {
	if audit == nil {
		return errors.New("访问审计记录不能为空")
	}
	action := strings.TrimSpace(audit.Action)
	if action == "" {
		action = "execute"
	}
	protocol := normalizeWebDataProtocol(audit.Protocol)
	userEmail := strings.TrimSpace(audit.UserEmail)
	if userEmail == "" && audit.UserID > 0 {
		if user, err := cp.repo.GetUserByID(audit.UserID); err == nil && user != nil {
			userEmail = user.Email
		}
	}
	proxyName := strings.TrimSpace(audit.ProxyName)
	if proxyName == "" && audit.ProxyID > 0 {
		if proxy, err := cp.repo.GetProxyByID(audit.ProxyID); err == nil && proxy != nil {
			proxyName = proxy.Name
		}
	}
	applicationName := strings.TrimSpace(audit.ApplicationName)
	if applicationName == "" && audit.ApplicationID > 0 {
		if application, err := cp.repo.GetApplicationByID(audit.ApplicationID); err == nil && application != nil {
			applicationName = application.Name
		}
	}
	details := normalizeAccessAuditDetails(protocol, audit)
	detailsJSON := "{}"
	if data, err := json.Marshal(details); err == nil {
		detailsJSON = string(data)
	}
	return cp.repo.CreateWebDataAudit(&model.WebDataAudit{
		UserID:           audit.UserID,
		UserEmail:        userEmail,
		ProxyID:          audit.ProxyID,
		ProxyName:        proxyName,
		ApplicationID:    audit.ApplicationID,
		ApplicationName:  applicationName,
		Protocol:         protocol,
		Action:           action,
		StatementPreview: audit.StatementPreview,
		StatementSHA256:  audit.StatementSHA256,
		Success:          audit.Success,
		Error:            audit.Error,
		ElapsedMS:        audit.ElapsedMS,
		Details:          detailsJSON,
	})
}

func normalizeAccessAuditDetails(protocol string, audit *WebDataAudit) map[string]any {
	details := map[string]any{}
	if audit == nil {
		return details
	}
	for key, value := range audit.Details {
		key = strings.TrimSpace(key)
		if key == "" || isEmptyAccessAuditDetailValue(value) {
			continue
		}
		details[key] = value
	}
	if value := strings.TrimSpace(audit.Database); value != "" {
		if protocol == "ssh" {
			details["ssh_user"] = value
		} else {
			details["database"] = value
		}
	}
	action := normalizeWebDataAuditAction(audit.Action)
	if audit.AffectedRows != 0 || (action == "execute" && isWebDataProtocol(protocol)) {
		details["affected_rows"] = audit.AffectedRows
	}
	if value := strings.TrimSpace(audit.ClientIP); value != "" {
		details["client_ip"] = value
	}
	return details
}

func parseAccessAuditDetails(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var details map[string]any
	if err := json.Unmarshal([]byte(raw), &details); err != nil || len(details) == 0 {
		return nil
	}
	return details
}

func isEmptyAccessAuditDetailValue(value any) bool {
	switch v := value.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(v) == ""
	case map[string]any:
		return len(v) == 0
	case []any:
		return len(v) == 0
	default:
		return false
	}
}

func (cp *controlPlane) ListWebDataAudits(ctx context.Context, proxyID uint, limit int) ([]*WebDataAuditEntry, error) {
	if err := cp.validateWebDataProxy(proxyID); err != nil {
		return nil, err
	}
	result, err := cp.ListWebDataAuditEntries(ctx, &WebDataAuditListQuery{
		ProxyID: proxyID,
		Limit:   limit,
	})
	if err != nil {
		return nil, err
	}
	return result.Items, nil
}

func (cp *controlPlane) ListWebDataAuditEntries(ctx context.Context, query *WebDataAuditListQuery) (*WebDataAuditList, error) {
	userID, err := requireWebSSHUserID(ctx)
	if err != nil {
		return nil, err
	}
	if query == nil {
		query = &WebDataAuditListQuery{}
	}
	if query.ProxyID > 0 {
		if err := cp.validateAccessAuditProxy(query.ProxyID); err != nil {
			return nil, err
		}
	}
	page := query.Page
	pageSize := query.PageSize
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = query.Limit
	}
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 500 {
		pageSize = 500
	}
	normalizedProtocol := normalizeWebDataProtocol(query.Protocol)
	if query.Protocol != "" && !isAccessAuditProtocol(normalizedProtocol) {
		return nil, errors.New("访问审计协议类型不支持")
	}
	action := normalizeWebDataAuditAction(query.Action)
	if query.Action != "" && action == "" {
		return nil, errors.New("访问审计类型不支持")
	}
	auditQuery := &dao.ListWebDataAuditsQuery{
		UserID:    userID,
		ProxyID:   query.ProxyID,
		Protocol:  normalizedProtocol,
		Action:    action,
		Success:   query.Success,
		Keyword:   strings.TrimSpace(query.Keyword),
		StartTime: query.StartTime,
		EndTime:   query.EndTime,
		Limit:     pageSize,
		Offset:    (page - 1) * pageSize,
	}
	audits, err := cp.repo.ListWebDataAudits(&dao.ListWebDataAuditsQuery{
		UserID:    auditQuery.UserID,
		ProxyID:   auditQuery.ProxyID,
		Protocol:  auditQuery.Protocol,
		Action:    auditQuery.Action,
		Success:   auditQuery.Success,
		Keyword:   auditQuery.Keyword,
		StartTime: auditQuery.StartTime,
		EndTime:   auditQuery.EndTime,
		Limit:     auditQuery.Limit,
		Offset:    auditQuery.Offset,
	})
	if err != nil {
		return nil, err
	}
	total, err := cp.repo.CountWebDataAudits(auditQuery)
	if err != nil {
		return nil, err
	}
	users := cp.webDataAuditUsers(audits)
	proxies := cp.webDataAuditProxies(audits)
	applications := cp.webDataAuditApplications(audits)
	entries := make([]*WebDataAuditEntry, 0, len(audits))
	for _, audit := range audits {
		entry := &WebDataAuditEntry{
			ID:               audit.ID,
			UserID:           audit.UserID,
			UserEmail:        audit.UserEmail,
			ProxyID:          audit.ProxyID,
			ProxyName:        audit.ProxyName,
			ApplicationID:    audit.ApplicationID,
			ApplicationName:  audit.ApplicationName,
			Protocol:         audit.Protocol,
			Action:           normalizeWebDataAuditAction(audit.Action),
			StatementPreview: audit.StatementPreview,
			StatementSHA256:  audit.StatementSHA256,
			Success:          audit.Success,
			Error:            audit.Error,
			ElapsedMS:        audit.ElapsedMS,
			Details:          parseAccessAuditDetails(audit.Details),
			CreatedAt:        audit.CreatedAt.Format(time.RFC3339),
		}
		if entry.Action == "" {
			entry.Action = "execute"
		}
		if entry.UserEmail == "" {
			if user := users[audit.UserID]; user != nil {
				entry.UserEmail = user.Email
			}
		}
		if entry.ProxyName == "" {
			if proxy := proxies[audit.ProxyID]; proxy != nil {
				entry.ProxyName = proxy.Name
			}
		}
		if entry.ApplicationName == "" {
			if application := applications[audit.ApplicationID]; application != nil {
				entry.ApplicationName = application.Name
			}
		}
		entries = append(entries, entry)
	}
	return &WebDataAuditList{
		Items:    entries,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (cp *controlPlane) webDataAuditUsers(audits []*model.WebDataAudit) map[uint]*model.User {
	users := map[uint]*model.User{}
	for _, audit := range audits {
		if audit == nil || audit.UserID == 0 {
			continue
		}
		if _, ok := users[audit.UserID]; ok {
			continue
		}
		user, err := cp.repo.GetUserByID(audit.UserID)
		if err == nil && user != nil {
			users[audit.UserID] = user
		}
	}
	return users
}

func (cp *controlPlane) webDataAuditProxies(audits []*model.WebDataAudit) map[uint]*model.Proxy {
	proxies := map[uint]*model.Proxy{}
	for _, audit := range audits {
		if audit == nil || audit.ProxyID == 0 {
			continue
		}
		if _, ok := proxies[audit.ProxyID]; ok {
			continue
		}
		proxy, err := cp.repo.GetProxyByID(audit.ProxyID)
		if err == nil && proxy != nil {
			proxies[audit.ProxyID] = proxy
		}
	}
	return proxies
}

func (cp *controlPlane) webDataAuditApplications(audits []*model.WebDataAudit) map[uint]*model.Application {
	applications := map[uint]*model.Application{}
	for _, audit := range audits {
		if audit == nil || audit.ApplicationID == 0 {
			continue
		}
		if _, ok := applications[audit.ApplicationID]; ok {
			continue
		}
		application, err := cp.repo.GetApplicationByID(audit.ApplicationID)
		if err == nil && application != nil {
			applications[audit.ApplicationID] = application
		}
	}
	return applications
}

func (cp *controlPlane) loadWebDataTarget(proxyID uint) (*WebDataTarget, error) {
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
	appType := strings.ToLower(string(application.ApplicationType))
	protocol := normalizeWebDataProtocol(appType)
	if !isWebDataProtocol(protocol) {
		return nil, errors.New("仅 MySQL/PostgreSQL/Redis/MongoDB 应用支持 WebData")
	}
	effectiveStatus, effectiveStatusMessage := cp.proxyEffectiveStatus(proxy, application)
	var edgeID uint64
	if len(application.EdgeIDs) > 0 {
		edgeID = uint64(application.EdgeIDs[0])
	}
	return &WebDataTarget{
		ProxyID:                proxy.ID,
		ProxyName:              proxy.Name,
		ApplicationID:          application.ID,
		ApplicationName:        application.Name,
		Protocol:               protocol,
		ApplicationType:        appType,
		TargetHost:             application.IP,
		TargetPort:             application.Port,
		EffectiveStatus:        effectiveStatus,
		EffectiveStatusMessage: effectiveStatusMessage,
		edgeID:                 edgeID,
	}, nil
}

func (cp *controlPlane) validateWebDataProxy(proxyID uint) error {
	_, err := cp.loadWebDataTarget(proxyID)
	return err
}

func (cp *controlPlane) validateAccessAuditProxy(proxyID uint) error {
	if proxyID == 0 {
		return errors.New("访问 ID 不能为空")
	}
	_, err := cp.repo.GetProxyByID(proxyID)
	return err
}

func (cp *controlPlane) loadWebDataCredentials(proxyID, userID uint, protocol string) ([]*WebDataCredential, error) {
	saved, err := cp.repo.ListWebDataCredentialsByProxyAndUser(proxyID, userID, protocol)
	if err != nil {
		return nil, err
	}
	credentials := make([]*WebDataCredential, 0, len(saved))
	for _, item := range saved {
		credentials = append(credentials, webDataCredentialFromModel(item))
	}
	return credentials, nil
}

func isWebDataProtocol(protocol string) bool {
	switch normalizeWebDataProtocol(protocol) {
	case "mysql", "postgresql", "redis", "mongodb":
		return true
	default:
		return false
	}
}

func isAccessAuditProtocol(protocol string) bool {
	switch normalizeWebDataProtocol(protocol) {
	case "ssh":
		return true
	default:
		return isWebDataProtocol(protocol)
	}
}

func normalizeWebDataProtocol(protocol string) string {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "database":
		return "mysql"
	case "postgres", "postgresql":
		return "postgresql"
	case "mongo", "mongodb":
		return "mongodb"
	default:
		return strings.ToLower(strings.TrimSpace(protocol))
	}
}

func normalizeWebDataAuditAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "", "execute":
		return "execute"
	case "test_connection":
		return "test_connection"
	case "open_session":
		return "open_session"
	case "close_session":
		return "close_session"
	case "save_credential":
		return "save_credential"
	case "delete_credential":
		return "delete_credential"
	default:
		return ""
	}
}

func normalizeWebDataCredentialIdentity(protocol, username, database, authDatabase string) (string, string, string, string) {
	return normalizeWebDataProtocol(protocol), strings.TrimSpace(username), strings.TrimSpace(database), strings.TrimSpace(authDatabase)
}

func normalizeWebDataTLSMode(tlsMode string) string {
	switch strings.ToLower(strings.TrimSpace(tlsMode)) {
	case "require", "skip-verify", "preferred", "true":
		return strings.ToLower(strings.TrimSpace(tlsMode))
	default:
		return "disable"
	}
}

func normalizeWebDataAuthMechanism(authMechanism string) string {
	value := strings.ToUpper(strings.TrimSpace(authMechanism))
	switch value {
	case "SCRAM-SHA-1", "SCRAM-SHA-256", "MONGODB-CR", "PLAIN":
		return value
	default:
		return ""
	}
}

func normalizeWebDataConnectionParams(params string) string {
	return strings.TrimSpace(params)
}

func webDataCredentialFromModel(item *model.WebDataCredential) *WebDataCredential {
	if item == nil {
		return nil
	}
	credential := &WebDataCredential{
		ID:               item.ID,
		Saved:            true,
		Name:             item.Name,
		Protocol:         item.Protocol,
		Username:         item.Username,
		Database:         item.Database,
		AuthDatabase:     item.AuthDatabase,
		RedisDB:          item.RedisDB,
		TLSMode:          item.TLSMode,
		Schema:           item.Schema,
		AuthMechanism:    item.AuthMechanism,
		DirectConnection: item.DirectConnection,
		ConnectionParams: item.ConnectionParams,
	}
	if credential.Name == "" {
		credential.Name = defaultWebDataCredentialName(item)
	}
	if credential.TLSMode == "" {
		credential.TLSMode = "disable"
	}
	if item.LastUsedAt != nil {
		credential.LastUsedAt = item.LastUsedAt.Format(time.RFC3339)
	}
	return credential
}

func webDataCredentialSecretFromModel(item *model.WebDataCredential) *WebDataCredentialSecret {
	if item == nil {
		return nil
	}
	tlsMode := item.TLSMode
	if tlsMode == "" {
		tlsMode = "disable"
	}
	return &WebDataCredentialSecret{
		ID:                item.ID,
		Name:              item.Name,
		Protocol:          item.Protocol,
		Username:          item.Username,
		Database:          item.Database,
		AuthDatabase:      item.AuthDatabase,
		RedisDB:           item.RedisDB,
		TLSMode:           tlsMode,
		Schema:            item.Schema,
		AuthMechanism:     item.AuthMechanism,
		DirectConnection:  item.DirectConnection,
		ConnectionParams:  item.ConnectionParams,
		EncryptedPassword: item.EncryptedPassword,
		Nonce:             item.Nonce,
	}
}

func defaultWebDataCredentialName(item *model.WebDataCredential) string {
	if item == nil {
		return "Saved connection"
	}
	parts := make([]string, 0, 3)
	if item.Username != "" {
		parts = append(parts, item.Username)
	}
	if item.Protocol == "redis" {
		parts = append(parts, fmt.Sprintf("db%d", item.RedisDB))
	} else if item.Database != "" {
		parts = append(parts, item.Database)
	}
	if len(parts) == 0 {
		return strings.ToUpper(item.Protocol)
	}
	return strings.Join(parts, " / ")
}
