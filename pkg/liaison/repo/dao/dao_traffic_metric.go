package dao

import (
	"time"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// CreateTrafficMetric 创建流量监控记录
func (d *dao) CreateTrafficMetric(metric *model.TrafficMetric) error {
	return d.getDB().Create(metric).Error
}

// ListTrafficMetrics 查询流量监控数据
func (d *dao) ListTrafficMetrics(query *ListTrafficMetricsQuery) ([]*model.TrafficMetric, error) {
	var metrics []*model.TrafficMetric
	db := d.getDB()

	if len(query.ApplicationIDs) > 0 {
		db = db.Where("application_id IN ?", query.ApplicationIDs)
	}
	if len(query.ProxyIDs) > 0 {
		db = db.Where("proxy_id IN ?", query.ProxyIDs)
	}
	if query.StartTime != nil {
		db = db.Where("timestamp >= ?", *query.StartTime)
	}
	if query.EndTime != nil {
		db = db.Where("timestamp <= ?", *query.EndTime)
	}

	// 按时间正序排列（从早到晚），方便前端处理
	db = db.Order("timestamp ASC")

	if query.Limit > 0 {
		db = db.Limit(query.Limit)
	} else {
		// 如果没有设置limit，默认返回10000条
		db = db.Limit(10000)
	}

	err := db.Find(&metrics).Error
	return metrics, err
}

// GetTrafficMetricsByTimeRange 获取指定时间范围内的流量统计（按应用聚合）
func (d *dao) GetTrafficMetricsByTimeRange(startTime, endTime time.Time, applicationIDs []uint) ([]*model.TrafficMetric, error) {
	var metrics []*model.TrafficMetric
	db := d.getDB().
		Where("timestamp >= ? AND timestamp <= ?", startTime, endTime)

	if len(applicationIDs) > 0 {
		db = db.Where("application_id IN ?", applicationIDs)
	}

	// 按应用ID和时间分组，聚合流量
	err := db.Select("application_id, proxy_id, MIN(timestamp) as timestamp, SUM(bytes_in) as bytes_in, SUM(bytes_out) as bytes_out").
		Group("application_id, timestamp").
		Order("timestamp ASC").
		Find(&metrics).Error

	return metrics, err
}
