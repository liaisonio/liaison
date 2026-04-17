package controlplane

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/jumboframes/armorigo/log"
	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
)

// 本地时间格式（不带时区信息）
const localTimeFormat = "2006-01-02T15:04:05"

func (cp *controlPlane) ListTrafficMetrics(_ context.Context, req *v1.ListTrafficMetricsRequest) (*v1.ListTrafficMetricsResponse, error) {
	query := &dao.ListTrafficMetricsQuery{
		Limit: int(req.Limit),
	}

	// 转换应用ID列表
	if len(req.ApplicationIds) > 0 {
		query.ApplicationIDs = make([]uint, len(req.ApplicationIds))
		for i, id := range req.ApplicationIds {
			query.ApplicationIDs[i] = uint(id)
		}
	}

	// 转换代理ID列表
	if len(req.ProxyIds) > 0 {
		query.ProxyIDs = make([]uint, len(req.ProxyIds))
		for i, id := range req.ProxyIds {
			query.ProxyIDs[i] = uint(id)
		}
	}

	// 解析时间范围（前端发送的是本地时间或RFC3339格式）
	if req.StartTime != "" {
		// 尝试解析RFC3339格式，如果失败则尝试本地时间格式
		startTime, err := time.Parse(time.RFC3339, req.StartTime)
		if err != nil {
			startTime, err = time.Parse(localTimeFormat, req.StartTime)
		}
		if err == nil {
			// 使用本地时间用于查询
			query.StartTime = &startTime
		}
	}
	if req.EndTime != "" {
		// 尝试解析RFC3339格式，如果失败则尝试本地时间格式
		endTime, err := time.Parse(time.RFC3339, req.EndTime)
		if err != nil {
			endTime, err = time.Parse(localTimeFormat, req.EndTime)
		}
		if err == nil {
			// 使用本地时间用于查询
			query.EndTime = &endTime
		}
	}

	// 如果没有设置 limit，根据时间范围计算合理的limit
	// 24小时 = 1440分钟，如果有多个应用，需要更大的limit
	if query.Limit == 0 {
		if query.StartTime != nil && query.EndTime != nil {
			// 计算时间范围（分钟）
			minutes := int(query.EndTime.Sub(*query.StartTime).Minutes())
			// 假设最多10个应用，每个应用每分钟1条数据
			query.Limit = minutes * 10
			if query.Limit > 10000 {
				query.Limit = 10000 // 最大限制
			}
		} else {
			query.Limit = 10000 // 默认返回10000条
		}
	}

	metrics, err := cp.repo.ListTrafficMetrics(query)
	if err != nil {
		return nil, err
	}

	// 调试日志：记录查询结果
	if query.StartTime != nil && query.EndTime != nil {
		log.Debugf("查询流量数据: startTime=%s, endTime=%s, limit=%d, 返回%d条",
			query.StartTime.Format(localTimeFormat),
			query.EndTime.Format(localTimeFormat),
			query.Limit,
			len(metrics))
	}

	// 按分钟聚合数据
	// key: "application_id:proxy_id:YYYY-MM-DD HH:MM"
	type aggregatedMetric struct {
		ApplicationID uint
		ProxyID       uint
		Timestamp     time.Time
		BytesIn       int64
		BytesOut      int64
		Count         int
	}
	aggregated := make(map[string]*aggregatedMetric)

	for _, metric := range metrics {
		// 将时间戳对齐到分钟
		alignedTime := time.Date(
			metric.Timestamp.Year(),
			metric.Timestamp.Month(),
			metric.Timestamp.Day(),
			metric.Timestamp.Hour(),
			metric.Timestamp.Minute(),
			0,
			0,
			metric.Timestamp.Location(),
		)
		key := fmt.Sprintf("%d:%d:%s", metric.ApplicationID, metric.ProxyID, alignedTime.Format("2006-01-02 15:04"))

		if agg, exists := aggregated[key]; exists {
			agg.BytesIn += metric.BytesIn
			agg.BytesOut += metric.BytesOut
			agg.Count++
		} else {
			aggregated[key] = &aggregatedMetric{
				ApplicationID: metric.ApplicationID,
				ProxyID:       metric.ProxyID,
				Timestamp:     alignedTime,
				BytesIn:       metric.BytesIn,
				BytesOut:      metric.BytesOut,
				Count:         1,
			}
		}
	}

	// 转换为 proto 格式
	metricsV1 := make([]*v1.TrafficMetric, 0, len(aggregated))
	for _, agg := range aggregated {
		// 计算平均值
		avgBytesIn := agg.BytesIn / int64(agg.Count)
		avgBytesOut := agg.BytesOut / int64(agg.Count)

		// 将 int64 转换为 int32，确保 JSON 序列化为数字而不是字符串
		// 注意：如果值超过 int32 最大值（2GB），会被截断
		bytesIn := int32(avgBytesIn)
		bytesOut := int32(avgBytesOut)
		// 如果值超过 int32 最大值，设置为最大值
		if avgBytesIn > 2147483647 {
			bytesIn = 2147483647
		}
		if avgBytesOut > 2147483647 {
			bytesOut = 2147483647
		}

		// 直接使用数据库返回的时间戳，不进行格式化
		// 使用time.Time的默认字符串表示（RFC3339格式）
		metricsV1 = append(metricsV1, &v1.TrafficMetric{
			Id:            0, // 聚合后的数据没有原始ID
			ApplicationId: uint64(agg.ApplicationID),
			ProxyId:       uint64(agg.ProxyID),
			Timestamp:     agg.Timestamp.Format(time.RFC3339), // 使用RFC3339格式，这是time.Time的默认JSON序列化格式
			BytesIn:       bytesIn,
			BytesOut:      bytesOut,
		})
	}

	// 按时间排序
	sort.Slice(metricsV1, func(i, j int) bool {
		// 尝试解析RFC3339格式，如果失败则尝试本地时间格式
		ti, err := time.Parse(time.RFC3339, metricsV1[i].Timestamp)
		if err != nil {
			ti, _ = time.Parse(localTimeFormat, metricsV1[i].Timestamp)
		}
		tj, err := time.Parse(time.RFC3339, metricsV1[j].Timestamp)
		if err != nil {
			tj, _ = time.Parse(localTimeFormat, metricsV1[j].Timestamp)
		}
		return ti.Before(tj)
	})

	return &v1.ListTrafficMetricsResponse{
		Code:    200,
		Message: "success",
		Data: &v1.TrafficMetrics{
			Metrics: metricsV1,
		},
	}, nil
}
