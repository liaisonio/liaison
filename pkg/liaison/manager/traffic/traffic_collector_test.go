package traffic

import (
	"errors"
	"testing"

	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"gorm.io/gorm"
)

type sampleRepo struct {
	dao.Dao
	proxies []*model.Proxy
	metrics []*model.TrafficMetric
	err     error
}

func (r *sampleRepo) ListProxies(*dao.ListProxiesQuery) ([]*model.Proxy, error) {
	return r.proxies, r.err
}
func (r *sampleRepo) CreateTrafficMetric(m *model.TrafficMetric) error {
	r.metrics = append(r.metrics, m)
	return nil
}

func TestCollector_RecordsIdleAndActiveMinutes(t *testing.T) {
	r := &sampleRepo{proxies: []*model.Proxy{{Model: gorm.Model{ID: 1}, ApplicationID: 2}}}
	c := &TrafficCollector{repo: r, stats: make(map[string]*trafficStats)}
	c.flush()
	c.RecordTraffic(1, 2, 600, 120)
	c.flush()
	c.flush()
	if len(r.metrics) != 3 {
		t.Fatalf("samples=%d", len(r.metrics))
	}
	if r.metrics[0].BytesIn != 0 || r.metrics[1].BytesIn != 600 || r.metrics[1].BytesOut != 120 || r.metrics[2].BytesIn != 0 {
		t.Fatal("expected idle, activity, idle")
	}
	r.proxies = nil
	c.flush()
	if len(r.metrics) != 3 {
		t.Fatal("deleted access must stop sampling")
	}
}

func TestCollector_TargetQueryFailureDoesNotInventZeros(t *testing.T) {
	r := &sampleRepo{err: errors.New("unavailable")}
	c := &TrafficCollector{repo: r, stats: make(map[string]*trafficStats)}
	c.flush()
	if len(r.metrics) != 0 {
		t.Fatal("failed collection must remain a gap")
	}
	c.RecordTraffic(1, 2, 42, 0)
	c.flush()
	if len(r.metrics) != 1 || r.metrics[0].BytesIn != 42 {
		t.Fatal("retain observed traffic on target lookup failure")
	}
}
