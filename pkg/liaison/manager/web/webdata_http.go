package web

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/go-sql-driver/mysql"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"gorm.io/gorm"
)

const (
	webDataSessionTTL       = 30 * time.Minute
	webDataConnectTimeout   = 12 * time.Second
	webDataExecuteTimeout   = 2 * time.Minute
	webDataMetadataTimeout  = 30 * time.Second
	webDataResultRowLimit   = 500
	webDataStatementMaxSize = 1024 * 1024
)

type createWebDataSessionRequest struct {
	CredentialID        uint   `json:"credential_id"`
	Protocol            string `json:"protocol"`
	Username            string `json:"username"`
	Password            string `json:"password"`
	Database            string `json:"database"`
	AuthDatabase        string `json:"auth_database"`
	RedisDB             int    `json:"redis_db"`
	RedisDBSet          bool   `json:"-"`
	TLSMode             string `json:"tls_mode"`
	Schema              string `json:"schema"`
	AuthMechanism       string `json:"auth_mechanism"`
	DirectConnection    bool   `json:"direct_connection"`
	DirectConnectionSet bool   `json:"-"`
	ConnectionParams    string `json:"connection_params"`
	SaveCredential      bool   `json:"save_credential"`
}

func (req *createWebDataSessionRequest) UnmarshalJSON(data []byte) error {
	var raw struct {
		CredentialID     uint   `json:"credential_id"`
		Protocol         string `json:"protocol"`
		Username         string `json:"username"`
		Password         string `json:"password"`
		Database         string `json:"database"`
		AuthDatabase     string `json:"auth_database"`
		RedisDB          *int   `json:"redis_db"`
		TLSMode          string `json:"tls_mode"`
		Schema           string `json:"schema"`
		AuthMechanism    string `json:"auth_mechanism"`
		DirectConnection *bool  `json:"direct_connection"`
		ConnectionParams string `json:"connection_params"`
		SaveCredential   bool   `json:"save_credential"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	req.CredentialID = raw.CredentialID
	req.Protocol = raw.Protocol
	req.Username = raw.Username
	req.Password = raw.Password
	req.Database = raw.Database
	req.AuthDatabase = raw.AuthDatabase
	if raw.RedisDB != nil {
		req.RedisDB = *raw.RedisDB
		req.RedisDBSet = true
	}
	req.TLSMode = raw.TLSMode
	req.Schema = raw.Schema
	req.AuthMechanism = raw.AuthMechanism
	req.ConnectionParams = raw.ConnectionParams
	req.SaveCredential = raw.SaveCredential
	if raw.DirectConnection != nil {
		req.DirectConnection = *raw.DirectConnection
		req.DirectConnectionSet = true
	}
	return nil
}

type webDataCredentialRequest struct {
	ID               uint   `json:"id"`
	Name             string `json:"name"`
	Protocol         string `json:"protocol"`
	Username         string `json:"username"`
	Password         string `json:"password"`
	Database         string `json:"database"`
	AuthDatabase     string `json:"auth_database"`
	RedisDB          int    `json:"redis_db"`
	TLSMode          string `json:"tls_mode"`
	Schema           string `json:"schema"`
	AuthMechanism    string `json:"auth_mechanism"`
	DirectConnection bool   `json:"direct_connection"`
	ConnectionParams string `json:"connection_params"`
}

type createWebDataSessionResponse struct {
	Token        string   `json:"token"`
	ExpiresAt    string   `json:"expires_at"`
	Capabilities []string `json:"capabilities"`
}

type webDataExecuteRequest struct {
	Statement string `json:"statement"`
}

type webDataExecuteResponse struct {
	Type         string           `json:"type"`
	Columns      []string         `json:"columns,omitempty"`
	Rows         []map[string]any `json:"rows,omitempty"`
	AffectedRows int64            `json:"affected_rows"`
	Message      string           `json:"message,omitempty"`
	ElapsedMS    int64            `json:"elapsed_ms"`
	Truncated    bool             `json:"truncated"`
	Error        string           `json:"error,omitempty"`
}

type webDataMetadataNode struct {
	Key      string                `json:"key"`
	Title    string                `json:"title"`
	Type     string                `json:"type"`
	Value    string                `json:"value,omitempty"`
	Meta     map[string]string     `json:"meta,omitempty"`
	Children []webDataMetadataNode `json:"children,omitempty"`
}

type webDataMetadataResponse struct {
	Nodes []webDataMetadataNode `json:"nodes"`
}

type webDataObjectRequest struct {
	ObjectType string
	Database   string
	Schema     string
	Name       string
	Key        string
}

type webDataObjectResponse struct {
	ObjectType string           `json:"object_type"`
	Database   string           `json:"database,omitempty"`
	Schema     string           `json:"schema,omitempty"`
	Name       string           `json:"name,omitempty"`
	Key        string           `json:"key,omitempty"`
	DDL        string           `json:"ddl,omitempty"`
	Columns    []map[string]any `json:"columns,omitempty"`
	Indexes    []map[string]any `json:"indexes,omitempty"`
	Message    string           `json:"message,omitempty"`
	Extra      []map[string]any `json:"extra,omitempty"`
}

type webDataSession struct {
	token            string
	userID           uint
	proxyID          uint
	protocol         string
	username         string
	database         string
	authDatabase     string
	redisDB          int
	tlsMode          string
	schema           string
	authMechanism    string
	directConnection bool
	connectionParams string
	expiresAt        time.Time
	target           *controlplane.WebDataTarget
	sqlDB            *sql.DB
	redisClient      *redis.Client
	mongoClient      *mongo.Client
	mu               sync.Mutex
}

type webDataSessionStore struct {
	mu       sync.Mutex
	sessions map[string]*webDataSession
}

type webDataMongoDialer struct {
	dialContext func(context.Context, string, string) (net.Conn, error)
}

type webDataDeadlineSafeConn struct {
	net.Conn
}

func (c webDataDeadlineSafeConn) SetDeadline(t time.Time) error {
	return ignoreDeadlineError(func() error {
		return c.Conn.SetDeadline(t)
	})
}

func (c webDataDeadlineSafeConn) SetReadDeadline(t time.Time) error {
	return ignoreDeadlineError(func() error {
		return c.Conn.SetReadDeadline(t)
	})
}

func (c webDataDeadlineSafeConn) SetWriteDeadline(t time.Time) error {
	return ignoreDeadlineError(func() error {
		return c.Conn.SetWriteDeadline(t)
	})
}

func ignoreDeadlineError(fn func() error) (err error) {
	defer func() {
		if recover() != nil {
			err = nil
		}
	}()
	_ = fn()
	return nil
}

func (d webDataMongoDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := d.dialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	if conn == nil {
		return nil, errors.New("webdata mongodb tunnel returned nil connection")
	}
	return webDataDeadlineSafeConn{Conn: conn}, nil
}

func (web *web) webDataDeadlineSafeDialContext(proxyID uint, label string) func(context.Context, string, string) (net.Conn, error) {
	dialContext := web.webDataDialContext(proxyID)
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		if conn == nil {
			return nil, fmt.Errorf("webdata %s tunnel returned nil connection", label)
		}
		return webDataDeadlineSafeConn{Conn: conn}, nil
	}
}

func newWebDataSessionStore() *webDataSessionStore {
	return &webDataSessionStore{sessions: map[string]*webDataSession{}}
}

func (s *webDataSessionStore) create(session *webDataSession) (*webDataSession, error) {
	token, err := randomWebSSHToken()
	if err != nil {
		return nil, err
	}
	session.token = token
	session.expiresAt = time.Now().Add(webDataSessionTTL)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked(time.Now())
	s.sessions[token] = session
	return session, nil
}

func (s *webDataSessionStore) get(token string) (*webDataSession, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	session, ok := s.sessions[token]
	if !ok {
		return nil, false
	}
	if time.Now().After(session.expiresAt) {
		delete(s.sessions, token)
		session.close()
		return nil, false
	}
	session.expiresAt = time.Now().Add(webDataSessionTTL)
	return session, true
}

func (s *webDataSessionStore) delete(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if session, ok := s.sessions[token]; ok {
		delete(s.sessions, token)
		session.close()
	}
}

func (s *webDataSessionStore) cleanupLocked(now time.Time) {
	for token, session := range s.sessions {
		if now.After(session.expiresAt) {
			delete(s.sessions, token)
			session.close()
		}
	}
}

func (s *webDataSession) close() {
	if s.sqlDB != nil {
		_ = s.sqlDB.Close()
		s.sqlDB = nil
	}
	if s.redisClient != nil {
		_ = s.redisClient.Close()
		s.redisClient = nil
	}
	if s.mongoClient != nil {
		_ = s.mongoClient.Disconnect(context.Background())
		s.mongoClient = nil
	}
}

func (web *web) handleWebDataTargetHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDataProxyID(r, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebDataTarget(ctx, proxyID)
	if err != nil {
		status := webDataHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": target})
}

func (web *web) handleCreateWebDataSessionHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDataProxyID(r, "/session")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	var req createWebDataSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid request body"})
		return
	}
	normalizeWebDataSessionRequest(&req)

	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebDataTarget(ctx, proxyID)
	if err != nil {
		status := webDataHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	if target.EffectiveStatus != "active" {
		writeJSON(w, http.StatusConflict, map[string]any{"code": http.StatusConflict, "message": target.EffectiveStatusMessage})
		return
	}

	password := []byte(req.Password)
	savedCredential := false
	if req.CredentialID > 0 {
		credential, err := web.controlPlane.GetWebDataCredentialSecretByID(ctx, proxyID, req.CredentialID)
		if err != nil {
			status := webDataHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
		password, err = web.decryptWebSSHPassword(credential.EncryptedPassword, credential.Nonce)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "已保存密码无法解密，请清除后重新保存"})
			return
		}
		applyWebDataCredentialDefaults(&req, credential)
		savedCredential = true
	} else if len(password) == 0 {
		if credential, err := web.controlPlane.GetWebDataCredentialSecret(ctx, proxyID, req.Protocol, req.Username, req.Database, req.AuthDatabase); err == nil {
			password, err = web.decryptWebSSHPassword(credential.EncryptedPassword, credential.Nonce)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "已保存密码无法解密，请清除后重新保存"})
				return
			}
			applyWebDataCredentialDefaults(&req, credential)
			savedCredential = true
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			status := webDataHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
	}
	if req.Protocol == "" {
		req.Protocol = target.Protocol
	}
	if req.Protocol != target.Protocol {
		zeroBytes(password)
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "协议类型与应用类型不匹配"})
		return
	}
	if req.SaveCredential && !savedCredential && len(password) > 0 && web.credentialKey == nil {
		zeroBytes(password)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "WebData 凭据加密密钥未配置"})
		return
	}

	session := &webDataSession{
		userID:           user.ID,
		proxyID:          proxyID,
		protocol:         req.Protocol,
		username:         req.Username,
		database:         req.Database,
		authDatabase:     req.AuthDatabase,
		redisDB:          req.RedisDB,
		tlsMode:          req.TLSMode,
		schema:           req.Schema,
		authMechanism:    req.AuthMechanism,
		directConnection: req.DirectConnection,
		connectionParams: req.ConnectionParams,
		target:           target,
	}
	connectCtx, cancel := context.WithTimeout(ctx, webDataConnectTimeout)
	defer cancel()
	started := time.Now()
	if err := web.openWebDataClient(connectCtx, session, string(password)); err != nil {
		elapsed := time.Since(started).Milliseconds()
		log.Debugf("webdata session open failed: proxy_id=%d user_id=%d protocol=%s err=%v", proxyID, user.ID, req.Protocol, err)
		web.recordWebDataAudit(r, target, user.ID, "open_session", req.Protocol, webDataAuditDatabase(&req), "", false, 0, elapsed, "connection failed")
		zeroBytes(password)
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "连接失败"})
		return
	}
	elapsed := time.Since(started).Milliseconds()
	if req.SaveCredential && !savedCredential && len(password) > 0 {
		encryptedPassword, nonce, err := web.encryptWebSSHPassword(password)
		if err != nil {
			zeroBytes(password)
			session.close()
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": err.Error()})
			return
		}
		if _, err := web.controlPlane.SaveWebDataCredentialProfile(ctx, proxyID, &controlplane.WebDataCredentialProfile{
			Protocol:          req.Protocol,
			Username:          req.Username,
			Database:          req.Database,
			AuthDatabase:      req.AuthDatabase,
			RedisDB:           req.RedisDB,
			TLSMode:           req.TLSMode,
			Schema:            req.Schema,
			AuthMechanism:     req.AuthMechanism,
			DirectConnection:  req.DirectConnection,
			ConnectionParams:  req.ConnectionParams,
			EncryptedPassword: encryptedPassword,
			Nonce:             nonce,
			PasswordChanged:   true,
		}); err != nil {
			zeroBytes(password)
			session.close()
			writeJSON(w, webDataHTTPStatus(err), map[string]any{"code": webDataHTTPStatus(err), "message": err.Error()})
			return
		}
	} else if savedCredential {
		if req.CredentialID > 0 {
			_ = web.controlPlane.TouchWebDataCredentialByID(ctx, proxyID, req.CredentialID)
		} else {
			_ = web.controlPlane.TouchWebDataCredential(ctx, proxyID, req.Protocol, req.Username, req.Database, req.AuthDatabase)
		}
	}
	zeroBytes(password)
	req.Password = ""

	created, err := web.webData.create(session)
	if err != nil {
		session.close()
		web.recordWebDataAudit(r, target, user.ID, "open_session", req.Protocol, webDataAuditDatabase(&req), "", false, 0, elapsed, "failed to create session")
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "failed to create session"})
		return
	}
	web.recordWebDataAudit(r, target, user.ID, "open_session", req.Protocol, webDataAuditDatabase(&req), "", true, 0, elapsed, "")
	writeJSON(w, http.StatusOK, map[string]any{
		"code":    200,
		"message": "success",
		"data": createWebDataSessionResponse{
			Token:        created.token,
			ExpiresAt:    created.expiresAt.Format(time.RFC3339),
			Capabilities: webDataCapabilities(created.protocol),
		},
	})
}

func (web *web) handleTestWebDataConnectionHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDataProxyID(r, "/test")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	var req createWebDataSessionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid request body"})
		return
	}
	normalizeWebDataSessionRequest(&req)

	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebDataTarget(ctx, proxyID)
	if err != nil {
		status := webDataHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	if target.EffectiveStatus != "active" {
		writeJSON(w, http.StatusConflict, map[string]any{"code": http.StatusConflict, "message": target.EffectiveStatusMessage})
		return
	}

	password := []byte(req.Password)
	if req.CredentialID > 0 && len(password) == 0 {
		credential, err := web.controlPlane.GetWebDataCredentialSecretByID(ctx, proxyID, req.CredentialID)
		if err != nil {
			status := webDataHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
		password, err = web.decryptWebSSHPassword(credential.EncryptedPassword, credential.Nonce)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "已保存密码无法解密，请清除后重新保存"})
			return
		}
		applyWebDataCredentialDefaults(&req, credential)
	} else if len(password) == 0 {
		if credential, err := web.controlPlane.GetWebDataCredentialSecret(ctx, proxyID, req.Protocol, req.Username, req.Database, req.AuthDatabase); err == nil {
			password, err = web.decryptWebSSHPassword(credential.EncryptedPassword, credential.Nonce)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "已保存密码无法解密，请清除后重新保存"})
				return
			}
			applyWebDataCredentialDefaults(&req, credential)
		} else if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			status := webDataHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
	}
	if req.Protocol == "" {
		req.Protocol = target.Protocol
	}
	if req.Protocol != target.Protocol {
		zeroBytes(password)
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "协议类型与应用类型不匹配"})
		return
	}

	session := &webDataSession{
		userID:           user.ID,
		proxyID:          proxyID,
		protocol:         req.Protocol,
		username:         req.Username,
		database:         req.Database,
		authDatabase:     req.AuthDatabase,
		redisDB:          req.RedisDB,
		tlsMode:          req.TLSMode,
		schema:           req.Schema,
		authMechanism:    req.AuthMechanism,
		directConnection: req.DirectConnection,
		connectionParams: req.ConnectionParams,
		target:           target,
	}
	connectCtx, cancel := context.WithTimeout(ctx, webDataConnectTimeout)
	defer cancel()
	started := time.Now()
	if err := web.openWebDataClient(connectCtx, session, string(password)); err != nil {
		elapsed := time.Since(started).Milliseconds()
		log.Debugf("webdata connection test failed: proxy_id=%d user_id=%d protocol=%s err=%v", proxyID, user.ID, req.Protocol, err)
		web.recordWebDataAudit(r, target, user.ID, "test_connection", req.Protocol, webDataAuditDatabase(&req), "", false, 0, elapsed, "connection test failed")
		zeroBytes(password)
		session.close()
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "测试连接失败"})
		return
	}
	elapsed := time.Since(started).Milliseconds()
	zeroBytes(password)
	req.Password = ""
	session.close()
	web.recordWebDataAudit(r, target, user.ID, "test_connection", req.Protocol, webDataAuditDatabase(&req), "", true, 0, elapsed, "")
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "连接测试成功"})
}

func (web *web) handleWebDataCredentialHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete && r.Method != http.MethodPost && r.Method != http.MethodPut {
		w.Header().Set("Allow", "POST, PUT, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	proxyID, err := parseWebDataProxyID(r, "/credential")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	target, err := web.controlPlane.GetWebDataTarget(ctx, proxyID)
	if err != nil {
		status := webDataHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	if r.Method == http.MethodDelete {
		q := r.URL.Query()
		auditDatabase := strings.TrimSpace(q.Get("database"))
		if id, _ := strconv.ParseUint(strings.TrimSpace(q.Get("id")), 10, 64); id > 0 {
			if err := web.controlPlane.DeleteWebDataCredentialByID(ctx, proxyID, uint(id)); err != nil {
				web.recordWebDataAudit(r, target, user.ID, "delete_credential", target.Protocol, auditDatabase, "", false, 0, 0, err.Error())
				status := webDataHTTPStatus(err)
				writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
				return
			}
		} else if err := web.controlPlane.DeleteWebDataCredential(ctx, proxyID, q.Get("protocol"), q.Get("username"), q.Get("database"), q.Get("auth_database")); err != nil {
			web.recordWebDataAudit(r, target, user.ID, "delete_credential", q.Get("protocol"), auditDatabase, "", false, 0, 0, err.Error())
			status := webDataHTTPStatus(err)
			writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
			return
		}
		web.recordWebDataAudit(r, target, user.ID, "delete_credential", target.Protocol, auditDatabase, "", true, 0, 0, "")
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success"})
		return
	}

	var req webDataCredentialRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid request body"})
		return
	}
	req.Protocol = normalizeWebDataProtocol(req.Protocol)
	req.Username = strings.TrimSpace(req.Username)
	req.Database = strings.TrimSpace(req.Database)
	req.AuthDatabase = strings.TrimSpace(req.AuthDatabase)
	req.TLSMode = strings.ToLower(strings.TrimSpace(req.TLSMode))

	password := []byte(req.Password)
	passwordChanged := req.ID == 0 || len(password) > 0
	if passwordChanged && web.credentialKey == nil {
		zeroBytes(password)
		writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": "WebData 凭据加密密钥未配置"})
		return
	}

	var encryptedPassword, nonce string
	if passwordChanged {
		encryptedPassword, nonce, err = web.encryptWebSSHPassword(password)
		zeroBytes(password)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]any{"code": http.StatusInternalServerError, "message": err.Error()})
			return
		}
	}
	credential, err := web.controlPlane.SaveWebDataCredentialProfile(ctx, proxyID, &controlplane.WebDataCredentialProfile{
		ID:                req.ID,
		Name:              req.Name,
		Protocol:          req.Protocol,
		Username:          req.Username,
		Database:          req.Database,
		AuthDatabase:      req.AuthDatabase,
		RedisDB:           req.RedisDB,
		TLSMode:           req.TLSMode,
		Schema:            req.Schema,
		AuthMechanism:     req.AuthMechanism,
		DirectConnection:  req.DirectConnection,
		ConnectionParams:  req.ConnectionParams,
		EncryptedPassword: encryptedPassword,
		Nonce:             nonce,
		PasswordChanged:   passwordChanged,
	})
	req.Password = ""
	if err != nil {
		web.recordWebDataAudit(r, target, user.ID, "save_credential", req.Protocol, webDataAuditCredentialDatabase(&req), "", false, 0, 0, err.Error())
		status := webDataHTTPStatus(err)
		writeJSON(w, status, map[string]any{"code": status, "message": err.Error()})
		return
	}
	web.recordWebDataAudit(r, target, user.ID, "save_credential", req.Protocol, webDataAuditCredentialDatabase(&req), "", true, 0, 0, "")
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": credential})
}

func (web *web) handleWebDataAuditsHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	ctx := context.WithValue(r.Context(), "user_id", user.ID)
	proxyID, err := parseWebDataProxyID(r, "/audits")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid proxy id"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 500 {
			writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid limit"})
			return
		}
		limit = parsed
	}
	audits, err := web.controlPlane.ListWebDataAudits(ctx, proxyID, limit)
	if err != nil {
		writeJSON(w, webDataHTTPStatus(err), map[string]any{"code": webDataHTTPStatus(err), "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": map[string]any{"items": audits}})
}

func (web *web) handleWebDataSessionHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		web.handleWebDataExecuteHTTP(w, r)
	case http.MethodDelete:
		web.handleDeleteWebDataSessionHTTP(w, r)
	default:
		w.Header().Set("Allow", "POST, DELETE")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
	}
}

func (web *web) handleWebDataExecuteHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	token, err := parseWebDataSessionToken(r, "/execute")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid webdata session"})
		return
	}
	session, ok := web.webData.get(token)
	if !ok || session.userID != user.ID {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": http.StatusUnauthorized, "message": "invalid or expired webdata session"})
		return
	}
	var req webDataExecuteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid request body"})
		return
	}
	statement := strings.TrimSpace(req.Statement)
	if statement == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "请输入要执行的命令"})
		return
	}
	if len(statement) > webDataStatementMaxSize {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "命令过大"})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	started := time.Now()
	execCtx, cancel := context.WithTimeout(r.Context(), webDataExecuteTimeout)
	defer cancel()
	result, execErr := session.execute(execCtx, statement)
	elapsed := time.Since(started).Milliseconds()
	if result == nil {
		result = &webDataExecuteResponse{Type: "message"}
	}
	result.ElapsedMS = elapsed
	if execErr != nil {
		result.Error = execErr.Error()
	}
	result.AffectedRows = normalizeAffectedRows(result.AffectedRows)
	if webDataShouldAuditExecute(session.protocol, statement) {
		clientIP, clientIPSource := remoteClientIPInfo(r)
		if err := web.controlPlane.RecordWebDataAudit(r.Context(), &controlplane.WebDataAudit{
			UserID:           user.ID,
			ProxyID:          session.proxyID,
			ApplicationID:    session.target.ApplicationID,
			Protocol:         session.protocol,
			Action:           "execute",
			Database:         webDataAuditSessionDatabase(session),
			StatementPreview: webDataStatementPreview(statement),
			StatementSHA256:  webDataStatementHash(statement),
			Success:          execErr == nil,
			AffectedRows:     result.AffectedRows,
			Error:            result.Error,
			ElapsedMS:        elapsed,
			ClientIP:         clientIP,
			Details:          map[string]any{"client_ip_source": clientIPSource},
		}); err != nil {
			log.Warnf("webdata audit record failed: proxy_id=%d user_id=%d action=execute err=%v", session.proxyID, user.ID, err)
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": result})
}

func (web *web) handleDeleteWebDataSessionHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	token, err := parseWebDataSessionToken(r, "")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid webdata session"})
		return
	}
	session, ok := web.webData.get(token)
	if !ok || session.userID != user.ID {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": http.StatusUnauthorized, "message": "invalid or expired webdata session"})
		return
	}
	web.webData.delete(token)
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success"})
}

func (web *web) handleWebDataMetadataHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	token, err := parseWebDataSessionToken(r, "/metadata")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid webdata session"})
		return
	}
	session, ok := web.webData.get(token)
	if !ok || session.userID != user.ID {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": http.StatusUnauthorized, "message": "invalid or expired webdata session"})
		return
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), webDataMetadataTimeout)
	defer cancel()
	nodes, err := session.metadata(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": webDataMetadataResponse{Nodes: []webDataMetadataNode{
			{Key: "metadata-error", Title: err.Error(), Type: "error"},
		}}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": webDataMetadataResponse{Nodes: nodes}})
}

func (web *web) handleWebDataObjectHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", "GET")
		writeJSON(w, http.StatusMethodNotAllowed, map[string]any{"code": http.StatusMethodNotAllowed, "message": "method not allowed"})
		return
	}
	user, err := web.authenticateHTTP(r)
	if err != nil {
		writeUnauthorized(w)
		return
	}
	token, err := parseWebDataSessionToken(r, "/object")
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "invalid webdata session"})
		return
	}
	session, ok := web.webData.get(token)
	if !ok || session.userID != user.ID {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"code": http.StatusUnauthorized, "message": "invalid or expired webdata session"})
		return
	}
	q := r.URL.Query()
	req := webDataObjectRequest{
		ObjectType: strings.TrimSpace(q.Get("type")),
		Database:   strings.TrimSpace(q.Get("database")),
		Schema:     strings.TrimSpace(q.Get("schema")),
		Name:       strings.TrimSpace(q.Get("name")),
		Key:        strings.TrimSpace(q.Get("key")),
	}
	if req.ObjectType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]any{"code": http.StatusBadRequest, "message": "object type is required"})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	ctx, cancel := context.WithTimeout(r.Context(), webDataMetadataTimeout)
	defer cancel()
	detail, err := session.objectDetails(ctx, req)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": webDataObjectResponse{
			ObjectType: req.ObjectType,
			Database:   req.Database,
			Schema:     req.Schema,
			Name:       req.Name,
			Key:        req.Key,
			Message:    err.Error(),
		}})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"code": 200, "message": "success", "data": detail})
}

func (web *web) openWebDataClient(ctx context.Context, session *webDataSession, password string) error {
	switch session.protocol {
	case "mysql":
		return web.openWebDataMySQL(ctx, session, password)
	case "postgresql":
		return web.openWebDataPostgreSQL(ctx, session, password)
	case "redis":
		return web.openWebDataRedis(ctx, session, password)
	case "mongodb":
		return web.openWebDataMongoDB(ctx, session, password)
	default:
		return fmt.Errorf("unsupported protocol %q", session.protocol)
	}
}

func (web *web) openWebDataMySQL(ctx context.Context, session *webDataSession, password string) error {
	cfg := mysql.NewConfig()
	cfg.User = session.username
	cfg.Passwd = password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(session.target.TargetHost, strconv.Itoa(session.target.TargetPort))
	cfg.DBName = session.database
	cfg.ParseTime = true
	cfg.AllowNativePasswords = true
	cfg.Timeout = webDataConnectTimeout
	cfg.ReadTimeout = webDataExecuteTimeout
	cfg.WriteTimeout = webDataExecuteTimeout
	cfg.DialFunc = web.webDataDeadlineSafeDialContext(session.proxyID, "mysql")
	if session.tlsMode == "require" || session.tlsMode == "true" {
		cfg.TLSConfig = "true"
	} else if session.tlsMode == "skip-verify" || session.tlsMode == "preferred" {
		cfg.TLSConfig = session.tlsMode
	}
	params, err := parseWebDataConnectionParams(session.connectionParams)
	if err != nil {
		return err
	}
	if len(params) > 0 {
		cfg.Params = params
	}
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return err
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(webDataSessionTTL)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return err
	}
	session.sqlDB = db
	return nil
}

func (web *web) openWebDataPostgreSQL(ctx context.Context, session *webDataSession, password string) error {
	sslMode := webDataPostgresSSLMode(session.tlsMode)
	u := &url.URL{
		Scheme: "postgres",
		Host:   net.JoinHostPort(session.target.TargetHost, strconv.Itoa(session.target.TargetPort)),
	}
	if session.username != "" || password != "" {
		u.User = url.UserPassword(session.username, password)
	}
	if session.database != "" {
		u.Path = "/" + session.database
	}
	q := u.Query()
	params, err := parseWebDataConnectionParams(session.connectionParams)
	if err != nil {
		return err
	}
	for key, value := range params {
		q.Set(key, value)
	}
	if session.schema != "" {
		q.Set("search_path", session.schema)
	}
	q.Set("sslmode", sslMode)
	u.RawQuery = q.Encode()
	cfg, err := pgx.ParseConfig(u.String())
	if err != nil {
		return err
	}
	if session.tlsMode == "skip-verify" {
		if cfg.TLSConfig == nil {
			cfg.TLSConfig = &tls.Config{}
		} else {
			cfg.TLSConfig = cfg.TLSConfig.Clone()
		}
		cfg.TLSConfig.InsecureSkipVerify = true //nolint:gosec // User-selected compatibility mode for private database endpoints.
	}
	cfg.DialFunc = web.webDataDeadlineSafeDialContext(session.proxyID, "postgresql")
	db := stdlib.OpenDB(*cfg)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(webDataSessionTTL)
	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return err
	}
	session.sqlDB = db
	return nil
}

func webDataPostgresSSLMode(tlsMode string) string {
	switch tlsMode {
	case "require", "true", "skip-verify":
		return "require"
	case "preferred":
		return "prefer"
	default:
		return "disable"
	}
}

func (web *web) openWebDataRedis(ctx context.Context, session *webDataSession, password string) error {
	addr := net.JoinHostPort(session.target.TargetHost, strconv.Itoa(session.target.TargetPort))
	tlsConfig := webDataTLSConfig(session.tlsMode)
	opt := &redis.Options{
		Addr:           addr,
		Username:       session.username,
		Password:       password,
		DB:             session.redisDB,
		Dialer:         web.webDataRedisDialContext(session.proxyID, addr, tlsConfig),
		DialTimeout:    webDataConnectTimeout,
		ReadTimeout:    webDataExecuteTimeout,
		WriteTimeout:   webDataExecuteTimeout,
		PoolSize:       1,
		MinIdleConns:   0,
		MaxIdleConns:   1,
		MaxActiveConns: 1,
	}
	client := redis.NewClient(opt)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return err
	}
	session.redisClient = client
	return nil
}

func (web *web) webDataRedisDialContext(proxyID uint, addr string, tlsConfig *tls.Config) func(context.Context, string, string) (net.Conn, error) {
	dialContext := web.webDataDeadlineSafeDialContext(proxyID, "redis")
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		conn, err := dialContext(ctx, network, address)
		if err != nil || tlsConfig == nil {
			return conn, err
		}
		cfg := tlsConfig.Clone()
		if cfg.ServerName == "" {
			host, _, splitErr := net.SplitHostPort(addr)
			if splitErr == nil {
				cfg.ServerName = host
			}
		}
		tlsConn := tls.Client(conn, cfg)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			_ = conn.Close()
			return nil, err
		}
		return webDataDeadlineSafeConn{Conn: tlsConn}, nil
	}
}

func (web *web) openWebDataMongoDB(ctx context.Context, session *webDataSession, password string) error {
	u := &url.URL{
		Scheme: "mongodb",
		Host:   net.JoinHostPort(session.target.TargetHost, strconv.Itoa(session.target.TargetPort)),
	}
	params, err := parseWebDataConnectionParams(session.connectionParams)
	if err != nil {
		return err
	}
	q := u.Query()
	for key, value := range params {
		q.Set(key, value)
	}
	u.RawQuery = q.Encode()
	clientOptions := options.Client().
		ApplyURI(u.String()).
		SetDialer(webDataMongoDialer{dialContext: web.webDataDialContext(session.proxyID)}).
		SetConnectTimeout(webDataConnectTimeout).
		SetServerSelectionTimeout(webDataConnectTimeout).
		SetDirect(session.directConnection).
		SetMaxPoolSize(1)
	if tlsConfig := webDataTLSConfig(session.tlsMode); tlsConfig != nil {
		clientOptions.SetTLSConfig(tlsConfig)
	}
	if session.username != "" || password != "" {
		authSource := session.authDatabase
		if authSource == "" {
			authSource = session.database
		}
		if authSource == "" {
			authSource = "admin"
		}
		clientOptions.SetAuth(options.Credential{
			AuthSource:    authSource,
			AuthMechanism: session.authMechanism,
			Username:      session.username,
			Password:      password,
		})
	}
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return err
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(context.Background())
		return err
	}
	session.mongoClient = client
	return nil
}

func (web *web) webDataDialContext(proxyID uint) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, _, err := web.controlPlane.OpenWebDataStream(ctx, proxyID)
		return conn, err
	}
}

func (s *webDataSession) execute(ctx context.Context, statement string) (*webDataExecuteResponse, error) {
	switch s.protocol {
	case "mysql", "postgresql":
		return s.executeSQL(ctx, statement)
	case "redis":
		return s.executeRedis(ctx, statement)
	case "mongodb":
		return s.executeMongo(ctx, statement)
	default:
		return nil, fmt.Errorf("unsupported protocol %q", s.protocol)
	}
}

func (s *webDataSession) executeSQL(ctx context.Context, statement string) (*webDataExecuteResponse, error) {
	if s.sqlDB == nil {
		return nil, errors.New("SQL 会话未连接")
	}
	if sqlStatementReturnsRows(statement) {
		rows, err := s.sqlDB.QueryContext(ctx, statement)
		if err != nil {
			return &webDataExecuteResponse{Type: "rows"}, err
		}
		defer rows.Close()
		return readSQLRows(rows)
	}
	result, err := s.sqlDB.ExecContext(ctx, statement)
	if err != nil {
		return &webDataExecuteResponse{Type: "message"}, err
	}
	affected, _ := result.RowsAffected()
	return &webDataExecuteResponse{
		Type:         "message",
		AffectedRows: affected,
		Message:      fmt.Sprintf("OK, %d row(s) affected", normalizeAffectedRows(affected)),
	}, nil
}

func (s *webDataSession) executeRedis(ctx context.Context, statement string) (*webDataExecuteResponse, error) {
	if s.redisClient == nil {
		return nil, errors.New("Redis 会话未连接")
	}
	args, err := parseRedisCommand(statement)
	if err != nil {
		return &webDataExecuteResponse{Type: "redis"}, err
	}
	cmd := redis.NewCmd(ctx, args...)
	if err := s.redisClient.Process(ctx, cmd); err != nil && !errors.Is(err, redis.Nil) {
		return &webDataExecuteResponse{Type: "redis"}, err
	}
	value, err := cmd.Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return &webDataExecuteResponse{Type: "redis"}, err
	}
	rows, columns, message := redisValueToRows(args, value, err)
	return &webDataExecuteResponse{
		Type:    "redis",
		Columns: columns,
		Rows:    rows,
		Message: message,
	}, nil
}

func (s *webDataSession) executeMongo(ctx context.Context, statement string) (*webDataExecuteResponse, error) {
	if s.mongoClient == nil {
		return nil, errors.New("MongoDB 会话未连接")
	}
	var command bson.D
	if err := bson.UnmarshalExtJSON([]byte(statement), true, &command); err != nil {
		return &webDataExecuteResponse{Type: "json"}, err
	}
	database := s.database
	if database == "" {
		database = s.authDatabase
	}
	if database == "" {
		database = "admin"
	}
	if strings.EqualFold(mongoCommandName(command), "find") {
		return s.executeMongoFind(ctx, database, command)
	}
	var result bson.M
	if err := s.mongoClient.Database(database).RunCommand(ctx, command).Decode(&result); err != nil {
		return &webDataExecuteResponse{Type: "json"}, err
	}
	row := bsonMapToPlainMap(result)
	return &webDataExecuteResponse{
		Type:    "json",
		Columns: sortedMapKeys(row),
		Rows:    []map[string]any{row},
		Message: "OK",
	}, nil
}

func (s *webDataSession) executeMongoFind(ctx context.Context, database string, command bson.D) (*webDataExecuteResponse, error) {
	collectionName, ok := mongoCommandString(command, "find")
	if !ok || strings.TrimSpace(collectionName) == "" {
		return &webDataExecuteResponse{Type: "json"}, errors.New("find collection is required")
	}
	filter := mongoCommandValue(command, "filter")
	if filter == nil {
		filter = bson.D{}
	}
	filter = normalizeMongoObjectIDFilter(filter)
	limit := mongoCommandInt64(command, "limit", 100)
	if limit < 1 {
		limit = 100
	}
	if limit > webDataResultRowLimit {
		limit = webDataResultRowLimit
	}
	cursor, err := s.mongoClient.Database(database).Collection(collectionName).Find(ctx, filter, options.Find().SetLimit(limit))
	if err != nil {
		return &webDataExecuteResponse{Type: "json"}, err
	}
	defer cursor.Close(ctx)
	rows := []map[string]any{}
	for cursor.Next(ctx) {
		var item bson.M
		if err := cursor.Decode(&item); err != nil {
			return &webDataExecuteResponse{Type: "json"}, err
		}
		rows = append(rows, bsonMapToPlainMap(item))
	}
	if err := cursor.Err(); err != nil {
		return &webDataExecuteResponse{Type: "json"}, err
	}
	return &webDataExecuteResponse{
		Type:    "json",
		Columns: unionRowKeys(rows),
		Rows:    rows,
		Message: fmt.Sprintf("%d document(s)", len(rows)),
	}, nil
}

func normalizeMongoObjectIDFilter(value any) any {
	return normalizeMongoObjectIDValue(value, false)
}

func normalizeMongoObjectIDValue(value any, objectIDField bool) any {
	if objectIDField {
		if converted, ok := mongoObjectIDFromText(value); ok {
			return converted
		}
	}
	switch typed := value.(type) {
	case bson.D:
		out := make(bson.D, 0, len(typed))
		for _, item := range typed {
			nextObjectIDField := objectIDField || isMongoObjectIDFieldKey(item.Key)
			out = append(out, bson.E{Key: item.Key, Value: normalizeMongoObjectIDValue(item.Value, nextObjectIDField)})
		}
		return out
	case bson.M:
		out := bson.M{}
		for key, item := range typed {
			nextObjectIDField := objectIDField || isMongoObjectIDFieldKey(key)
			out[key] = normalizeMongoObjectIDValue(item, nextObjectIDField)
		}
		return out
	case bson.A:
		out := make(bson.A, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeMongoObjectIDValue(item, objectIDField))
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, normalizeMongoObjectIDValue(item, objectIDField))
		}
		return out
	default:
		return value
	}
}

func mongoObjectIDFromText(value any) (primitive.ObjectID, bool) {
	text, ok := value.(string)
	if !ok {
		return primitive.NilObjectID, false
	}
	text = strings.TrimSpace(text)
	if strings.HasPrefix(strings.ToLower(text), "objectid(") && strings.HasSuffix(text, ")") {
		text = strings.TrimSpace(strings.TrimSuffix(text[len("ObjectId("):], ")"))
		text = strings.Trim(text, `"'`)
	}
	objectID, err := primitive.ObjectIDFromHex(text)
	if err != nil {
		return primitive.NilObjectID, false
	}
	return objectID, true
}

