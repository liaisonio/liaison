package dao

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Dao 接口定义
type Dao interface {
	GetAIApplication(context.Context, uint) (*model.AIApplication, error)
	SaveAIApplication(context.Context, *model.AIApplication) error
	GetAIAccess(context.Context, uint) (*model.AIAccess, error)
	SaveAIAccess(context.Context, *model.AIAccess) error
	CreateAIKey(context.Context, *model.AIKey) error
	GetAIKey(context.Context, string) (*model.AIKey, error)
	ListAIKeys(context.Context, uint, uint) ([]model.AIKey, error)
	GetAIKeyUsage(context.Context, uint, uint, uint) (*model.AIKeyUsage, error)
	UpdateAIKeyQuota(context.Context, uint, uint, uint, *int64) error
	RevokeAIKey(context.Context, uint, uint, uint) error
	RecordAIRequest(context.Context, *model.AIRequest) error
	GetLLMTokenUsage(context.Context, uint, uint, time.Time) (*model.LLMTokenUsageReport, error)
	ListAIRequests(context.Context, uint, uint, int) ([]model.AIRequest, error)
	ReadAgentModelConfig(context.Context) ([]byte, error)
	WriteAgentModelConfig(context.Context, []byte) error
	// 事务相关方法
	Begin() Dao
	Commit() error
	Rollback() error

	// Edge 相关方法
	GetEdge(id uint64) (*model.Edge, error)
	GetEdgeByAccessKey(accessKey string) (*model.AccessKey, *model.Edge, error)
	CreateEdge(edge *model.Edge) error
	GetEdgeByDeviceID(deviceID uint) (*model.Edge, error)
	ListEdges(query *ListEdgesQuery) ([]*model.Edge, error)
	CountEdges(query *ListEdgesQuery) (int64, error)
	UpdateEdge(edge *model.Edge) error
	UpdateEdgeOnlineStatus(edgeID uint64, onlineStatus model.EdgeOnlineStatus) error
	UpdateEdgeHeartbeatAt(edgeID uint64, heartbeatAt time.Time) error
	UpdateEdgeDeviceID(edgeID uint64, deviceID uint) error
	DeleteEdge(id uint64) error

	// EdgeDevice 相关方法
	CreateEdgeDevice(edgeDevice *model.EdgeDevice) error
	GetEdgeDevice(edgeID uint64, deviceID uint, relationType model.EdgeDeviceRelationType) (*model.EdgeDevice, error)
	GetEdgeDevicesByEdgeID(edgeID uint64, relationType *model.EdgeDeviceRelationType) ([]*model.EdgeDevice, error)
	GetEdgeDevicesByDeviceID(deviceID uint, relationType *model.EdgeDeviceRelationType) ([]*model.EdgeDevice, error)
	DeleteEdgeDevice(edgeID uint64, deviceID uint, relationType model.EdgeDeviceRelationType) error
	DeleteEdgeDevicesByEdgeID(edgeID uint64, relationType *model.EdgeDeviceRelationType) error
	DeleteEdgeDevicesByDeviceID(deviceID uint, relationType *model.EdgeDeviceRelationType) error

	// AccessKey 相关方法
	CreateAccessKey(accessKey *model.AccessKey) error
	GetAccessKeyByID(id uint) (*model.AccessKey, error)
	DeleteAccessKeysByEdgeID(edgeID uint64) error

	// Device 相关方法
	CreateDevice(device *model.Device) error
	CreateEthernetInterface(iface *model.EthernetInterface) error
	GetEthernetInterface(deviceID uint, ip, netmask, name, mac string) (*model.EthernetInterface, error)
	GetEthernetInterfacesByDeviceID(deviceID uint) ([]*model.EthernetInterface, error)
	UpdateEthernetInterface(iface *model.EthernetInterface) error
	DeleteEthernetInterface(id uint) error
	GetDeviceByID(id uint) (*model.Device, error)
	GetDeviceByFingerprint(fingerprint string) (*model.Device, error)
	GetDeviceByIP(ip string) (*model.Device, error)
	UpdateDeviceHeartbeat(deviceID uint) error
	ListDevices(query *ListDevicesQuery) ([]*model.Device, error)
	CountDevices(query *ListDevicesQuery) (int64, error)
	UpdateDevice(device *model.Device) error
	DeleteDevice(id uint) error
	UpdateDeviceUsage(deviceID uint, cpuUsage, memoryUsage, diskUsage float32) error

	// Application 相关方法
	CreateApplication(application *model.Application) error
	GetApplicationByID(id uint) (*model.Application, error)
	ListApplications(query *ListApplicationsQuery) ([]*model.Application, error)
	CountApplications(query *ListApplicationsQuery) (int64, error)
	UpdateApplication(application *model.Application) error
	DeleteApplication(id uint) error

	// Proxy 相关方法
	CreateProxy(proxy *model.Proxy) error
	GetProxyByID(id uint) (*model.Proxy, error)
	ListProxies(query *ListProxiesQuery) ([]*model.Proxy, error)
	CountProxies(query *ListProxiesQuery) (int64, error)
	UpdateProxy(proxy *model.Proxy) error
	DeleteProxy(id uint) error

	// Task 相关方法
	CreateTask(task *model.Task) error
	GetTask(taskID uint) (*model.Task, error)
	GetTaskByEdgeID(edgeID uint64) (*model.Task, error)
	ListTasks(query *ListTasksQuery) ([]*model.Task, error)
	UpdateTaskStatus(taskID uint, status model.TaskStatus) error
	UpdateTaskResult(taskID uint, status model.TaskStatus, result []byte) error
	UpdateTaskError(taskID uint, error string) error

	// User 相关方法
	CreateUser(user *model.User) error
	GetUserByID(id uint) (*model.User, error)
	GetUserByEmail(email string) (*model.User, error)
	UpdateUser(user *model.User) error
	UpdateUserLastLogin(userID uint) error
	UpdateUserLastLoginAndIP(userID uint, loginIP string) error
	ListUsers(offset, limit int) ([]*model.User, int64, error)
	DeleteUser(id uint) error
	CheckUserExists(email string) (bool, error)

	// Organization / membership related methods.
	CreateOrganization(organization *model.Organization) error
	GetOrganizationByID(id uint) (*model.Organization, error)
	GetOrganizationByName(name string) (*model.Organization, error)
	ListOrganizations() ([]*model.Organization, error)
	UpdateOrganization(organization *model.Organization) error
	DeleteOrganization(id uint) error
	CountOrganizationChildren(id uint) (int64, error)
	UpsertOrganizationMembership(membership *model.OrganizationMembership) error
	GetOrganizationMembership(organizationID, userID uint) (*model.OrganizationMembership, error)
	ListOrganizationMembers(organizationID uint) ([]*model.OrganizationMembership, error)
	ListUserOrganizations(userID uint) ([]*model.OrganizationMembership, error)
	DeleteOrganizationMembership(organizationID, userID uint) error
	DeleteOrganizationMembershipsByOrganization(organizationID uint) error
	DeleteOrganizationMembershipsByUser(userID uint) error
	GetRootOrganization() (*model.Organization, error)
	UpsertIAMRole(role *model.IAMRole) error
	GetIAMRoleByCode(code model.IAMRoleCode) (*model.IAMRole, error)
	ListIAMRoles() ([]*model.IAMRole, error)
	UpsertIAMPermission(permission *model.IAMPermission) error
	GetIAMPermissionByCode(code string) (*model.IAMPermission, error)
	ListIAMPermissionsByRole(roleID uint) ([]*model.IAMPermission, error)
	UpsertIAMRolePermission(rolePermission *model.IAMRolePermission) error
	ReplaceIAMRolePermissionSubset(roleID uint, subset, enabled []uint) error
	UpsertIAMRoleBinding(binding *model.IAMRoleBinding) error
	GetIAMRoleBinding(organizationID, userID uint) (*model.IAMRoleBinding, error)
	ListIAMRoleBindings() ([]*model.IAMRoleBinding, error)
	ListIAMRoleBindingsByUser(userID uint) ([]*model.IAMRoleBinding, error)
	DeleteIAMRoleBinding(organizationID, userID uint) error
	DeleteIAMRoleBindingsByOrganization(organizationID uint) error
	DeleteIAMRoleBindingsByUser(userID uint) error
	UpsertIAMResourceRelation(relation *model.IAMResourceRelation) error
	ListIAMResourceRelations(resourceType string, resourceID uint64) ([]*model.IAMResourceRelation, error)
	ListIAMResourceIDsForSubject(resourceType string, subjectType model.IAMSubjectType, subjectID uint) ([]uint64, error)
	DeleteIAMResourceRelations(resourceType string, resourceID uint64) error

	// Agent runtime persistence. Updates use optimistic versions so a turn can
	// be resumed safely after process restarts without double execution.
	CreateAgentSession(ctx context.Context, session *model.AgentSession) error
	GetAgentSession(ctx context.Context, id string) (*model.AgentSession, error)
	ListAgentSessions(ctx context.Context, createdBy uint) ([]*model.AgentSession, error)
	UpdateAgentSessionCAS(ctx context.Context, session *model.AgentSession, expectedVersion uint64) (bool, error)
	CreateAgentAttachment(ctx context.Context, attachment *model.AgentAttachment) error
	GetAgentAttachment(ctx context.Context, id string) (*model.AgentAttachment, error)
	ListAgentAttachments(ctx context.Context, sessionID string) ([]*model.AgentAttachment, error)
	UpdateAgentAttachmentCAS(ctx context.Context, attachment *model.AgentAttachment, expectedGeneration uint64) (bool, error)
	CreateAgentTurn(ctx context.Context, turn *model.AgentTurn) error
	GetAgentTurn(ctx context.Context, id string) (*model.AgentTurn, error)
	ListAgentTurns(ctx context.Context, sessionID string) ([]*model.AgentTurn, error)
	UpdateAgentTurnCAS(ctx context.Context, turn *model.AgentTurn, expectedVersion uint64) (bool, error)
	CreateAgentStep(ctx context.Context, step *model.AgentStep) error
	GetAgentStep(ctx context.Context, id string) (*model.AgentStep, error)
	ListAgentSessionSteps(ctx context.Context, sessionID string) ([]*model.AgentStep, error)
	NextAgentStepSequence(ctx context.Context, turnID string) (uint32, error)
	UpdateAgentStepCAS(ctx context.Context, step *model.AgentStep, expectedVersion uint64) (bool, error)
	CreateAgentMessage(ctx context.Context, message *model.AgentMessage) error
	ListAgentMessages(ctx context.Context, turnID string) ([]*model.AgentMessage, error)
	ListAgentSessionMessages(ctx context.Context, sessionID string) ([]*model.AgentMessage, error)
	NextAgentMessageSequence(ctx context.Context, turnID string) (uint32, error)
	CreateAgentApproval(ctx context.Context, approval *model.AgentApproval) error
	GetAgentApproval(ctx context.Context, id string) (*model.AgentApproval, error)
	ListAgentApprovals(ctx context.Context, sessionID string) ([]*model.AgentApproval, error)
	UpdateAgentApprovalCAS(ctx context.Context, approval *model.AgentApproval, expectedStatus uint8) (bool, error)
	CreateAgentToolsetSnapshot(ctx context.Context, snapshot *model.AgentToolsetSnapshot) error
	GetAgentToolsetSnapshot(ctx context.Context, id string) (*model.AgentToolsetSnapshot, error)

	// TrafficMetric 相关方法
	CreateTrafficMetric(metric *model.TrafficMetric) error
	ListTrafficMetrics(query *ListTrafficMetricsQuery) ([]*model.TrafficMetric, error)
	GetTrafficMetricsByTimeRange(startTime, endTime time.Time, applicationIDs []uint) ([]*model.TrafficMetric, error)

	// UserAPIToken (PAT) 相关方法
	CreateUserAPIToken(tok *model.UserAPIToken) error
	ListUserAPITokens(userID uint) ([]*model.UserAPIToken, error)
	GetUserAPITokensByPrefix(prefix string) ([]*model.UserAPIToken, error)
	RevokeUserAPIToken(userID, id uint) error
	TouchUserAPIToken(id uint, ip string) error
	CountUserAPITokens(userID uint) (int64, error)

	// ProxyFirewallRule 相关方法
	GetFirewallRuleByProxyID(proxyID uint) (*model.ProxyFirewallRule, error)
	UpsertFirewallRule(rule *model.ProxyFirewallRule) error
	DeleteFirewallRuleByProxyID(proxyID uint) error
	ListFirewallRulesByUserID(userID uint) ([]*model.ProxyFirewallRule, error)
	ListAllFirewallRules() ([]*model.ProxyFirewallRule, error)

	// WebSSHHostKey 相关方法
	GetWebSSHHostKeyByProxyID(proxyID uint) (*model.WebSSHHostKey, error)
	UpsertWebSSHHostKey(hostKey *model.WebSSHHostKey) error
	DeleteWebSSHHostKeyByProxyID(proxyID uint) error

	// WebSSHCredential 相关方法
	ListWebSSHCredentialsByProxyAndUser(proxyID, userID uint) ([]*model.WebSSHCredential, error)
	GetWebSSHCredential(proxyID, userID uint, username string) (*model.WebSSHCredential, error)
	UpsertWebSSHCredential(credential *model.WebSSHCredential) error
	TouchWebSSHCredential(proxyID, userID uint, username string) error
	DeleteWebSSHCredential(proxyID, userID uint, username string) error
	DeleteWebSSHCredentialByProxyID(proxyID uint) error

	// WebDesktopCredential 相关方法
	ListWebDesktopCredentialsByProxyAndUser(proxyID, userID uint, protocol string) ([]*model.WebDesktopCredential, error)
	GetWebDesktopCredential(proxyID, userID uint, protocol, username, domain string) (*model.WebDesktopCredential, error)
	UpsertWebDesktopCredential(credential *model.WebDesktopCredential) error
	TouchWebDesktopCredential(proxyID, userID uint, protocol, username, domain string) error
	DeleteWebDesktopCredential(proxyID, userID uint, protocol, username, domain string) error
	DeleteWebDesktopCredentialByProxyID(proxyID uint) error

	// WebDataCredential / Audit 相关方法
	ListWebDataCredentialsByProxyAndUser(proxyID, userID uint, protocol string) ([]*model.WebDataCredential, error)
	GetWebDataCredential(proxyID, userID uint, protocol, username, database, authDatabase string) (*model.WebDataCredential, error)
	GetWebDataCredentialByID(id, proxyID, userID uint) (*model.WebDataCredential, error)
	UpsertWebDataCredential(credential *model.WebDataCredential) error
	UpdateWebDataCredential(credential *model.WebDataCredential, updatePassword bool) error
	TouchWebDataCredential(proxyID, userID uint, protocol, username, database, authDatabase string) error
	TouchWebDataCredentialByID(id, proxyID, userID uint) error
	DeleteWebDataCredential(proxyID, userID uint, protocol, username, database, authDatabase string) error
	DeleteWebDataCredentialByID(id, proxyID, userID uint) error
	DeleteWebDataCredentialByProxyID(proxyID uint) error
	CreateWebDataAudit(audit *model.WebDataAudit) error
	ListWebDataAudits(query *ListWebDataAuditsQuery) ([]*model.WebDataAudit, error)
	CountWebDataAudits(query *ListWebDataAuditsQuery) (int64, error)

	// ManagementAudit 相关方法
	CreateManagementAudit(audit *model.ManagementAudit) error
	ListManagementAudits(query *ListManagementAuditsQuery) ([]*model.ManagementAudit, error)
	CountManagementAudits(query *ListManagementAuditsQuery) (int64, error)

	// 资源清理
	Close() error
}

