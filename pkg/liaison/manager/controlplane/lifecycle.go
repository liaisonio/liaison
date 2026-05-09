package controlplane

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

const (
	proxyEffectiveStatusActive      = "active"
	proxyEffectiveStatusStopped     = "stopped"
	proxyEffectiveStatusEdgeStopped = "edge_stopped"
	proxyEffectiveStatusEdgeOffline = "edge_offline"
	proxyEffectiveStatusInvalid     = "invalid"
)

// stopProxyRuntime stops the data-plane listener for proxy and revokes any
// associated firewall rule from the in-memory kernel. Safe to call on an
// already-stopped proxy or when no proxyManager is registered.
func (cp *controlPlane) stopProxyRuntime(proxy *model.Proxy) error {
	if proxy == nil || cp.proxyManager == nil {
		return nil
	}
	if err := cp.proxyManager.DeleteProxy(context.Background(), int(proxy.ID)); err != nil {
		return err
	}
	if cp.firewallManager != nil && proxy.Port > 0 {
		cp.firewallManager.Revoke(int(proxy.ID))
	}
	return nil
}

// startProxyRuntime (re-)starts the data-plane listener for proxy and, if a
// firewall rule is persisted for it, pushes the rule into the data-plane
// allowlist immediately.
func (cp *controlPlane) startProxyRuntime(proxy *model.Proxy, application *model.Application) error {
	if proxy == nil || application == nil || cp.proxyManager == nil {
		return nil
	}
	eligible, err := cp.proxyRuntimeEligible(proxy, application)
	if err != nil {
		return err
	}
	if !eligible {
		return nil
	}
	useHTTPS := application.ApplicationType == model.ApplicationTypeHTTP
	protoproxy := &proto.Proxy{
		ID:              int(proxy.ID),
		Name:            proxy.Name,
		ProxyPort:       proxy.Port,
		EdgeID:          uint64(application.EdgeIDs[0]),
		ApplicationID:   application.ID,
		Dst:             fmt.Sprintf("%s:%d", application.IP, application.Port),
		ApplicationType: string(application.ApplicationType),
		UseHTTPS:        useHTTPS,
	}
	if err := cp.proxyManager.CreateProxy(context.Background(), protoproxy); err != nil {
		return err
	}
	oldPort := proxy.Port
	if proxy.Port == 0 && protoproxy.ProxyPort > 0 {
		proxy.Port = protoproxy.ProxyPort
		if err := cp.ensureProxyPortAvailable(proxy.Port, proxy.ID); err != nil {
			_ = cp.stopProxyRuntime(proxy)
			proxy.Port = oldPort
			return err
		}
	}
	cp.reapplyFirewall(proxy.ID, proxy.Port)
	return nil
}

// reapplyFirewall reads the persisted allowlist for proxyID and pushes it to
// the data plane. Called from startProxyRuntime and RestoreFirewallRules so
// that kernel-side state stays in sync with DB-side state across restarts.
func (cp *controlPlane) reapplyFirewall(proxyID uint, port int) {
	if cp.firewallManager == nil || cp.repo == nil || port <= 0 {
		return
	}
	rule, err := cp.repo.GetFirewallRuleByProxyID(proxyID)
	if err != nil {
		log.Warnf("firewall: lookup proxy=%d failed: %v", proxyID, err)
		return
	}
	if rule == nil {
		// No persisted rule → allow-all. Clear any residual kernel state.
		cp.firewallManager.Revoke(int(proxyID))
		return
	}
	if applyErr := cp.firewallManager.Allow(int(proxyID), []string(rule.AllowedCIDRs)); applyErr != nil {
		log.Warnf("firewall: re-apply proxy=%d failed: %v", proxyID, applyErr)
	}
}