func isMongoObjectIDFieldKey(key string) bool {
	normalized := strings.TrimSpace(key)
	return normalized == "_id" || strings.HasSuffix(normalized, "._id")
}

func mongoCommandName(command bson.D) string {
	if len(command) == 0 {
		return ""
	}
	return command[0].Key
}

func mongoCommandValue(command bson.D, key string) any {
	for _, item := range command {
		if strings.EqualFold(item.Key, key) {
			return item.Value
		}
	}
	return nil
}

func mongoCommandString(command bson.D, key string) (string, bool) {
	value := mongoCommandValue(command, key)
	text, ok := value.(string)
	return text, ok
}

func mongoCommandInt64(command bson.D, key string, fallback int64) int64 {
	switch value := mongoCommandValue(command, key).(type) {
	case int:
		return int64(value)
	case int32:
		return int64(value)
	case int64:
		return value
	case float64:
		return int64(value)
	default:
		return fallback
	}
}

func (s *webDataSession) metadata(ctx context.Context) ([]webDataMetadataNode, error) {
	switch s.protocol {
	case "mysql":
		return s.mysqlMetadata(ctx)
	case "postgresql":
		return s.postgresMetadata(ctx)
	case "redis":
		return s.redisMetadata(ctx)
	case "mongodb":
		return s.mongoMetadata(ctx)
	default:
		return nil, fmt.Errorf("unsupported protocol %q", s.protocol)
	}
}

