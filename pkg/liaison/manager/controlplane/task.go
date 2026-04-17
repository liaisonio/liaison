package controlplane

import (
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

func (cp *controlPlane) checkTask() {
	// 查询所有pending和running状态的任务
	tasks, err := cp.repo.ListTasks(&dao.ListTasksQuery{
		Status: []model.TaskStatus{model.TaskStatusPending, model.TaskStatusRunning},
	})
	if err != nil {
		return
	}
	for _, task := range tasks {
		err = cp.repo.UpdateTaskError(task.ID, "task expired")
		if err != nil {
			return
		}
	}
}
