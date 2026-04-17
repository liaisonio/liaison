package dao

import (
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func (d *dao) CreateTask(task *model.Task) error {
	return d.getDB().Create(task).Error
}

func (d *dao) UpdateTaskStatus(taskID uint, status model.TaskStatus) error {
	return d.getDB().Model(&model.Task{}).Where("id = ?", taskID).Update("task_status", status).Error
}

func (d *dao) UpdateTaskResult(taskID uint, status model.TaskStatus, result []byte) error {
	return d.getDB().Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"task_result": result,
		"task_status": status,
	}).Error
}

func (d *dao) UpdateTaskError(taskID uint, error string) error {
	return d.getDB().Model(&model.Task{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"error":       error,
		"task_status": model.TaskStatusFailed,
	}).Error
}

func (d *dao) GetTask(taskID uint) (*model.Task, error) {
	var task model.Task
	if err := d.getDB().Where("id = ?", taskID).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (d *dao) GetTaskByEdgeID(edgeID uint64) (*model.Task, error) {
	var task model.Task
	if err := d.getDB().Where("edge_id = ?", edgeID).Where("task_status = ? or task_status = ?", model.TaskStatusPending, model.TaskStatusRunning).First(&task).Error; err != nil {
		return nil, err
	}
	return &task, nil
}

func (d *dao) ListTasks(query *ListTasksQuery) ([]*model.Task, error) {
	db := d.getDB()
	// 状态
	if len(query.Status) > 0 {
		db = db.Where("task_status IN ?", query.Status)
	}
	// 时间范围
	if query.StartTime > 0 {
		db = db.Where("created_at >= ?", time.Unix(query.StartTime, 0))
	}
	if query.EndTime > 0 {
		db = db.Where("created_at <= ?", time.Unix(query.EndTime, 0))
	}
	// 边缘ID
	if query.EdgeID > 0 {
		db = db.Where("edge_id = ?", query.EdgeID)
	}
	// 任务类型
	if query.TaskType != 0 {
		db = db.Where("task_type = ?", query.TaskType)
	}
	// 任务子类型
	if query.TaskSubType != 0 {
		db = db.Where("task_sub_type = ?", query.TaskSubType)
	}
	// 排序（必须在分页之前）
	if query.Order != "" {
		if query.Desc {
			db = db.Order(query.Order + " DESC")
		} else {
			db = db.Order(query.Order)
		}
	} else {
		// 默认按 ID 降序（最新的在前）
		db = db.Order("id DESC")
	}
	// 分页（必须在排序之后）
	if query.Page > 0 && query.PageSize > 0 {
		db = db.Offset((query.Page - 1) * query.PageSize).Limit(query.PageSize)
	}
	var tasks []*model.Task
	if err := db.Find(&tasks).Error; err != nil {
		return nil, err
	}
	return tasks, nil
}