func (s *webDataSession) mysqlMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	dbs, err := querySingleColumn(ctx, s.sqlDB, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	columnMap := map[string][]webDataMetadataNode{}
	if s.database != "" {
		rows, err := s.sqlDB.QueryContext(ctx, `
SELECT table_name, column_name, column_type, is_nullable, column_key
FROM information_schema.columns
WHERE table_schema = ?
ORDER BY table_name, ordinal_position`, s.database)
		if err == nil {
			defer rows.Close()
			for rows.Next() {
				var table, column, columnType, nullable, key string
				if err := rows.Scan(&table, &column, &columnType, &nullable, &key); err != nil {
					return nil, err
				}
				value := columnType
				if key != "" {
					value += " " + key
				}
				if nullable == "NO" {
					value += " not null"
				}
				columnMap[table] = append(columnMap[table], webDataMetadataNode{
					Key:   "mysql-column-" + s.database + "-" + table + "-" + column,
					Title: column,
					Type:  "column",
					Value: value,
					Meta: map[string]string{
						"database": s.database,
						"name":     table,
						"column":   column,
					},
				})
			}
			if err := rows.Err(); err != nil {
				return nil, err
			}
		}
	}
	root := webDataMetadataNode{Key: "mysql-databases", Title: "Databases", Type: "root"}
	for _, db := range dbs {
		node := webDataMetadataNode{Key: "mysql-db-" + db, Title: db, Type: "database", Meta: map[string]string{"database": db}}
		if s.database != "" && db == s.database {
			tables, _ := querySingleColumn(ctx, s.sqlDB, "SHOW TABLES")
			for _, table := range tables {
				node.Children = append(node.Children, webDataMetadataNode{
					Key:      "mysql-table-" + db + "-" + table,
					Title:    table,
					Type:     "table",
					Meta:     map[string]string{"database": db, "name": table},
					Children: columnMap[table],
				})
			}
		}
		root.Children = append(root.Children, node)
	}
	return []webDataMetadataNode{root}, nil
}

