package dao

import (
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

type Query struct {
	// Pagination
	Page, PageSize int
	// Time range
	StartTime, EndTime int64
	// Order
	Order string
	Desc  bool
}

type ListTasksQuery struct {
	Query
	EdgeID      uint
	Status      []model.TaskStatus
	TaskType    model.TaskType
	TaskSubType model.TaskSubType
}

type ListApplicationsQuery struct {
	Query
	DeviceIDs       []uint
	IDs             []uint
	ApplicationType string
}

type ListDevicesQuery struct {
	Query
	IDs  []uint
	Name string
	IP   string
}

type ListProxiesQuery struct {
	Query
	IDs            []uint
	ApplicationIDs []uint
	Name           string
}

type ListEdgesQuery struct {
	Query
	DeviceIDs []uint  // 已废弃，保留以兼容
	EdgeIDs   []uint64 // 通过 EdgeDevice 关系表查询的 Edge IDs
	Name      string
}

type ListTrafficMetricsQuery struct {
	ApplicationIDs []uint
	ProxyIDs       []uint
	StartTime      *time.Time
	EndTime        *time.Time
	Limit          int
}