// RestoreFirewallRules rehydrates the data-plane firewall state from DB on
// process startup. Call after the entry layer has started its proxies, so
// that firewall.Manager mirrors the persisted ProxyFirewallRule table.
func (cp *controlPlane) RestoreFirewallRules() {
	if cp.firewallManager == nil || cp.repo == nil {
		return
	}
	rules, err := cp.repo.ListAllFirewallRules()
	if err != nil {
		log.Warnf("firewall: list all rules failed: %v", err)
		return
	}
	for _, rule := range rules {
		proxy, err := cp.repo.GetProxyByID(rule.ProxyID)
		if err != nil {
			log.Warnf("firewall: proxy=%d missing, skip restore: %v", rule.ProxyID, err)
			continue
		}
		application, err := cp.repo.GetApplicationByID(proxy.ApplicationID)
		if err != nil {
			log.Warnf("firewall: application=%d missing for proxy=%d, skip restore: %v", proxy.ApplicationID, proxy.ID, err)
			continue
		}
		eligible, err := cp.proxyRuntimeEligible(proxy, application)
		if err != nil || !eligible {
			cp.firewallManager.Revoke(int(rule.ProxyID))
			continue
		}
		if err := cp.firewallManager.Allow(int(rule.ProxyID), []string(rule.AllowedCIDRs)); err != nil {
			log.Warnf("firewall: restore proxy=%d failed: %v", rule.ProxyID, err)
		}
	}
	log.Infof("firewall: restored %d rule(s) from DB", len(rules))
}

// ReconcileLifecycleState repairs persisted lifecycle state before entry starts
// listeners. It is intentionally conservative: stale runtime is stopped and
// orphaned proxies are soft-deleted, while traffic/task history is retained.
func (cp *controlPlane) ReconcileLifecycleState() {
	if cp.repo == nil {
		return
	}
	proxies, err := cp.repo.ListProxies(&dao.ListProxiesQuery{})
	if err != nil {
		log.Warnf("lifecycle: list proxies for reconcile failed: %v", err)
		return
	}
	for _, proxy := range proxies {
		application, appErr := cp.repo.GetApplicationByID(proxy.ApplicationID)
		if appErr != nil || application == nil {
			log.Warnf("lifecycle: deleting orphan proxy %d, application %d missing", proxy.ID, proxy.ApplicationID)
			if err := cp.stopProxyRuntime(proxy); err != nil {
				log.Warnf("lifecycle: stop orphan proxy %d failed, keeping record for retry: %v", proxy.ID, err)
				continue
			}
			if err := cp.repo.DeleteFirewallRuleByProxyID(proxy.ID); err != nil {
				log.Warnf("lifecycle: delete firewall rule for orphan proxy %d failed: %v", proxy.ID, err)
			}
			if err := cp.repo.DeleteProxy(proxy.ID); err != nil {
				log.Warnf("lifecycle: delete orphan proxy %d failed: %v", proxy.ID, err)
			}
			continue
		}

		if proxy.Status == model.ProxyStatusStopped {
			if err := cp.stopProxyRuntime(proxy); err != nil {
				log.Warnf("lifecycle: stop disabled proxy %d failed: %v", proxy.ID, err)
			}
			continue
		}

		eligible, eligibleErr := cp.proxyRuntimeEligible(proxy, application)
		if eligibleErr != nil {
			log.Warnf("lifecycle: proxy %d invalid, deleting: %v", proxy.ID, eligibleErr)
			if err := cp.stopProxyRuntime(proxy); err != nil {
				log.Warnf("lifecycle: stop invalid proxy %d failed, keeping record for retry: %v", proxy.ID, err)
				continue
			}
			if err := cp.repo.DeleteFirewallRuleByProxyID(proxy.ID); err != nil {
				log.Warnf("lifecycle: delete firewall rule for invalid proxy %d failed: %v", proxy.ID, err)
			}
			if err := cp.repo.DeleteProxy(proxy.ID); err != nil {
				log.Warnf("lifecycle: delete invalid proxy %d failed: %v", proxy.ID, err)
			}
			continue
		}
		if !eligible {
			if err := cp.stopProxyRuntime(proxy); err != nil {
				log.Warnf("lifecycle: stop non-runnable proxy %d failed: %v", proxy.ID, err)
			}
		}
	}
}

func (cp *controlPlane) RestoreProxyListeners() error {
	if cp.repo == nil || cp.proxyManager == nil {
		return nil
	}
	proxies, err := cp.repo.ListProxies(&dao.ListProxiesQuery{})
	if err != nil {
		return err
	}
	started := make([]*model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.Status != model.ProxyStatusRunning {
			continue
		}
		application, err := cp.repo.GetApplicationByID(proxy.ApplicationID)
		if err != nil {
			log.Warnf("lifecycle: skip restore proxy %d, application %d missing: %v", proxy.ID, proxy.ApplicationID, err)
			continue
		}
		eligible, err := cp.proxyRuntimeEligible(proxy, application)
		if err != nil {
			log.Warnf("lifecycle: skip restore proxy %d, invalid runtime state: %v", proxy.ID, err)
			continue
		}
		if !eligible {
			continue
		}
		proxy.Application = application
		if err := cp.startAndPersistProxyRuntime(proxy); err != nil {
			for _, startedProxy := range started {
				_ = cp.stopProxyRuntime(startedProxy)
			}
			return err
		}
		started = append(started, proxy)
	}
	return nil
}