func (s *webDataSession) postgresMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	columnRows, err := s.sqlDB.QueryContext(ctx, `
SELECT table_schema, table_name, column_name, data_type, is_nullable
FROM information_schema.columns
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY table_schema, table_name, ordinal_position`)
	if err != nil {
		return nil, err
	}
	columns := map[string][]webDataMetadataNode{}
	for columnRows.Next() {
		var schema, table, column, dataType, nullable string
		if err := columnRows.Scan(&schema, &table, &column, &dataType, &nullable); err != nil {
			_ = columnRows.Close()
			return nil, err
		}
		value := dataType
		if nullable == "NO" {
			value += " not null"
		}
		key := schema + "\x00" + table
		columns[key] = append(columns[key], webDataMetadataNode{
			Key:   "postgres-column-" + schema + "-" + table + "-" + column,
			Title: column,
			Type:  "column",
			Value: value,
			Meta:  map[string]string{"schema": schema, "name": table, "column": column},
		})
	}
	if err := columnRows.Close(); err != nil {
		return nil, err
	}
	if err := columnRows.Err(); err != nil {
		return nil, err
	}

	rows, err := s.sqlDB.QueryContext(ctx, `
SELECT table_schema, table_name
FROM information_schema.tables
WHERE table_schema NOT IN ('pg_catalog', 'information_schema')
ORDER BY table_schema, table_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	schemas := map[string][]string{}
	for rows.Next() {
		var schema, table string
		if err := rows.Scan(&schema, &table); err != nil {
			return nil, err
		}
		schemas[schema] = append(schemas[schema], table)
	}
	root := webDataMetadataNode{Key: "postgres-schemas", Title: "Schemas", Type: "root"}
	names := sortedMapStringKeys(schemas)
	for _, schema := range names {
		node := webDataMetadataNode{Key: "postgres-schema-" + schema, Title: schema, Type: "schema", Meta: map[string]string{"schema": schema}}
		for _, table := range schemas[schema] {
			node.Children = append(node.Children, webDataMetadataNode{
				Key:      "postgres-table-" + schema + "-" + table,
				Title:    table,
				Type:     "table",
				Meta:     map[string]string{"schema": schema, "name": table},
				Children: columns[schema+"\x00"+table],
			})
		}
		root.Children = append(root.Children, node)
	}
	return []webDataMetadataNode{root}, rows.Err()
}

func (s *webDataSession) redisMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	root := webDataMetadataNode{Key: "redis-root", Title: "Redis", Type: "root"}
	if info, err := s.redisClient.Info(ctx, "keyspace").Result(); err == nil {
		keyspace := webDataMetadataNode{Key: "redis-keyspace", Title: "Keyspace", Type: "group"}
		for _, line := range strings.Split(info, "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "db") {
				keyspace.Children = append(keyspace.Children, webDataMetadataNode{Key: "redis-" + line, Title: line, Type: "database"})
			}
		}
		root.Children = append(root.Children, keyspace)
	}
	keys, _, err := s.redisClient.Scan(ctx, 0, "*", 100).Result()
	if err != nil {
		return []webDataMetadataNode{root}, nil
	}
	keyNode := webDataMetadataNode{Key: "redis-keys", Title: "Keys", Type: "group"}
	for _, key := range keys {
		typ := s.redisClient.Type(ctx, key).Val()
		ttl := s.redisClient.TTL(ctx, key).Val()
		value := typ
		if ttl >= 0 {
			value = fmt.Sprintf("%s ttl=%s", typ, ttl)
		}
		keyNode.Children = append(keyNode.Children, webDataMetadataNode{
			Key:   "redis-key-" + key,
			Title: key,
			Type:  "key",
			Value: value,
			Meta:  map[string]string{"key": key},
		})
	}
	root.Children = append(root.Children, keyNode)
	return []webDataMetadataNode{root}, nil
}

func (s *webDataSession) mongoMetadata(ctx context.Context) ([]webDataMetadataNode, error) {
	root := webDataMetadataNode{Key: "mongo-databases", Title: "Databases", Type: "root"}
	dbNames, err := s.mongoClient.ListDatabaseNames(ctx, bson.D{})
	if err != nil {
		db := s.database
		if db == "" {
			db = s.authDatabase
		}
		if db == "" {
			db = "admin"
		}
		dbNames = []string{db}
	}
	for _, dbName := range dbNames {
		node := webDataMetadataNode{Key: "mongo-db-" + dbName, Title: dbName, Type: "database", Meta: map[string]string{"database": dbName}}
		if names, err := s.mongoClient.Database(dbName).ListCollectionNames(ctx, bson.D{}); err == nil {
			for _, name := range names {
				node.Children = append(node.Children, webDataMetadataNode{
					Key:   "mongo-coll-" + dbName + "-" + name,
					Title: name,
					Type:  "collection",
					Meta:  map[string]string{"database": dbName, "name": name},
				})
			}
		}
		root.Children = append(root.Children, node)
	}
	return []webDataMetadataNode{root}, nil
}

func (s *webDataSession) objectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	switch s.protocol {
	case "mysql":
		return s.mysqlObjectDetails(ctx, req)
	case "postgresql":
		return s.postgresObjectDetails(ctx, req)
	case "redis":
		return s.redisObjectDetails(ctx, req)
	case "mongodb":
		return s.mongoObjectDetails(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported protocol %q", s.protocol)
	}
}

func (s *webDataSession) mysqlObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "table" {
		return &webDataObjectResponse{ObjectType: req.ObjectType, Message: "请选择表查看详情"}, nil
	}
	database := firstNonEmpty(req.Database, s.database)
	if database == "" || req.Name == "" {
		return nil, errors.New("表名和数据库不能为空")
	}
	fullName := mysqlQuoteIdent(database) + "." + mysqlQuoteIdent(req.Name)
	columns, err := querySQLRowsAsMaps(ctx, s.sqlDB, "SHOW FULL COLUMNS FROM "+fullName)
	if err != nil {
		return nil, err
	}
	indexes, _ := querySQLRowsAsMaps(ctx, s.sqlDB, "SHOW INDEX FROM "+fullName)
	ddl := ""
	if rows, err := querySQLRowsAsMaps(ctx, s.sqlDB, "SHOW CREATE TABLE "+fullName); err == nil && len(rows) > 0 {
		for key, value := range rows[0] {
			if strings.Contains(strings.ToLower(key), "create table") {
				ddl = fmt.Sprint(value)
				break
			}
		}
	}
	return &webDataObjectResponse{
		ObjectType: "table",
		Database:   database,
		Name:       req.Name,
		DDL:        ddl,
		Columns:    columns,
		Indexes:    indexes,
		Message:    fmt.Sprintf("%s.%s", database, req.Name),
	}, nil
}

func (s *webDataSession) postgresObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "table" {
		return &webDataObjectResponse{ObjectType: req.ObjectType, Message: "请选择表查看详情"}, nil
	}
	schema := firstNonEmpty(req.Schema, "public")
	if req.Name == "" {
		return nil, errors.New("表名不能为空")
	}
	columns, err := querySQLRowsAsMaps(ctx, s.sqlDB, `
SELECT column_name, data_type, is_nullable, column_default, character_maximum_length, numeric_precision, numeric_scale
FROM information_schema.columns
WHERE table_schema = $1 AND table_name = $2
ORDER BY ordinal_position`, schema, req.Name)
	if err != nil {
		return nil, err
	}
	indexes, _ := querySQLRowsAsMaps(ctx, s.sqlDB, `
SELECT indexname, indexdef
FROM pg_indexes
WHERE schemaname = $1 AND tablename = $2
ORDER BY indexname`, schema, req.Name)
	ddlParts := make([]string, 0, len(indexes))
	for _, index := range indexes {
		if def, ok := index["indexdef"]; ok {
			ddlParts = append(ddlParts, fmt.Sprint(def)+";")
		}
	}
	return &webDataObjectResponse{
		ObjectType: "table",
		Schema:     schema,
		Name:       req.Name,
		DDL:        strings.Join(ddlParts, "\n"),
		Columns:    columns,
		Indexes:    indexes,
		Message:    fmt.Sprintf("%s.%s", schema, req.Name),
	}, nil
}

func (s *webDataSession) redisObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	key := firstNonEmpty(req.Key, req.Name)
	if key == "" {
		return nil, errors.New("Redis key 不能为空")
	}
	typ, err := s.redisClient.Type(ctx, key).Result()
	if err != nil {
		return nil, err
	}
	ttl, _ := s.redisClient.TTL(ctx, key).Result()
	extra := []map[string]any{
		{"property": "type", "value": typ},
		{"property": "ttl", "value": formatRedisTTL(ttl)},
	}
	if size, err := s.redisClient.MemoryUsage(ctx, key).Result(); err == nil {
		extra = append(extra, map[string]any{"property": "memory", "value": size})
	}
	return &webDataObjectResponse{
		ObjectType: "key",
		Key:        key,
		Message:    fmt.Sprintf("%s · %s · %s", key, typ, formatRedisTTL(ttl)),
		Extra:      extra,
	}, nil
}

func (s *webDataSession) mongoObjectDetails(ctx context.Context, req webDataObjectRequest) (*webDataObjectResponse, error) {
	if req.ObjectType != "collection" {
		return &webDataObjectResponse{ObjectType: req.ObjectType, Message: "请选择集合查看详情"}, nil
	}
	database := firstNonEmpty(req.Database, s.database, s.authDatabase, "admin")
	if req.Name == "" {
		return nil, errors.New("集合名不能为空")
	}
	collection := s.mongoClient.Database(database).Collection(req.Name)
	indexRows := []map[string]any{}
	if cursor, err := collection.Indexes().List(ctx); err == nil {
		defer cursor.Close(ctx)
		for cursor.Next(ctx) {
			var item bson.M
			if err := cursor.Decode(&item); err != nil {
				return nil, err
			}
			indexRows = append(indexRows, bsonMapToPlainMap(item))
		}
		if err := cursor.Err(); err != nil {
			return nil, err
		}
	}
	return &webDataObjectResponse{
		ObjectType: "collection",
		Database:   database,
		Name:       req.Name,
		Columns:    mongoCollectionFieldsFromIndexes(indexRows),
		Indexes:    indexRows,
		Message:    fmt.Sprintf("%s.%s", database, req.Name),
	}, nil
}

func mongoCollectionFieldsFromIndexes(indexRows []map[string]any) []map[string]any {
	fields := []map[string]any{}
	seen := map[string]bool{}
	appendField := func(name, source string) {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasPrefix(name, "$") || seen[name] {
			return
		}
		seen[name] = true
		fieldType := "unknown"
		if name == "_id" {
			fieldType = "ObjectId"
		}
		fields = append(fields, map[string]any{"name": name, "type": fieldType, "source": source})
	}
	appendField("_id", "default")
	for _, index := range indexRows {
		source := strings.TrimSpace(fmt.Sprint(index["name"]))
		if source == "" {
			source = "index"
		}
		for _, field := range mongoIndexFieldNames(index["key"]) {
			appendField(field, source)
		}
	}
	return fields
}

func mongoIndexFieldNames(value any) []string {
	names := []string{}
	switch typed := value.(type) {
	case map[string]any:
		names = append(names, sortedMapKeys(typed)...)
	case map[string]float64:
		for key := range typed {
			names = append(names, key)
		}
		sort.Strings(names)
	case bson.M:
		names = append(names, sortedMapKeys(map[string]any(typed))...)
	case bson.D:
		for _, item := range typed {
			names = append(names, item.Key)
		}
	}
	return names
}

func readSQLRows(rows *sql.Rows) (*webDataExecuteResponse, error) {
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := &webDataExecuteResponse{Type: "rows", Columns: columns, Rows: []map[string]any{}}
	values := make([]any, len(columns))
	ptrs := make([]any, len(columns))
	for i := range values {
		ptrs[i] = &values[i]
	}
	for rows.Next() {
		if len(result.Rows) >= webDataResultRowLimit {
			result.Truncated = true
			break
		}
		if err := rows.Scan(ptrs...); err != nil {
			return result, err
		}
		row := map[string]any{}
		for i, column := range columns {
			row[column] = normalizeDBValue(values[i])
		}
		result.Rows = append(result.Rows, row)
	}
	if err := rows.Err(); err != nil {
		return result, err
	}
	result.Message = fmt.Sprintf("%d row(s)", len(result.Rows))
	return result, nil
}

func querySingleColumn(ctx context.Context, db *sql.DB, query string) ([]string, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var values []string
	for rows.Next() {
		var value string
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, rows.Err()
}

func querySQLRowsAsMaps(ctx context.Context, db *sql.DB, query string, args ...any) ([]map[string]any, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result, err := readSQLRows(rows)
	if err != nil {
		return nil, err
	}
	return result.Rows, nil
}

func normalizeDBValue(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case []byte:
		return string(v)
	case time.Time:
		return v.Format(time.RFC3339Nano)
	default:
		return v
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}

func mysqlQuoteIdent(value string) string {
	return "`" + strings.ReplaceAll(value, "`", "``") + "`"
}

