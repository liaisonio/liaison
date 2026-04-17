package frontierbound

import (
	"encoding/json"
	"errors"
	"net"
	"time"

	"github.com/jumboframes/armorigo/log"
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
	now := time.Now()
	err := fb.repo.UpdateEdgeHeartbeatAt(edgeID, now)
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
	// 更新心跳时间
	fb.updateEdgeHeartbeat(edgeID)
	return nil
}

func (fb *frontierBound) offline(edgeID uint64, meta []byte, addr net.Addr) error {
	log.Infof("edge offline: %d, meta: %s, addr: %s", edgeID, string(meta), addr.String())
	err := fb.repo.UpdateEdgeOnlineStatus(edgeID, model.EdgeOnlineStatusOffline)
	if err != nil {
		return err
	}
	// 查询所有edge任务，如果是pending和running状态，则更新为failed
	err = fb.repo.UpdateTaskError(uint(edgeID), "edge offline")
	if err != nil {
		return err
	}
	return nil
}