type dao struct {
	db *gorm.DB
	tx *gorm.DB     // 事务对象
	mu sync.RWMutex // 保护事务状态的互斥锁

	// config
	config *config.Configuration
}

func NewDao(config *config.Configuration) (Dao, error) {
	d := &dao{
		config: config,
	}
	db, err := gorm.Open(sqlite.Open(config.Manager.DB), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	d.db = db
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.Exec("PRAGMA synchronous = NORMAL;")
	sqlDB.Exec("PRAGMA journal_mode = WAL;")
	sqlDB.Exec("PRAGMA cache_size = -2000;") // 2MB cache
	sqlDB.Exec("PRAGMA temp_store = MEMORY;")
	sqlDB.Exec("PRAGMA locking_mode = NORMAL;")
	sqlDB.Exec("PRAGMA mmap_size = 268435456;") // 256MB memory map size
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := d.initDB(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *dao) initDB() error {
	if err := d.resetAccessAuditSchema(); err != nil {
		return err
	}
	if err := d.db.AutoMigrate(
		&model.AIApplication{}, &model.AIAccess{}, &model.AIKey{}, &model.AIRequest{},
		&model.LLMTokenUsage{},
		&model.Edge{},
		&model.AccessKey{},
		&model.Device{},
		&model.EthernetInterface{},
		&model.EdgeDevice{},
		&model.Application{},
		&model.Proxy{},
		&model.Task{},
		&model.User{},
		&model.Organization{},
		&model.OrganizationMembership{},
		&model.IAMRole{},
		&model.IAMPermission{},
		&model.IAMRolePermission{},
		&model.IAMRoleBinding{},
		&model.IAMResourceRelation{},
		&model.TrafficMetric{},
		&model.UserAPIToken{},
		&model.ProxyFirewallRule{},
		&model.WebSSHHostKey{},
		&model.WebSSHCredential{},
		&model.WebDesktopCredential{},
		&model.WebDataCredential{},
		&model.WebDataAudit{},
		&model.ManagementAudit{},
		&model.AgentSession{},
		&model.AgentModelSetting{},
		&model.AgentAttachment{},
		&model.AgentTurn{},
		&model.AgentMessage{},
		&model.AgentStep{},
		&model.AgentApproval{},
		&model.AgentToolsetSnapshot{},
	); err != nil {
		return err
	}
	if err := d.backfillProxyAccessProtocols(); err != nil {
		return err
	}
	if err := d.backfillWebSSHAuditProtocols(); err != nil {
		return err
	}
	return d.migrateWebSSHCredentials()
}

func (d *dao) backfillProxyAccessProtocols() error {
	return d.db.Exec(`
		UPDATE proxies
		SET access_protocol = CASE
			WHEN application_id IN (SELECT id FROM applications WHERE application_type = 'http') THEN 'http'
			WHEN port = 0 AND application_id IN (SELECT id FROM applications WHERE application_type = 'ssh') THEN 'webssh'
			WHEN port = 0 THEN 'web'
			ELSE 'tcp'
		END
		WHERE access_protocol IS NULL OR access_protocol = ''
	`).Error
}

func (d *dao) backfillWebSSHAuditProtocols() error {
	// Browser WebSSH audits were historically stored as "ssh". Only migrate
	// rows carrying the WebSSH-only client_ip_source marker so native SSH
	// history is never reclassified by guesswork. The update is idempotent.
	return d.db.Exec(`
		UPDATE access_audits
		SET protocol = 'webssh'
		WHERE protocol = 'ssh'
		  AND user_id > 0
		  AND details LIKE '%"client_ip_source"%'
	`).Error
}

func (d *dao) resetAccessAuditSchema() error {
	if d.db.Migrator().HasTable("webdata_audits") {
		if err := d.db.Migrator().DropTable("webdata_audits"); err != nil {
			return err
		}
	}
	if d.db.Migrator().HasTable("access_audits") {
		hasOldColumns := d.db.Migrator().HasColumn("access_audits", "database_name") ||
			d.db.Migrator().HasColumn("access_audits", "affected_rows") ||
			d.db.Migrator().HasColumn("access_audits", "client_ip")
		if hasOldColumns {
			return d.db.Migrator().DropTable("access_audits")
		}
	}
	return nil
}

// Begin 开始事务 - 返回新的事务 DAO 实例
func (d *dao) Begin() Dao {
	d.mu.Lock()
	defer d.mu.Unlock()

	// 总是返回一个新的 DAO 实例，包含事务
	tx := d.db.Begin()
	return &dao{
		db:     d.db,
		tx:     tx,
		config: d.config,
	}
}

// Commit 提交事务 - 线程安全
func (d *dao) Commit() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.tx == nil {
		return nil
	}
	err := d.tx.Commit().Error
	d.tx = nil // 清除事务对象
	return err
}

// Rollback 回滚事务 - 线程安全
func (d *dao) Rollback() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.tx == nil {
		return nil
	}
	err := d.tx.Rollback().Error
	d.tx = nil // 清除事务对象
	return err
}