func postgresQuoteIdent(value string) string {
	return `"` + strings.ReplaceAll(value, `"`, `""`) + `"`
}

func redisValueToRows(args []any, value any, redisErr error) ([]map[string]any, []string, string) {
	if errors.Is(redisErr, redis.Nil) {
		return []map[string]any{{"value": nil}}, []string{"value"}, "(nil)"
	}
	switch v := value.(type) {
	case []any:
		if rows, ok := redisZSetScoreRows(args, v); ok {
			return rows, []string{"index", "member", "score"}, fmt.Sprintf("%d member(s)", len(rows))
		}
		rows := make([]map[string]any, 0, len(v))
		for i, item := range v {
			rows = append(rows, map[string]any{"index": i, "value": redisPlainValue(item)})
		}
		return rows, []string{"index", "value"}, fmt.Sprintf("%d item(s)", len(rows))
	case map[string]any:
		rows := make([]map[string]any, 0, len(v))
		for _, key := range sortedMapKeys(v) {
			rows = append(rows, map[string]any{"key": key, "value": redisPlainValue(v[key])})
		}
		return rows, []string{"key", "value"}, fmt.Sprintf("%d item(s)", len(rows))
	case map[any]any:
		rows := redisMapValueToRows(v)
		return rows, []string{"key", "value"}, fmt.Sprintf("%d item(s)", len(rows))
	case map[string]string:
		rows := make([]map[string]any, 0, len(v))
		keys := make([]string, 0, len(v))
		for key := range v {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			rows = append(rows, map[string]any{"key": key, "value": v[key]})
		}
		return rows, []string{"key", "value"}, fmt.Sprintf("%d item(s)", len(rows))
	default:
		return []map[string]any{{"value": redisPlainValue(v)}}, []string{"value"}, "OK"
	}
}

