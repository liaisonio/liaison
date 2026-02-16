package proto

import "context"

// manager <-> entry
// 一个Proxy是三元组
type Proxy struct {
	ID   int
	Name string
	// 代理端口
	ProxyPort int
	// 边缘ID
	EdgeID uint64
	// 应用ID（用于流量统计）
	ApplicationID uint
	// 目的地址
	Dst string
	// 应用类型（用于决定使用 HTTP 还是 TCP 代理）
	ApplicationType string
	// 是否使用 HTTPS（仅对 HTTP 应用有效）
	UseHTTPS bool
}

type ProxyManager interface {
	CreateProxy(ctx context.Context, proxy *Proxy) error
	DeleteProxy(ctx context.Context, id int) error
}

// manager <-> edge
type Meta struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// device
type Device struct {
	Fingerprint string                     `json:"fingerprint"`
	HostName    string                     `json:"host_name"`
	CPU         int                        `json:"cpu"`
	Memory      int                        `json:"memory"`
	Disk        int                        `json:"disk"` // 根磁盘大小（MB）
	OS          string                     `json:"os"`
	OSVersion   string                     `json:"os_version"`
	DeviceUsage DeviceUsage                `json:"device_usage"`
	Interfaces  []*DeviceEthernetInterface `json:"interfaces"`
	EdgeID      uint64                     `json:"edge_id"` // 如果包含edge id，则表示是edge所在的机器
}

type DeviceEthernetInterface struct {
	Name    string    `json:"name"`
	MAC     string    `json:"mac"`
	IPMasks []*IPMask `json:"ip_masks"`
}

type IPMask struct {
	IP      string `json:"ip"`
	Netmask string `json:"netmask"`
}

type DeviceUsage struct {
	Fingerprint string  `json:"fingerprint"`
	CPUUsage    float32 `json:"cpu_usage"`
	MemoryUsage float32 `json:"memory_usage"`
	DiskUsage   float32 `json:"disk_usage"`
}

// task
type ScanApplicationTaskRequest struct {
	TaskID   uint     `json:"task_id"`
	Nets     []string `json:"nets"`
	Port     int      `json:"port"` // 如果为0，则扫描主流端口
	Protocol string   `json:"protocol"`
}

type ScannedApplication struct {
	IP       string `json:"ip"`
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

type ScanApplicationTaskResult struct {
	TaskID              uint                 `json:"task_id"`
	ScannedApplications []ScannedApplication `json:"scanned_applications"`
	Error               string               `json:"error"`
	Status              string               `json:"status"` // running, completed, failed
}

type Dst struct {
	Addr          string `json:"addr"`
	ApplicationID uint   `json:"application_id,omitempty"` // 应用ID（用于流量统计）
	ProxyID       uint   `json:"proxy_id,omitempty"`       // 代理ID（用于流量统计）
}

type PullTaskScanApplicationRequest struct {
	EdgeID uint64 `json:"edge_id"`
}

type PullTaskScanApplicationResponse struct {
	TaskID   uint     `json:"task_id"`
	Nets     []string `json:"nets"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
}

// ping device
type GetEdgeDiscoveredDevicesRequest struct {
	EdgeID uint64 `json:"edge_id"`
}

type DiscoveredDevice struct {
	DeviceID uint64 `json:"device_id"`
	IP       string `json:"ip"` // 设备的 IP 地址（用于 ping）
}

type GetEdgeDiscoveredDevicesResponse struct {
	Devices []DiscoveredDevice `json:"devices"`
}

type UpdateDeviceHeartbeatRequest struct {
	DeviceID uint64 `json:"device_id"`
}

// traffic metric
type ReportTrafficMetricRequest struct {
	ProxyID       uint  `json:"proxy_id"`
	ApplicationID uint  `json:"application_id"`
	BytesIn       int64 `json:"bytes_in"`
	BytesOut      int64 `json:"bytes_out"`
}