// getDB 获取当前使用的数据库连接（事务或主连接）- 线程安全
func (d *dao) getDB() *gorm.DB {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.tx != nil {
		return d.tx
	}
	return d.db
}

func (d *dao) Close() error {
	sqlDB, err := d.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// User 相关方法实现
func (d *dao) CreateUser(user *model.User) error {
	return d.getDB().Create(user).Error
}

func (d *dao) GetUserByID(id uint) (*model.User, error) {
	var user model.User
	err := d.getDB().First(&user, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *dao) GetUserByEmail(email string) (*model.User, error) {
	var user model.User
	err := d.getDB().Where("email = ?", email).First(&user).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &user, nil
}

func (d *dao) UpdateUser(user *model.User) error {
	return d.getDB().Save(user).Error
}

func (d *dao) UpdateUserLastLogin(userID uint) error {
	now := time.Now()
	return d.getDB().Model(&model.User{}).Where("id = ?", userID).Update("last_login", now).Error
}

func (d *dao) UpdateUserLastLoginAndIP(userID uint, loginIP string) error {
	now := time.Now()
	return d.getDB().Model(&model.User{}).Where("id = ?", userID).Updates(map[string]interface{}{
		"last_login": now,
		"login_ip":   loginIP,
	}).Error
}

func (d *dao) ListUsers(offset, limit int) ([]*model.User, int64, error) {
	var users []*model.User
	var total int64

	// 获取总数
	if err := d.getDB().Model(&model.User{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	// 获取用户列表
	err := d.getDB().Offset(offset).Limit(limit).Find(&users).Error
	return users, total, err
}

func (d *dao) DeleteUser(id uint) error {
	return d.getDB().Delete(&model.User{}, id).Error
}

func (d *dao) CheckUserExists(email string) (bool, error) {
	var count int64
	err := d.getDB().Model(&model.User{}).Where("email = ?", email).Count(&count).Error
	return count > 0, err
}

/*
使用示例：

// 并发事务示例
func (s *Service) CreateMultipleEdgesConcurrently(edges []*model.Edge) error {
	var wg sync.WaitGroup
	errChan := make(chan error, len(edges))

	for _, edge := range edges {
		wg.Add(1)
		go func(e *model.Edge) {
			defer wg.Done()

			// 每个 goroutine 都有自己的事务 DAO
			txDao := s.dao.Begin()
			defer func() {
				if r := recover(); r != nil {
					txDao.Rollback()
				}
			}()

			if err := txDao.CreateEdge(e); err != nil {
				txDao.Rollback()
				errChan <- err
				return
			}

			if err := txDao.Commit(); err != nil {
				errChan <- err
			}
		}(edge)
	}

	wg.Wait()
	close(errChan)

	// 检查是否有错误
	for err := range errChan {
		if err != nil {
			return err
		}
	}

	return nil
}

// 在业务层使用事务
func (s *Service) CreateEdgeWithDevice(edge *model.Edge, device *model.Device) error {
	// 开始事务，获得新的事务 DAO
	txDao := s.dao.Begin()

	// 创建 Edge
	if err := txDao.CreateEdge(edge); err != nil {
		txDao.Rollback()
		return err
	}

	// 创建 Device
	if err := txDao.CreateDevice(device); err != nil {
		txDao.Rollback()
		return err
	}

	// 提交事务
	return txDao.Commit()
}

// 或者使用 defer 来确保回滚
func (s *Service) CreateEdgeWithDeviceSafe(edge *model.Edge, device *model.Device) error {
	txDao := s.dao.Begin()
	defer func() {
		if r := recover(); r != nil {
			txDao.Rollback()
		}
	}()

	if err := txDao.CreateEdge(edge); err != nil {
		txDao.Rollback()
		return err
	}

	if err := txDao.CreateDevice(device); err != nil {
		txDao.Rollback()
		return err
	}

	return txDao.Commit()
}

// 多个事务并发执行
func (s *Service) ProcessMultipleTransactions() error {
	// 事务1：创建 Edge
	txDao1 := s.dao.Begin()
	if err := txDao1.CreateEdge(&model.Edge{Name: "edge1"}); err != nil {
		txDao1.Rollback()
		return err
	}

	// 事务2：创建 Device（与事务1并发）
	txDao2 := s.dao.Begin()
	if err := txDao2.CreateDevice(&model.Device{Name: "device1"}); err != nil {
		txDao2.Rollback()
		txDao1.Rollback() // 也要回滚事务1
		return err
	}

	// 提交两个事务
	if err := txDao1.Commit(); err != nil {
		txDao2.Rollback()
		return err
	}

	return txDao2.Commit()
}
*/