func redisZSetScoreRows(args []any, values []any) ([]map[string]any, bool) {
	command := redisCommandName(args)
	if command != "ZRANGE" && command != "ZREVRANGE" && command != "ZRANGEBYSCORE" && command != "ZREVRANGEBYSCORE" {
		return nil, false
	}
	if !redisArgsContain(args, "WITHSCORES") {
		return nil, false
	}
	rows := []map[string]any{}
	if len(values) == 0 {
		return rows, true
	}
	nestedPairs := true
	for _, item := range values {
		pair, ok := item.([]any)
		if !ok || len(pair) != 2 {
			nestedPairs = false
			break
		}
	}
	if nestedPairs {
		for i, item := range values {
			pair := item.([]any)
			rows = append(rows, map[string]any{"index": i, "member": redisPlainValue(pair[0]), "score": redisPlainValue(pair[1])})
		}
		return rows, true
	}
	if len(values)%2 != 0 {
		return nil, false
	}
	for i := 0; i < len(values); i += 2 {
		rows = append(rows, map[string]any{
			"index":  i / 2,
			"member": redisPlainValue(values[i]),
			"score":  redisPlainValue(values[i+1]),
		})
	}
	return rows, true
}

func redisCommandName(args []any) string {
	if len(args) == 0 {
		return ""
	}
	return strings.ToUpper(fmt.Sprint(args[0]))
}

func redisArgsContain(args []any, value string) bool {
	for _, arg := range args {
		if strings.EqualFold(fmt.Sprint(arg), value) {
			return true
		}
	}
	return false
}

func redisMapValueToRows(value map[any]any) []map[string]any {
	rows := make([]map[string]any, 0, len(value))
	keys := make([]string, 0, len(value))
	index := map[string]any{}
	for key := range value {
		text := fmt.Sprint(redisPlainValue(key))
		keys = append(keys, text)
		index[text] = key
	}
	sort.Strings(keys)
	for _, text := range keys {
		rows = append(rows, map[string]any{
			"key":   text,
			"value": redisPlainValue(value[index[text]]),
		})
	}
	return rows
}

