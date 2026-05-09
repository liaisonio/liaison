package frontierbound

import (
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

// 获取设备ID
func (fb *frontierBound) getID(meta []byte) (uint64, error) {
	var m proto.Meta
	if err := json.Unmarshal(meta, &m); err != nil {
		return 0, err
	}
	ak, edge, err := fb.repo.GetEdgeByAccessKey(m.AccessKey)
	if err != nil {
		return 0, err
	}
	if ak.SecretKey != m.SecretKey {
		return 0, errors.New("invalid secret key")
	}

	return uint64(edge.ID), nil
}

// updateEdgeHeartbeat 更新 edge 心跳时间
func (fb *frontierBound) updateEdgeHeartbeat(edgeID uint64) {
	log.Debugf("update edge heartbeat: %d", edgeID)
	edge, err := fb.repo.GetEdge(edgeID)
	if err != nil {
		log.Errorf("get edge error while updating heartbeat: %s, edge_id: %d", err, edgeID)
		return
	}
	if edge.Online != model.EdgeOnlineStatusOnline {
		if err := fb.repo.UpdateEdgeOnlineStatus(edgeID, model.EdgeOnlineStatusOnline); err != nil {
			log.Errorf("update edge online status error: %s, edge_id: %d", err, edgeID)
			return
		}
	}
	now := time.Now()
	err = fb.repo.UpdateEdgeHeartbeatAt(edgeID, now)
	if err != nil {
		log.Errorf("update edge heartbeat error: %s, edge_id: %d", err, edgeID)
	}
}

func (fb *frontierBound) online(edgeID uint64, meta []byte, addr net.Addr) error {
	log.Infof("edge online: %d, meta: %s, addr: %s", edgeID, string(meta), addr.String())
	err := fb.repo.UpdateEdgeOnlineStatus(edgeID, model.EdgeOnlineStatusOnline)
	if err != nil {
		return err
	}
	fb.rememberEdgeConn(edgeID, addr)
	// 更新心跳时间
	fb.updateEdgeHeartbeat(edgeID)
	return nil
}

func (fb *frontierBound) offline(edgeID uint64, meta []byte, addr net.Addr) error {
	log.Infof("edge offline: %d, meta: %s, addr: %s", edgeID, string(meta), addr.String())
	if !fb.shouldApplyEdgeOffline(edgeID, addr) {
		log.Infof("ignore stale edge offline: %d, addr: %s", edgeID, addr.String())
		return nil
	}
	err := fb.repo.UpdateEdgeOnlineStatus(edgeID, model.EdgeOnlineStatusOffline)
	if err != nil {
		return err
	}
	fb.forgetEdgeConn(edgeID, addr)
	tasks, err := fb.repo.ListTasks(&dao.ListTasksQuery{
		EdgeID: uint(edgeID),
		Status: []model.TaskStatus{
			model.TaskStatusPending,
			model.TaskStatusRunning,
		},
	})
	if err != nil {
		return err
	}
	for _, task := range tasks {
		if err := fb.repo.UpdateTaskError(task.ID, "edge offline"); err != nil {
			return err
		}
	}
	return nil
}

func (fb *frontierBound) rememberEdgeConn(edgeID uint64, addr net.Addr) {
	if addr == nil {
		return
	}
	fb.edgeConnMu.Lock()
	defer fb.edgeConnMu.Unlock()
	if fb.edgeConnAddr == nil {
		fb.edgeConnAddr = map[uint64]string{}
	}
	fb.edgeConnAddr[edgeID] = addr.String()
}

func (fb *frontierBound) shouldApplyEdgeOffline(edgeID uint64, addr net.Addr) bool {
	if addr == nil {
		return true
	}
	fb.edgeConnMu.Lock()
	defer fb.edgeConnMu.Unlock()
	if fb.edgeConnAddr == nil {
		return true
	}
	currentAddr, ok := fb.edgeConnAddr[edgeID]
	return !ok || currentAddr == addr.String()
}

func (fb *frontierBound) forgetEdgeConn(edgeID uint64, addr net.Addr) {
	if addr == nil {
		return
	}
	fb.edgeConnMu.Lock()
	defer fb.edgeConnMu.Unlock()
	if fb.edgeConnAddr == nil {
		return
	}
	if fb.edgeConnAddr[edgeID] == addr.String() {
		delete(fb.edgeConnAddr, edgeID)
	}
}

func (fb *frontierBound) forgetEdge(edgeID uint64) {
	fb.edgeConnMu.Lock()
	defer fb.edgeConnMu.Unlock()
	if fb.edgeConnAddr == nil {
		return
	}
	delete(fb.edgeConnAddr, edgeID)
}
