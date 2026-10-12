package model

import (
	"time"

	"github.com/liaisonio/liaison/pkg/proto"
	"gorm.io/gorm"
)

type TaskType int

const (
	TaskTypeScan TaskType = iota + 1
)

type TaskSubType int

const (
	TaskSubTypeScanApplication TaskSubType = iota + 1
)

type TaskStatus int

func (t TaskStatus) String() string {
	switch t {
	case TaskStatusPending:
		return "pending"
	case TaskStatusRunning:
		return "running"
	case TaskStatusCompleted:
		return "completed"
	case TaskStatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

const (
	TaskStatusPending TaskStatus = iota + 1
	TaskStatusRunning
	TaskStatusCompleted
	TaskStatusFailed
)

type Task struct {
	gorm.Model
	EdgeID      uint64      `gorm:"column:edge_id;type:bigint;not null"` // edge的任务
	TaskType    TaskType    `gorm:"column:task_type;type:int;not null"`
	TaskSubType TaskSubType `gorm:"column:task_sub_type;type:int;not null"`
	TaskStatus  TaskStatus  `gorm:"column:task_status;type:int;not null"`
	TaskParams  []byte      `gorm:"column:task_params;type:blob;not null"`
	TaskResult  []byte      `gorm:"column:task_result;type:blob;not null"`
	Error       string      `gorm:"column:error;type:text;not null"`
	ExpiredAt   time.Time   `gorm:"column:expired_at;type:datetime;not null"`
}

func (Task) TableName() string {
	return "tasks"
}

// task scan application
type TaskScanApplicationParams struct {
	Nets     []string `json:"nets"`
	Port     int      `json:"port"`
	Protocol string   `json:"protocol"`
}

type ScannedApplication struct {
	IP       string                   `json:"ip"`
	Port     int                      `json:"port"`
	Protocol string                   `json:"protocol"`
	ModelAPI *proto.ModelAPIDiscovery `json:"model_api,omitempty"`
}

type TaskScanApplicationResult struct {
	ScannedApplications []ScannedApplication `json:"scanned_applications"`
}