func (cp *controlPlane) proxyRuntimeEligible(proxy *model.Proxy, application *model.Application) (bool, error) {
	if proxy == nil || proxy.Status != model.ProxyStatusRunning {
		return false, nil
	}
	if application == nil {
		return false, errors.New("proxy application is missing")
	}
	if len(application.EdgeIDs) == 0 {
		return false, errors.New("application has no edge")
	}
	edge, err := cp.repo.GetEdge(uint64(application.EdgeIDs[0]))
	if err != nil {
		return false, fmt.Errorf("application edge %d is missing: %w", application.EdgeIDs[0], err)
	}
	if edge.Status == model.EdgeStatusStopped {
		return false, nil
	}
	if edge.Status != model.EdgeStatusRunning {
		return false, fmt.Errorf("unknown edge status %d", edge.Status)
	}
	return true, nil
}

func (cp *controlPlane) validateProxyApplication(application *model.Application) (*model.Edge, error) {
	if application == nil {
		return nil, errors.New("application is missing")
	}
	if len(application.EdgeIDs) == 0 {
		return nil, errors.New("application has no edge")
	}
	edge, err := cp.repo.GetEdge(uint64(application.EdgeIDs[0]))
	if err != nil {
		return nil, err
	}
	if edge.Status != model.EdgeStatusRunning && edge.Status != model.EdgeStatusStopped {
		return nil, fmt.Errorf("unknown edge status %d", edge.Status)
	}
	return edge, nil
}

func (cp *controlPlane) proxyEffectiveStatus(proxy *model.Proxy, application *model.Application) (string, string) {
	if proxy == nil {
		return proxyEffectiveStatusInvalid, "访问不存在"
	}
	if proxy.Status == model.ProxyStatusStopped {
		return proxyEffectiveStatusStopped, "访问未启用"
	}
	if proxy.Status != model.ProxyStatusRunning {
		return proxyEffectiveStatusInvalid, "访问状态无效"
	}
	edge, err := cp.validateProxyApplication(application)
	if err != nil {
		return proxyEffectiveStatusInvalid, "关联应用或连接器无效"
	}
	if edge.Status == model.EdgeStatusStopped {
		return proxyEffectiveStatusEdgeStopped, "连接器已禁用"
	}
	if edge.Online != model.EdgeOnlineStatusOnline {
		return proxyEffectiveStatusEdgeOffline, "连接器离线"
	}
	return proxyEffectiveStatusActive, "访问可用"
}

func (cp *controlPlane) ensureProxyPortAvailable(port int, excludeProxyID uint) error {
	if port <= 0 {
		return nil
	}
	conflict, err := cp.findProxyPortConflict(port, excludeProxyID)
	if err != nil {
		return err
	}
	if conflict != nil {
		return fmt.Errorf("public port %d is already used by proxy %d", port, conflict.ID)
	}
	return nil
}

func (cp *controlPlane) findProxyPortConflict(port int, excludeProxyID uint) (*model.Proxy, error) {
	if port <= 0 {
		return nil, nil
	}
	proxies, err := cp.repo.ListProxies(&dao.ListProxiesQuery{})
	if err != nil {
		return nil, err
	}
	for _, proxy := range proxies {
		if proxy.ID != excludeProxyID && proxy.Port == port {
			return proxy, nil
		}
	}
	return nil, nil
}

func (cp *controlPlane) allocateAvailableProxyPort(excludeProxyID uint) (int, error) {
	const maxAttempts = 32
	for i := 0; i < maxAttempts; i++ {
		port, err := probeFreeTCPPort()
		if err != nil {
			return 0, err
		}
		conflict, err := cp.findProxyPortConflict(port, excludeProxyID)
		if err != nil {
			return 0, err
		}
		if conflict == nil {
			return port, nil
		}
	}
	return 0, fmt.Errorf("failed to allocate an available public port after %d attempts", maxAttempts)
}

func probeFreeTCPPort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, errors.New("allocated listener is not tcp")
	}
	return addr.Port, nil
}

