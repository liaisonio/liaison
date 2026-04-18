package controlplane

import (
	"context"
	"fmt"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

// stopProxyRuntime stops the data-plane listener for proxy and revokes any
// associated firewall rule from the in-memory kernel. Safe to call on an
// already-stopped proxy or when no proxyManager is registered.
func (cp *controlPlane) stopProxyRuntime(proxy *model.Proxy) error {
	if proxy == nil || cp.proxyManager == nil {
		return nil
	}
	if proxy.Status != model.ProxyStatusRunning {
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
	if proxy.Status != model.ProxyStatusRunning {
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
		if err := cp.firewallManager.Allow(int(rule.ProxyID), []string(rule.AllowedCIDRs)); err != nil {
			log.Warnf("firewall: restore proxy=%d failed: %v", rule.ProxyID, err)
		}
	}
	log.Infof("firewall: restored %d rule(s) from DB", len(rules))
}
