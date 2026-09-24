package controlplane

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	v1 "github.com/liaisonio/liaison/api/v1"
	"github.com/liaisonio/liaison/pkg/liaison/manager/aigateway"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/repo/dao"
	"github.com/liaisonio/liaison/pkg/liaison/repo/model"
	"github.com/liaisonio/liaison/pkg/proto"
)

var ErrAIExistingApplication = errors.New("model application already exists")

type AISetupRequest struct {
	EdgeID          uint64            `json:"edge_id"`
	ServiceURL      string            `json:"service_url"`
	Protocol        string            `json:"protocol"`
	APIKey          string            `json:"api_key"`
	ApplicationName string            `json:"application_name"`
	Name            string            `json:"name"`
	Description     string            `json:"description"`
	Models          map[string]string `json:"models"`
}

type AISetupResult struct {
	ID            uint `json:"id"`
	ApplicationID uint `json:"application_id"`
}

func parseAISetup(v AISetupRequest) (aigateway.Target, AIApplicationConfig, error) {
	p, known := aigateway.LookupProtocol(v.Protocol)
	u, err := url.Parse(strings.TrimSpace(v.ServiceURL))
	if err != nil || !known || v.EdgeID == 0 || len(v.ServiceURL) > 2048 || len(v.APIKey) > 8192 || strings.ContainsAny(v.APIKey, "\r\n") || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.Contains(v.ServiceURL, "#") || u.RawPath != "" || !isValidApplicationHost(u.Hostname()) {
		return aigateway.Target{}, AIApplicationConfig{}, ErrAIInvalid
	}
	port := 80
	if u.Scheme == "https" {
		port = 443
	}
	if u.Port() != "" {
		port, err = strconv.Atoi(u.Port())
	}
	if err != nil || port < 1 || port > 65535 || strings.HasSuffix(u.Host, ":") {
		return aigateway.Target{}, AIApplicationConfig{}, ErrAIInvalid
	}
	base := strings.TrimSuffix(u.Path, "/")
	if base == "" {
		base = p.BasePath
	}
	target := aigateway.Target{Host: strings.ToLower(u.Hostname()), Port: port, TLS: u.Scheme == "https", BasePath: base}
	upstream, err := aigateway.NewUpstream(target, func(context.Context) (net.Conn, error) { return nil, ErrAIUnavailable })
	if err != nil {
		return target, AIApplicationConfig{}, ErrAIInvalid
	}
	upstream.Close()
	return target, AIApplicationConfig{Protocol: v.Protocol, BasePath: base, TLS: target.TLS, APIKey: v.APIKey}, nil
}

func (s *AIService) authorizeSetup(ctx context.Context, edgeID uint64) error {
	for _, permission := range [][2]string{{"applications", "create"}, {"applications", "update"}, {"accesses", "create"}, {"accesses", "update"}} {
		if _, err := s.actor(ctx, permission[0], permission[1]); err != nil {
			return err
		}
	}
	if err := requireVisibleResource(ctx, s.cp.repo, resourceConnector, edgeID); err != nil {
		return iam.ErrForbidden
	}
	_, err := s.cp.repo.GetEdge(edgeID)
	return err
}

// ProbeSetup uses a transient target over the authorized connector. No application,
// credential, access or traffic attribution is persisted by discovery.
func (s *AIService) ProbeSetup(ctx context.Context, v AISetupRequest) (aigateway.ProbeResult, error) {
	target, _, err := parseAISetup(v)
	if err != nil {
		return aigateway.ProbeResult{}, err
	}
	if err = s.authorizeSetup(ctx, v.EdgeID); err != nil {
		return aigateway.ProbeResult{}, err
	}
	if !s.cp.probeLimiter.Allow() {
		return aigateway.ProbeResult{State: "busy"}, nil
	}
	select {
	case s.cp.probeSlots <- struct{}{}:
		defer func() { <-s.cp.probeSlots }()
	default:
		return aigateway.ProbeResult{State: "busy"}, nil
	}
	u, err := aigateway.NewUpstream(target, func(next context.Context) (net.Conn, error) {
		if err := s.authorizeSetup(next, v.EdgeID); err != nil {
			return nil, err
		}
		edge, err := s.cp.repo.GetEdge(v.EdgeID)
		if err != nil || edge.Status != model.EdgeStatusRunning || edge.Online != model.EdgeOnlineStatusOnline || s.cp.frontierBound == nil {
			return nil, ErrAIUnavailable
		}
		stream, err := s.cp.frontierBound.OpenStream(next, v.EdgeID)
		if err != nil {
			return nil, err
		}
		data, err := json.Marshal(proto.Dst{Addr: net.JoinHostPort(target.Host, strconv.Itoa(target.Port))})
		if err != nil {
			stream.Close()
			return nil, err
		} // Close the failed tunnel; preserve the original error.
		frame := make([]byte, 4+len(data))
		binary.BigEndian.PutUint32(frame, uint32(len(data)))
		copy(frame[4:], data)
		stop := context.AfterFunc(next, func() { stream.Close() })
		n, err := stream.Write(frame)
		stop()
		if err != nil || n != len(frame) {
			stream.Close()
			if err == nil {
				err = io.ErrShortWrite
			}
			return nil, err
		}
		return stream, nil
	})
	if err != nil {
		return aigateway.ProbeResult{}, err
	}
	defer u.Close()
	return u.Probe(ctx, v.APIKey, v.Protocol), nil
}