type lifecycleDeleteTracker struct {
	proxies      map[uint]bool
	applications map[uint]bool
	edges        map[uint64]bool
	devices      map[uint]bool
}

func newLifecycleDeleteTracker() *lifecycleDeleteTracker {
	return &lifecycleDeleteTracker{
		proxies:      map[uint]bool{},
		applications: map[uint]bool{},
		edges:        map[uint64]bool{},
		devices:      map[uint]bool{},
	}
}

func (cp *controlPlane) deleteProxyCascade(proxyID uint, tracker *lifecycleDeleteTracker) error {
	if tracker == nil {
		tracker = newLifecycleDeleteTracker()
	}
	if tracker.proxies[proxyID] {
		return nil
	}
	tracker.proxies[proxyID] = true

	proxy, err := cp.repo.GetProxyByID(proxyID)
	if err != nil {
		return err
	}
	if err := cp.stopProxyRuntime(proxy); err != nil {
		return fmt.Errorf("stop proxy %d runtime: %w", proxyID, err)
	}
	if err := cp.repo.DeleteFirewallRuleByProxyID(proxyID); err != nil {
		return fmt.Errorf("delete firewall rule for proxy %d: %w", proxyID, err)
	}
	return cp.repo.DeleteProxy(proxyID)
}

func (cp *controlPlane) deleteApplicationCascade(applicationID uint, tracker *lifecycleDeleteTracker) error {
	if tracker == nil {
		tracker = newLifecycleDeleteTracker()
	}
	if tracker.applications[applicationID] {
		return nil
	}
	tracker.applications[applicationID] = true

	proxies, err := cp.repo.ListProxies(&dao.ListProxiesQuery{ApplicationIDs: []uint{applicationID}})
	if err != nil {
		return err
	}
	for _, proxy := range proxies {
		if err := cp.deleteProxyCascade(proxy.ID, tracker); err != nil {
			return err
		}
	}
	return cp.repo.DeleteApplication(applicationID)
}

func (cp *controlPlane) deleteEdgeCascade(edgeID uint64, tracker *lifecycleDeleteTracker) error {
	if tracker == nil {
		tracker = newLifecycleDeleteTracker()
	}
	if tracker.edges[edgeID] {
		return nil
	}
	tracker.edges[edgeID] = true

	applications, err := cp.listApplicationsByEdgeID(edgeID)
	if err != nil {
		return err
	}
	for _, application := range applications {
		if err := cp.deleteApplicationCascade(application.ID, tracker); err != nil {
			return err
		}
	}
	if err := cp.failActiveEdgeTasks(edgeID, "edge deleted"); err != nil {
		return err
	}
	if err := cp.repo.DeleteAccessKeysByEdgeID(edgeID); err != nil {
		return err
	}
	if cp.frontierBound != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := cp.frontierBound.KickEdge(ctx, edgeID); err != nil {
			log.Warnf("kick deleted edge %d failed: %s", edgeID, err)
		}
		cancel()
	}
	if err := cp.repo.DeleteEdge(edgeID); err != nil {
		return err
	}
	if err := cp.deleteEdgeOwnedDevicesCascade(edgeID, tracker); err != nil {
		return err
	}
	if err := cp.repo.DeleteEdgeDevicesByEdgeID(edgeID, nil); err != nil {
		return err
	}
	return nil
}

func (cp *controlPlane) deleteEdgeOwnedDevicesCascade(edgeID uint64, tracker *lifecycleDeleteTracker) error {
	edgeDevices, err := cp.repo.GetEdgeDevicesByEdgeID(edgeID, nil)
	if err != nil {
		return err
	}
	deviceIDs := map[uint]bool{}
	for _, edgeDevice := range edgeDevices {
		deviceIDs[edgeDevice.DeviceID] = true
	}

	for deviceID := range deviceIDs {
		if tracker != nil && tracker.devices[deviceID] {
			continue
		}
		relations, err := cp.repo.GetEdgeDevicesByDeviceID(deviceID, nil)
		if err != nil {
			return err
		}
		ownedByDeletingEdges := true
		for _, relation := range relations {
			if relation.EdgeID == edgeID {
				continue
			}
			if tracker == nil || !tracker.edges[relation.EdgeID] {
				ownedByDeletingEdges = false
				break
			}
		}
		if !ownedByDeletingEdges {
			continue
		}
		if err := cp.deleteDeviceCascade(deviceID, tracker); err != nil {
			return err
		}
	}
	return nil
}

