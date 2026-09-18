package controlplane

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/liaisonio/liaison/pkg/trafficconn"
)

func httpEntryMode(p *model.Proxy) string {
	if p.HTTPEntryMode == "" {
		return "port"
	}
	return p.HTTPEntryMode
}

func (cp *controlPlane) validateHTTPEntryMode(protocol model.AccessProtocol, mode string) error {
	if mode == "" {
		return nil
	}
	if protocol != model.AccessProtocolHTTP {
		return badRequest("HTTP_ENTRY_PROTOCOL", "入口方式仅适用于 Web 访问")
	}
	if mode != "path" && mode != "port" && mode != "domain" {
		return badRequest("HTTP_ENTRY_MODE", "入口方式无效")
	}
	if mode == "domain" && !cp.conf.Manager.WebDomainReady() {
		return badRequest("HTTP_DOMAIN_UNAVAILABLE", "请先配置入口域名和匹配的 HTTPS 证书")
	}
	return nil
}

func sharedHTTPEntry(p *model.Proxy, app *model.Application) bool {
	return p != nil && app != nil && effectiveAccessProtocol(p, app) == model.AccessProtocolHTTP && (httpEntryMode(p) == "path" || httpEntryMode(p) == "domain")
}

func (cp *controlPlane) httpEntryURL(p *model.Proxy) string {
	if httpEntryMode(p) == "path" {
		return strings.TrimRight(cp.conf.Manager.ServerURL, "/") + fmt.Sprintf("/access/%d/web/", p.ID)
	}
	u, err := url.Parse(cp.conf.Manager.ServerURL)
	port := ""
	if err == nil && u.Port() != "" && u.Port() != "443" {
		port = ":" + u.Port()
	}
	return fmt.Sprintf("https://a-%d.%s%s/", p.ID, strings.ToLower(strings.TrimSpace(cp.conf.Manager.WebDomain)), port)
}

type HTTPEntryTarget struct{ Mode, URL, Address string }

func (cp *controlPlane) HTTPEntrySourceAllowed(id uint, remote string) bool {
	rule, err := cp.repo.GetFirewallRuleByProxyID(id)
	if err != nil {
		return false
	}
	if rule == nil {
		return true
	}
	host, _, err := net.SplitHostPort(remote)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, cidr := range rule.AllowedCIDRs {
		_, network, e := net.ParseCIDR(cidr)
		if e == nil && network.Contains(ip) {
			return true
		}
	}
	return false
}

func (cp *controlPlane) HTTPEntryTarget(ctx context.Context, id uint) (*HTTPEntryTarget, error) {
	if err := requireVisibleResource(ctx, cp.repo, resourceAccess, uint64(id)); err != nil {
		return nil, err
	}
	p, err := cp.repo.GetProxyByID(id)
	if err != nil {
		return nil, err
	}
	app, err := cp.repo.GetApplicationByID(p.ApplicationID)
	if err != nil {
		return nil, err
	}
	if !sharedHTTPEntry(p, app) {
		return nil, badRequest("HTTP_ENTRY_REQUIRED", "访问不是共享 Web 入口")
	}
	if err := cp.validateHTTPEntryMode(model.AccessProtocolHTTP, httpEntryMode(p)); err != nil {
		return nil, err
	}
	if status, _ := cp.proxyEffectiveStatus(p, app); status != proxyEffectiveStatusActive {
		return nil, conflict("HTTP_ENTRY_UNAVAILABLE", "访问暂不可用")
	}
	return &HTTPEntryTarget{Mode: httpEntryMode(p), URL: cp.httpEntryURL(p), Address: net.JoinHostPort(app.IP, fmt.Sprint(app.Port))}, nil
}

func (cp *controlPlane) OpenHTTPEntryStream(ctx context.Context, id uint) (net.Conn, error) {
	if _, err := cp.HTTPEntryTarget(ctx, id); err != nil {
		return nil, err
	}
	p, err := cp.repo.GetProxyByID(id)
	if err != nil {
		return nil, err
	}
	app, err := cp.repo.GetApplicationByID(p.ApplicationID)
	if err != nil {
		return nil, err
	}
	if cp.frontierBound == nil || len(app.EdgeIDs) == 0 {
		return nil, fmt.Errorf("connector unavailable")
	}
	stream, err := cp.frontierBound.OpenStream(ctx, uint64(app.EdgeIDs[0]))
	if err != nil {
		return nil, err
	}
	conn := trafficconn.TargetConn(stream, cp.trafficRecorder, p.ID, app.ID)
	data, err := json.Marshal(proto.Dst{Addr: net.JoinHostPort(app.IP, fmt.Sprint(app.Port)), ApplicationID: app.ID, ProxyID: p.ID})
	if err != nil {
		conn.Close()
		return nil, err
	}
	frame := make([]byte, 4, len(data)+4)
	binary.BigEndian.PutUint32(frame, uint32(len(data)))
	frame = append(frame, data...)
	if _, err = conn.Write(frame); err != nil {
		conn.Close()
		return nil, err
	}
	return conn, nil
}
