package traffic

import (
	"fmt"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

// TrafficCollector 流量统计收集器
// 负责收集流量数据，每分钟落盘一次
type TrafficCollector struct {
	repo dao.Dao
	mu   sync.RWMutex
	// key: "proxyID:applicationID", value: traffic stats
	stats map[string]*trafficStats
	stop  chan struct{}
}

type trafficStats struct {
	ProxyID       uint
	ApplicationID uint
	BytesIn       int64
	BytesOut      int64
}

// NewTrafficCollector 创建流量统计收集器
func NewTrafficCollector(repo dao.Dao) *TrafficCollector {
	collector := &TrafficCollector{
		repo:  repo,
		stats: make(map[string]*trafficStats),
		stop:  make(chan struct{}),
	}

	// 启动定时落盘任务
	go collector.flushLoop()

	return collector
}

// RecordTraffic 记录流量（线程安全）
func (tc *TrafficCollector) RecordTraffic(proxyID, applicationID uint, bytesIn, bytesOut int64) {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	key := trafficKey(proxyID, applicationID)
	stats, exists := tc.stats[key]
	if !exists {
		stats = &trafficStats{
			ProxyID:       proxyID,
			ApplicationID: applicationID,
		}
		tc.stats[key] = stats
	}

	stats.BytesIn += bytesIn
	stats.BytesOut += bytesOut
}

// flushLoop 每分钟落盘一次
func (tc *TrafficCollector) flushLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			tc.flush()
		case <-tc.stop:
			// 退出前最后一次落盘
			tc.flush()
			return
		}
	}
}

// flush 将统计数据落盘
func (tc *TrafficCollector) flush() {
	tc.mu.Lock()
	if len(tc.stats) == 0 {
		tc.mu.Unlock()
		return
	}

	// 复制统计数据
	statsToFlush := make([]*trafficStats, 0, len(tc.stats))
	for _, stats := range tc.stats {
		statsToFlush = append(statsToFlush, &trafficStats{
			ProxyID:       stats.ProxyID,
			ApplicationID: stats.ApplicationID,
			BytesIn:       stats.BytesIn,
			BytesOut:      stats.BytesOut,
		})
	}

	// 清空统计数据
	tc.stats = make(map[string]*trafficStats)
	tc.mu.Unlock()

	// 落盘（在锁外执行，避免阻塞）
	// 直接使用当前本地时间
	now := time.Now()

	for _, stats := range statsToFlush {
		metric := &model.TrafficMetric{
			ApplicationID: stats.ApplicationID,
			ProxyID:       stats.ProxyID,
			Timestamp:     now,
			BytesIn:       stats.BytesIn,
			BytesOut:      stats.BytesOut,
		}

		if err := tc.repo.CreateTrafficMetric(metric); err != nil {
			log.Errorf("failed to create traffic metric: %s", err)
		}
	}

	log.Debugf("flushed %d traffic metrics", len(statsToFlush))
}

// Stop 停止收集器
func (tc *TrafficCollector) Stop() {
	close(tc.stop)
}

func trafficKey(proxyID, applicationID uint) string {
	return fmt.Sprintf("%d:%d", proxyID, applicationID)
}