func (cp *controlPlane) deleteDeviceCascade(deviceID uint, tracker *lifecycleDeleteTracker) error {
	if tracker == nil {
		tracker = newLifecycleDeleteTracker()
	}
	if tracker.devices[deviceID] {
		return nil
	}
	tracker.devices[deviceID] = true

	hostType := model.EdgeDeviceRelationHost
	edgeDevices, err := cp.repo.GetEdgeDevicesByDeviceID(deviceID, &hostType)
	if err != nil {
		return err
	}
	for _, edgeDevice := range edgeDevices {
		if err := cp.deleteEdgeCascade(edgeDevice.EdgeID, tracker); err != nil {
			return err
		}
	}

	applications, err := cp.repo.ListApplications(&dao.ListApplicationsQuery{DeviceIDs: []uint{deviceID}})
	if err != nil {
		return err
	}
	for _, application := range applications {
		if err := cp.deleteApplicationCascade(application.ID, tracker); err != nil {
			return err
		}
	}
	return cp.repo.DeleteDevice(deviceID)
}

func (cp *controlPlane) listApplicationsByEdgeID(edgeID uint64) ([]*model.Application, error) {
	applications, err := cp.repo.ListApplications(&dao.ListApplicationsQuery{})
	if err != nil {
		return nil, err
	}
	filtered := make([]*model.Application, 0)
	for _, application := range applications {
		for _, appEdgeID := range application.EdgeIDs {
			if uint64(appEdgeID) == edgeID {
				filtered = append(filtered, application)
				break
			}
		}
	}
	return filtered, nil
}

func (cp *controlPlane) listProxiesByEdgeID(edgeID uint64) ([]*model.Proxy, error) {
	applications, err := cp.listApplicationsByEdgeID(edgeID)
	if err != nil {
		return nil, err
	}
	if len(applications) == 0 {
		return []*model.Proxy{}, nil
	}
	applicationIDs := make([]uint, 0, len(applications))
	applicationMap := make(map[uint]*model.Application, len(applications))
	for _, application := range applications {
		applicationIDs = append(applicationIDs, application.ID)
		applicationMap[application.ID] = application
	}
	proxies, err := cp.repo.ListProxies(&dao.ListProxiesQuery{ApplicationIDs: applicationIDs})
	if err != nil {
		return nil, err
	}
	for _, proxy := range proxies {
		proxy.Application = applicationMap[proxy.ApplicationID]
	}
	return proxies, nil
}

func (cp *controlPlane) startAndPersistProxyRuntime(proxy *model.Proxy) error {
	if proxy == nil {
		return nil
	}
	oldPort := proxy.Port
	if err := cp.startProxyRuntime(proxy, proxy.Application); err != nil {
		return err
	}
	if oldPort == 0 && proxy.Port > 0 {
		if err := cp.repo.UpdateProxy(proxy); err != nil {
			_ = cp.stopProxyRuntime(proxy)
			return err
		}
	}
	return nil
}

func (cp *controlPlane) stopEdgeProxies(edgeID uint64) error {
	proxies, err := cp.listProxiesByEdgeID(edgeID)
	if err != nil {
		return err
	}
	for _, proxy := range proxies {
		if proxy.Status != model.ProxyStatusRunning {
			continue
		}
		if err := cp.stopProxyRuntime(proxy); err != nil {
			return err
		}
	}
	return nil
}

func (cp *controlPlane) startEdgeProxies(edgeID uint64) error {
	proxies, err := cp.listProxiesByEdgeID(edgeID)
	if err != nil {
		return err
	}
	started := make([]*model.Proxy, 0, len(proxies))
	for _, proxy := range proxies {
		if proxy.Status != model.ProxyStatusRunning {
			continue
		}
		if err := cp.startAndPersistProxyRuntime(proxy); err != nil {
			for _, startedProxy := range started {
				_ = cp.stopProxyRuntime(startedProxy)
			}
			return err
		}
		started = append(started, proxy)
	}
	return nil
}

func (cp *controlPlane) failActiveEdgeTasks(edgeID uint64, reason string) error {
	tasks, err := cp.repo.ListTasks(&dao.ListTasksQuery{
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
		if err := cp.repo.UpdateTaskError(task.ID, reason); err != nil {
			return err
		}
	}
	return nil
}
