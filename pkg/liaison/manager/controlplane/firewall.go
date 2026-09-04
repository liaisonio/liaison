package controlplane

import (
	"context"
	"fmt"
	"net"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/liaison/manager/timefmt"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
)

const defaultAllowAllCIDR = "0.0.0.0/0"

// FirewallData is the API-level representation of a proxy's firewall rule.
type FirewallData struct {
	ProxyID      uint     `json:"proxy_id"`
	AllowedCIDRs []string `json:"allowed_cidrs"`
	UpdatedAt    string   `json:"updated_at"`
}

// GetProxyFirewall returns the current allowlist for a proxy. If no rule
// exists, the response advertises allow-all via a single 0.0.0.0/0 entry.
func (cp *controlPlane) GetProxyFirewall(ctx context.Context, proxyID uint) (*FirewallData, error) {
	if _, err := cp.getProxyForFirewall(ctx, proxyID); err != nil {
		return nil, err
	}

	rule, err := cp.repo.GetFirewallRuleByProxyID(proxyID)
	if err != nil {
		return nil, err
	}
	if rule == nil {
		return &FirewallData{
			ProxyID:      proxyID,
			AllowedCIDRs: []string{defaultAllowAllCIDR},
		}, nil
	}
	return &FirewallData{
		ProxyID:      proxyID,
		AllowedCIDRs: []string(rule.AllowedCIDRs),
		UpdatedAt:    timefmt.FormatDateTime(rule.UpdatedAt),
	}, nil
}

// UpsertProxyFirewall creates or replaces the source-IP allowlist for a proxy.
// An empty cidrs slice means "deny all".
func (cp *controlPlane) UpsertProxyFirewall(ctx context.Context, proxyID uint, cidrs []string) (*FirewallData, error) {
	proxy, err := cp.getProxyForFirewall(ctx, proxyID)
	if err != nil {
		return nil, err
	}

	for _, cidr := range cidrs {
		if _, _, err := net.ParseCIDR(cidr); err != nil {
			return nil, fmt.Errorf("invalid CIDR %q: %w", cidr, err)
		}
	}

	rule := &model.ProxyFirewallRule{
		ProxyID:      proxyID,
		AllowedCIDRs: model.StringSlice(cidrs),
	}
	if err := cp.repo.UpsertFirewallRule(rule); err != nil {
		return nil, err
	}

	// Push to the data plane. Best-effort: if the proxy isn't running yet the
	// rule will be re-applied when startProxyRuntime is next invoked.
	if cp.firewallManager != nil && proxy.Port > 0 {
		if err := cp.firewallManager.Allow(int(proxyID), cidrs); err != nil {
			log.Warnf("firewall: Allow proxy=%d port=%d failed: %v", proxyID, proxy.Port, err)
		}
	}

	return &FirewallData{
		ProxyID:      proxyID,
		AllowedCIDRs: cidrs,
	}, nil
}

// DeleteProxyFirewall removes the allowlist for a proxy, restoring allow-all.
func (cp *controlPlane) DeleteProxyFirewall(ctx context.Context, proxyID uint) error {
	proxy, err := cp.getProxyForFirewall(ctx, proxyID)
	if err != nil {
		return err
	}

	if err := cp.repo.DeleteFirewallRuleByProxyID(proxyID); err != nil {
		return err
	}

	if cp.firewallManager != nil && proxy.Port > 0 {
		cp.firewallManager.Revoke(int(proxyID))
	}
	return nil
}

// getProxyForFirewall loads the target proxy and validates it's firewallable.
// In this private-deployment build every proxy (HTTP or TCP) has its own
// fixed listen port, so L4 CIDR matching is always feasible — no protocol
// restriction is applied.
func (cp *controlPlane) getProxyForFirewall(ctx context.Context, proxyID uint) (*model.Proxy, error) {
	if err := requireVisibleResource(ctx, cp.repo, resourceAccess, uint64(proxyID)); err != nil {
		return nil, err
	}
	proxy, err := cp.repo.GetProxyByID(proxyID)
	if err != nil {
		return nil, mapRecordNotFound(err, "PROXY_NOT_FOUND", "访问不存在")
	}
	if proxy == nil {
		return nil, notFound("PROXY_NOT_FOUND", "访问不存在", nil)
	}
	return proxy, nil
}
