package dao

import (
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
	return d.db.AutoMigrate(
		&model.Edge{},
		&model.AccessKey{},
		&model.Device{},
		&model.EthernetInterface{},
		&model.EdgeDevice{},
		&model.Application{},
		&model.Proxy{},
		&model.Task{},
		&model.User{},
		&model.TrafficMetric{},
		&model.UserAPIToken{},
		&model.ProxyFirewallRule{},
	)
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