// CreateSetup commits the application, ownership, encrypted upstream and access
// together. AI accesses have no listener/runtime side effects to roll back.
func (s *AIService) CreateSetup(ctx context.Context, v AISetupRequest) (result AISetupResult, err error) {
	target, config, err := parseAISetup(v)
	if err != nil {
		return result, err
	}
	if err = s.authorizeSetup(ctx, v.EdgeID); err != nil {
		return result, err
	}
	if len(v.Models) == 0 || len(v.Models) > 100 || len(aigateway.AllowedModels(aigateway.ModelAliases(v.Models), v.Models)) != len(v.Models) || len(v.Description) > 4096 {
		return result, ErrAIInvalid
	}
	name, err := normalizeResourceName(v.Name, "Access")
	if err != nil {
		return result, ErrAIInvalid
	}
	if v.ApplicationName == "" {
		v.ApplicationName = fmt.Sprintf("%s:%d", target.Host, target.Port)
	}
	appName, err := normalizeResourceName(v.ApplicationName, "App")
	if err != nil {
		return result, ErrAIInvalid
	}
	s.setupMu.Lock()
	defer s.setupMu.Unlock()
	ids, scoped, err := visibleResourceIDs(ctx, s.cp.repo, resourceApplication)
	if err != nil {
		return result, err
	}
	query := &dao.ListApplicationsQuery{Query: dao.Query{ScopeApplied: scoped}}
	for _, id := range ids {
		query.IDs = append(query.IDs, uint(id))
	}
	apps, err := s.cp.repo.ListApplications(query)
	if err != nil {
		return result, err
	}
	appType := v.Protocol
	if appType == "openai-compatible" {
		appType = "openai"
	}
	for _, app := range apps {
		if strings.EqualFold(app.IP, target.Host) && app.Port == target.Port && aiApplicationProtocolMatches(app.ApplicationType, v.Protocol) {
			for _, id := range app.EdgeIDs {
				if uint64(id) == v.EdgeID {
					return result, ErrAIExistingApplication
				}
			}
		}
	}
	tx := s.cp.repo.Begin()
	defer func() {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			err = errors.Join(err, rollbackErr)
		}
	}()
	cp := &controlPlane{repo: tx}
	created, err := cp.CreateApplication(ctx, &v1.CreateApplicationRequest{Name: appName, ApplicationType: appType, EdgeId: v.EdgeID, Ip: target.Host, Port: int32(target.Port)})
	if err != nil {
		return result, err
	}
	appID := uint(created.Data.Id)
	app, err := tx.GetApplicationByID(appID)
	if err != nil {
		return result, err
	}
	fingerprint := aiFingerprint(app, &config)
	encrypted := ""
	if config.APIKey != "" {
		nonce := make([]byte, s.cipher.NonceSize())
		if _, err = rand.Read(nonce); err != nil {
			return result, err
		}
		encrypted = base64.StdEncoding.EncodeToString(s.cipher.Seal(nonce, nonce, []byte(config.APIKey), []byte(fingerprint)))
	}
	if err = tx.SaveAIApplication(ctx, &model.AIApplication{ApplicationID: appID, Protocol: config.Protocol, BasePath: config.BasePath, TLS: config.TLS, EncryptedKey: encrypted, TargetFingerprint: fingerprint}); err != nil {
		return result, err
	}
	proxy := &model.Proxy{Name: name, Description: v.Description, ApplicationID: appID, AccessProtocol: model.AccessProtocolAI, Status: model.ProxyStatusRunning}
	if err = tx.CreateProxy(proxy); err != nil {
		return result, err
	}
	if err = claimResource(ctx, tx, resourceAccess, uint64(proxy.ID)); err != nil {
		return result, err
	}
	encodedModels, err := json.Marshal(v.Models)
	if err != nil {
		return result, err
	}
	if err = tx.SaveAIAccess(ctx, &model.AIAccess{ProxyID: proxy.ID, Enabled: true, Models: string(encodedModels)}); err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = tx.Commit(); err != nil {
		return result, err
	}
	return AISetupResult{ID: proxy.ID, ApplicationID: appID}, nil
}