func redisPlainValue(value any) any {
	switch v := value.(type) {
	case []byte:
		return string(v)
	case []any:
		items := make([]any, len(v))
		for i, item := range v {
			items[i] = redisPlainValue(item)
		}
		return items
	case map[any]any:
		out := map[string]any{}
		for key, value := range v {
			out[fmt.Sprint(redisPlainValue(key))] = redisPlainValue(value)
		}
		return out
	case map[string]any:
		out := map[string]any{}
		for key, value := range v {
			out[key] = redisPlainValue(value)
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for key, value := range v {
			out[key] = value
		}
		return out
	default:
		return v
	}
}

func formatRedisTTL(ttl time.Duration) string {
	switch {
	case ttl == -2*time.Second || ttl == -2*time.Nanosecond:
		return "not found"
	case ttl < 0:
		return "no expire"
	default:
		return ttl.String()
	}
}

func bsonMapToPlainMap(value bson.M) map[string]any {
	data, err := bson.MarshalExtJSON(value, false, false)
	if err != nil {
		return map[string]any{"value": fmt.Sprint(value)}
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return map[string]any{"value": string(data)}
	}
	return out
}

func parseRedisCommand(input string) ([]any, error) {
	var parts []any
	var current strings.Builder
	var quote rune
	escaped := false
	for _, r := range input {
		if escaped {
			current.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' && quote != 0 {
			escaped = true
			continue
		}
		if quote != 0 {
			if r == quote {
				quote = 0
			} else {
				current.WriteRune(r)
			}
			continue
		}
		if r == '\'' || r == '"' {
			quote = r
			continue
		}
		if unicode.IsSpace(r) {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
			continue
		}
		current.WriteRune(r)
	}
	if escaped || quote != 0 {
		return nil, errors.New("命令引号或转义不完整")
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	if len(parts) == 0 {
		return nil, errors.New("请输入 Redis 命令")
	}
	return parts, nil
}

func sqlStatementReturnsRows(statement string) bool {
	first := strings.ToLower(firstSQLWord(statement))
	if strings.Contains(strings.ToLower(statement), " returning ") {
		return true
	}
	switch first {
	case "select", "show", "describe", "desc", "explain", "with", "values", "table":
		return true
	default:
		return false
	}
}

func firstSQLWord(statement string) string {
	statement = strings.TrimSpace(statement)
	statement = strings.TrimPrefix(statement, "\ufeff")
	for strings.HasPrefix(statement, "--") || strings.HasPrefix(statement, "/*") {
		if strings.HasPrefix(statement, "--") {
			idx := strings.IndexByte(statement, '\n')
			if idx < 0 {
				return ""
			}
			statement = strings.TrimSpace(statement[idx+1:])
			continue
		}
		idx := strings.Index(statement, "*/")
		if idx < 0 {
			return ""
		}
		statement = strings.TrimSpace(statement[idx+2:])
	}
	for i, r := range statement {
		if unicode.IsSpace(r) || r == '(' || r == ';' {
			return statement[:i]
		}
	}
	return statement
}

func sortedMapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func unionRowKeys(rows []map[string]any) []string {
	seen := map[string]bool{}
	keys := []string{}
	for _, row := range rows {
		for key := range row {
			if !seen[key] {
				seen[key] = true
				keys = append(keys, key)
			}
		}
	}
	sort.Strings(keys)
	return keys
}

func sortedMapStringKeys(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func sortedMapStringValueKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizeAffectedRows(rows int64) int64 {
	if rows < 0 {
		return 0
	}
	return rows
}

func zeroBytes(value []byte) {
	for i := range value {
		value[i] = 0
	}
}

func webDataCapabilities(protocol string) []string {
	switch protocol {
	case "redis":
		return []string{"execute", "metadata", "object_detail", "key_scan", "key_preview"}
	case "mongodb":
		return []string{"execute", "metadata", "object_detail", "json_command", "collection_preview"}
	default:
		return []string{"execute", "metadata", "object_detail", "sql", "table_preview", "ddl"}
	}
}

func webDataStatementPreview(statement string) string {
	statement = strings.Join(strings.Fields(statement), " ")
	if len(statement) > 512 {
		return statement[:512]
	}
	return statement
}

func webDataStatementHash(statement string) string {
	sum := sha256.Sum256([]byte(statement))
	return hex.EncodeToString(sum[:])
}

func webDataShouldAuditExecute(protocol, statement string) bool {
	return !webDataExecuteIsQuery(protocol, statement)
}

func webDataExecuteIsQuery(protocol, statement string) bool {
	switch normalizeWebDataProtocol(protocol) {
	case "mysql", "postgresql":
		return webDataSQLIsQuery(statement)
	case "redis":
		return webDataRedisIsQuery(statement)
	case "mongodb":
		return webDataMongoIsQuery(statement)
	default:
		return false
	}
}

func webDataSQLIsQuery(statement string) bool {
	statements := splitSQLStatementsForAudit(statement)
	if len(statements) == 0 {
		return true
	}
	for _, item := range statements {
		if !webDataSingleSQLIsQuery(item) {
			return false
		}
	}
	return true
}

func webDataSingleSQLIsQuery(statement string) bool {
	first := strings.ToLower(firstSQLWord(statement))
	switch first {
	case "select", "show", "describe", "desc", "explain", "values", "table":
		if first == "explain" && webDataExplainExecutesWrite(statement) {
			return false
		}
		return true
	case "with":
		return !containsSQLWriteKeyword(statement)
	default:
		return false
	}
}

func webDataExplainExecutesWrite(statement string) bool {
	words := sqlWordsOutsideLiterals(statement)
	if len(words) == 0 || strings.ToLower(words[0]) != "explain" {
		return false
	}
	hasAnalyze := false
	for _, word := range words[1:] {
		word = strings.ToLower(word)
		if word == "analyze" || word == "analyse" {
			hasAnalyze = true
			continue
		}
		if hasAnalyze && isSQLWriteKeyword(word) {
			return true
		}
	}
	return false
}

func splitSQLStatementsForAudit(statement string) []string {
	statements := []string{}
	start := 0
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	dollarQuote := ""
	for i := 0; i < len(statement); i++ {
		ch := statement[i]
		next := byte(0)
		if i+1 < len(statement) {
			next = statement[i+1]
		}
		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if dollarQuote != "" {
			if strings.HasPrefix(statement[i:], dollarQuote) {
				i += len(dollarQuote) - 1
				dollarQuote = ""
			}
			continue
		}
		if inSingleQuote {
			if ch == '\\' && next != 0 {
				i++
				continue
			}
			if ch == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inSingleQuote = false
			}
			continue
		}
		if inDoubleQuote {
			if ch == '"' {
				if next == '"' {
					i++
					continue
				}
				inDoubleQuote = false
			}
			continue
		}
		if inBacktick {
			if ch == '`' {
				if next == '`' {
					i++
					continue
				}
				inBacktick = false
			}
			continue
		}
		switch {
		case ch == '-' && next == '-':
			inLineComment = true
			i++
		case ch == '#':
			inLineComment = true
		case ch == '/' && next == '*':
			inBlockComment = true
			i++
		case ch == '\'':
			inSingleQuote = true
		case ch == '"':
			inDoubleQuote = true
		case ch == '`':
			inBacktick = true
		case ch == '$':
			if delimiter := sqlDollarQuoteDelimiter(statement[i:]); delimiter != "" {
				dollarQuote = delimiter
				i += len(delimiter) - 1
			}
		case ch == ';':
			if item := strings.TrimSpace(statement[start:i]); item != "" {
				statements = append(statements, item)
			}
			start = i + 1
		}
	}
	if item := strings.TrimSpace(statement[start:]); item != "" {
		statements = append(statements, item)
	}
	return statements
}

func sqlDollarQuoteDelimiter(value string) string {
	if !strings.HasPrefix(value, "$") {
		return ""
	}
	for i := 1; i < len(value); i++ {
		ch := value[i]
		if ch == '$' {
			return value[:i+1]
		}
		if !(ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9') {
			return ""
		}
	}
	return ""
}

func containsSQLWriteKeyword(statement string) bool {
	for _, keyword := range sqlWriteKeywords() {
		if containsWord(statement, keyword) {
			return true
		}
	}
	return false
}

func isSQLWriteKeyword(word string) bool {
	word = strings.ToLower(strings.TrimSpace(word))
	for _, keyword := range sqlWriteKeywords() {
		if word == keyword {
			return true
		}
	}
	return false
}

func sqlWriteKeywords() []string {
	return []string{
		"insert", "update", "delete", "merge", "create", "alter", "drop",
		"truncate", "replace", "load", "copy", "grant", "revoke", "call",
	}
}

func webDataRedisIsQuery(statement string) bool {
	args, err := parseRedisCommand(statement)
	if err != nil || len(args) == 0 {
		return false
	}
	command, ok := args[0].(string)
	if !ok {
		return false
	}
	switch strings.ToUpper(strings.TrimSpace(command)) {
	case "PING", "AUTH", "SELECT", "READONLY", "READWRITE",
		"GET", "MGET", "GETRANGE", "STRLEN", "EXISTS", "TYPE", "TTL", "PTTL",
		"SCAN", "SSCAN", "HSCAN", "ZSCAN", "RANDOMKEY", "KEYS",
		"HGET", "HMGET", "HGETALL", "HEXISTS", "HLEN", "HKEYS", "HVALS", "HSTRLEN",
		"LLEN", "LINDEX", "LRANGE",
		"SCARD", "SISMEMBER", "SMISMEMBER", "SMEMBERS", "SRANDMEMBER",
		"ZCARD", "ZCOUNT", "ZRANGE", "ZREVRANGE", "ZRANGEBYSCORE", "ZREVRANGEBYSCORE",
		"ZRANGEBYLEX", "ZREVRANGEBYLEX", "ZRANK", "ZREVRANK", "ZSCORE", "ZMSCORE",
		"XINFO", "XLEN", "XRANGE", "XREVRANGE", "XREAD", "XPENDING",
		"DBSIZE", "INFO", "COMMAND":
		return true
	default:
		return false
	}
}

func webDataMongoIsQuery(statement string) bool {
	var command bson.D
	if err := bson.UnmarshalExtJSON([]byte(strings.TrimSpace(statement)), true, &command); err != nil || len(command) == 0 {
		return false
	}
	name := strings.ToLower(strings.TrimSpace(command[0].Key))
	if name == "aggregate" {
		lower := strings.ToLower(statement)
		return !strings.Contains(lower, `"$out"`) && !strings.Contains(lower, `"$merge"`)
	}
	switch name {
	case "find", "count", "distinct", "listcollections", "listindexes",
		"dbstats", "collstats", "ping", "hello", "ismaster", "buildinfo", "serverstatus":
		return true
	default:
		return false
	}
}

func containsWord(text, word string) bool {
	text = strings.ToLower(text)
	word = strings.ToLower(word)
	start := 0
	for {
		idx := strings.Index(text[start:], word)
		if idx < 0 {
			return false
		}
		idx += start
		beforeOK := idx == 0 || !isWordRune(rune(text[idx-1]))
		afterIdx := idx + len(word)
		afterOK := afterIdx >= len(text) || !isWordRune(rune(text[afterIdx]))
		if beforeOK && afterOK {
			return true
		}
		start = afterIdx
	}
}

func isWordRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func (web *web) recordWebDataAudit(r *http.Request, target *controlplane.WebDataTarget, userID uint, action, protocol, database, statement string, success bool, affectedRows, elapsedMS int64, errText string) {
	if target == nil || userID == 0 {
		return
	}
	action = strings.TrimSpace(action)
	if action == "" {
		action = "execute"
	}
	protocol = normalizeWebDataProtocol(protocol)
	if protocol == "" {
		protocol = target.Protocol
	}
	statement = strings.TrimSpace(statement)
	if statement == "" {
		statement = webDataAuditStatement(action)
	}
	clientIP, clientIPSource := remoteClientIPInfo(r)
	if err := web.controlPlane.RecordWebDataAudit(r.Context(), &controlplane.WebDataAudit{
		UserID:           userID,
		ProxyID:          target.ProxyID,
		ApplicationID:    target.ApplicationID,
		Protocol:         protocol,
		Action:           action,
		Database:         strings.TrimSpace(database),
		StatementPreview: webDataStatementPreview(statement),
		StatementSHA256:  webDataStatementHash(statement),
		Success:          success,
		AffectedRows:     affectedRows,
		Error:            errText,
		ElapsedMS:        elapsedMS,
		ClientIP:         clientIP,
		Details:          map[string]any{"client_ip_source": clientIPSource},
	}); err != nil {
		log.Warnf("webdata audit record failed: proxy_id=%d user_id=%d action=%s err=%v", target.ProxyID, userID, action, err)
	}
}

func webDataAuditStatement(action string) string {
	switch strings.TrimSpace(action) {
	case "test_connection":
		return "TEST CONNECTION"
	case "open_session":
		return "OPEN SESSION"
	case "save_credential":
		return "SAVE CONNECTION"
	case "delete_credential":
		return "DELETE CONNECTION"
	default:
		return strings.ToUpper(action)
	}
}

func webDataAuditDatabase(req *createWebDataSessionRequest) string {
	if req == nil {
		return ""
	}
	if normalizeWebDataProtocol(req.Protocol) == "redis" {
		return fmt.Sprintf("db%d", req.RedisDB)
	}
	return strings.TrimSpace(req.Database)
}

func webDataAuditCredentialDatabase(req *webDataCredentialRequest) string {
	if req == nil {
		return ""
	}
	if normalizeWebDataProtocol(req.Protocol) == "redis" {
		return fmt.Sprintf("db%d", req.RedisDB)
	}
	return strings.TrimSpace(req.Database)
}

func webDataAuditSessionDatabase(session *webDataSession) string {
	if session == nil {
		return ""
	}
	if normalizeWebDataProtocol(session.protocol) == "redis" {
		return fmt.Sprintf("db%d", session.redisDB)
	}
	return strings.TrimSpace(session.database)
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

func normalizeWebDataSessionRequest(req *createWebDataSessionRequest) {
	if req == nil {
		return
	}
	req.Protocol = normalizeWebDataProtocol(req.Protocol)
	req.Username = strings.TrimSpace(req.Username)
	req.Database = strings.TrimSpace(req.Database)
	req.AuthDatabase = strings.TrimSpace(req.AuthDatabase)
	req.TLSMode = strings.ToLower(strings.TrimSpace(req.TLSMode))
	req.Schema = strings.TrimSpace(req.Schema)
	req.AuthMechanism = strings.ToUpper(strings.TrimSpace(req.AuthMechanism))
	req.ConnectionParams = strings.TrimSpace(req.ConnectionParams)
}

func applyWebDataCredentialDefaults(req *createWebDataSessionRequest, credential *controlplane.WebDataCredentialSecret) {
	if req == nil || credential == nil {
		return
	}
	if req.Protocol == "" {
		req.Protocol = credential.Protocol
	}
	if req.Username == "" {
		req.Username = credential.Username
	}
	if req.Database == "" {
		req.Database = credential.Database
	}
	if req.AuthDatabase == "" {
		req.AuthDatabase = credential.AuthDatabase
	}
	if !req.RedisDBSet {
		req.RedisDB = credential.RedisDB
	}
	if req.TLSMode == "" {
		req.TLSMode = credential.TLSMode
	}
	if req.Schema == "" {
		req.Schema = credential.Schema
	}
	if req.AuthMechanism == "" {
		req.AuthMechanism = credential.AuthMechanism
	}
	if !req.DirectConnectionSet {
		req.DirectConnection = credential.DirectConnection
	}
	if req.ConnectionParams == "" {
		req.ConnectionParams = credential.ConnectionParams
	}
}

func parseWebDataConnectionParams(raw string) (map[string]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	if len(raw) > 4096 {
		return nil, errors.New("高级连接参数过长")
	}
	normalized := strings.NewReplacer("\r\n", "\n", "\r", "\n", ";", "\n", ",", "\n", "&", "\n").Replace(raw)
	params := map[string]string{}
	for _, line := range strings.Split(normalized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return nil, fmt.Errorf("连接参数 %q 需要使用 key=value 格式", line)
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" {
			return nil, errors.New("连接参数 key 不能为空")
		}
		params[key] = value
	}
	return params, nil
}

func webDataTLSConfig(tlsMode string) *tls.Config {
	switch strings.ToLower(strings.TrimSpace(tlsMode)) {
	case "require", "true", "preferred":
		return &tls.Config{}
	case "skip-verify":
		return &tls.Config{InsecureSkipVerify: true} //nolint:gosec // User-selected tunnel-side compatibility mode.
	default:
		return nil
	}
}

func remoteClientIP(r *http.Request) string {
	ip, _ := remoteClientIPInfo(r)
	return ip
}

func remoteClientIPInfo(r *http.Request) (string, string) {
	remoteIP, remoteSource := remoteAddrIP(r.RemoteAddr)
	if trustedForwardedHeaderPeer(remoteIP) {
		if v := headerIP(r.Header.Get("X-Real-IP")); v != "" {
			return v, "x-real-ip"
		}
		if v := headerIP(r.Header.Get("X-Forwarded-For")); v != "" {
			return v, "x-forwarded-for"
		}
	}
	return remoteIP, remoteSource
}

func remoteAddrIP(remoteAddr string) (string, string) {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return "", "remote_addr"
	}
	if host, _, err := net.SplitHostPort(remoteAddr); err == nil {
		return host, "remote_addr"
	}
	return remoteAddr, "remote_addr"
}

func trustedForwardedHeaderPeer(remoteIP string) bool {
	ip := net.ParseIP(strings.TrimSpace(remoteIP))
	return ip != nil && ip.IsLoopback()
}

func headerIP(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if idx := strings.Index(raw, ","); idx > 0 {
		raw = strings.TrimSpace(raw[:idx])
	}
	if ip := net.ParseIP(raw); ip != nil {
		return ip.String()
	}
	return ""
}

func sqlWordsOutsideLiterals(statement string) []string {
	words := []string{}
	var current strings.Builder
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false
	dollarQuote := ""
	flush := func() {
		if current.Len() == 0 {
			return
		}
		words = append(words, current.String())
		current.Reset()
	}
	for i := 0; i < len(statement); i++ {
		ch := statement[i]
		next := byte(0)
		if i+1 < len(statement) {
			next = statement[i+1]
		}
		if inLineComment {
			if ch == '\n' || ch == '\r' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if ch == '*' && next == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if dollarQuote != "" {
			if strings.HasPrefix(statement[i:], dollarQuote) {
				i += len(dollarQuote) - 1
				dollarQuote = ""
			}
			continue
		}
		if inSingleQuote {
			if ch == '\\' && next != 0 {
				i++
				continue
			}
			if ch == '\'' {
				if next == '\'' {
					i++
					continue
				}
				inSingleQuote = false
			}
			continue
		}
		if inDoubleQuote {
			if ch == '"' {
				if next == '"' {
					i++
					continue
				}
				inDoubleQuote = false
			}
			continue
		}
		if inBacktick {
			if ch == '`' {
				if next == '`' {
					i++
					continue
				}
				inBacktick = false
			}
			continue
		}
		switch {
		case ch == '-' && next == '-':
			flush()
			inLineComment = true
			i++
		case ch == '#':
			flush()
			inLineComment = true
		case ch == '/' && next == '*':
			flush()
			inBlockComment = true
			i++
		case ch == '\'':
			flush()
			inSingleQuote = true
		case ch == '"':
			flush()
			inDoubleQuote = true
		case ch == '`':
			flush()
			inBacktick = true
		case ch == '$':
			if delimiter := sqlDollarQuoteDelimiter(statement[i:]); delimiter != "" {
				flush()
				dollarQuote = delimiter
				i += len(delimiter) - 1
				continue
			}
			flush()
		case ch == '_' || ch >= 'a' && ch <= 'z' || ch >= 'A' && ch <= 'Z' || ch >= '0' && ch <= '9':
			current.WriteByte(ch)
		default:
			flush()
		}
	}
	flush()
	return words
}

func parseWebDataProxyID(r *http.Request, suffix string) (uint, error) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webdata/proxies/")
	if suffix != "" {
		if !strings.HasSuffix(path, suffix) {
			return 0, errors.New("invalid proxy id")
		}
		path = strings.TrimSuffix(path, suffix)
	} else if strings.Contains(path, "/") {
		return 0, errors.New("invalid proxy id")
	}
	id, err := strconv.ParseUint(strings.Trim(path, "/"), 10, 32)
	if err != nil || id == 0 {
		return 0, errors.New("invalid proxy id")
	}
	return uint(id), nil
}

func parseWebDataSessionToken(r *http.Request, suffix string) (string, error) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/webdata/sessions/")
	if suffix != "" {
		if !strings.HasSuffix(path, suffix) {
			return "", errors.New("invalid session")
		}
		path = strings.TrimSuffix(path, suffix)
	}
	token := strings.Trim(path, "/")
	if token == "" || strings.Contains(token, "/") {
		return "", errors.New("invalid session")
	}
	return token, nil
}

func webDataHTTPStatus(err error) int {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return http.StatusNotFound
	}
	msg := err.Error()
	if strings.Contains(msg, "仅 MySQL") || strings.Contains(msg, "不能为空") || strings.Contains(msg, "无效") {
		return http.StatusBadRequest
	}
	if strings.Contains(msg, "不可用") || strings.Contains(msg, "离线") || strings.Contains(msg, "禁用") {
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}
