package liaison

import (
	"context"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"

	"github.com/liaisonio/liaison/pkg/entry"
	"github.com/liaisonio/liaison/pkg/liaison/config"
	"github.com/liaisonio/liaison/pkg/liaison/manager/accesssession"
	agentapplication "github.com/liaisonio/liaison/pkg/liaison/manager/agent/application"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/assistance"
	agentexecutor "github.com/liaisonio/liaison/pkg/liaison/manager/agent/executor"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/management"
	"github.com/liaisonio/liaison/pkg/liaison/manager/agent/modelsettings"
	agentpolicy "github.com/liaisonio/liaison/pkg/liaison/manager/agent/policy"
	agentruntime "github.com/liaisonio/liaison/pkg/liaison/manager/agent/runtime"
	agenttool "github.com/liaisonio/liaison/pkg/liaison/manager/agent/tool"
	"github.com/liaisonio/liaison/pkg/liaison/manager/controlplane"
	"github.com/liaisonio/liaison/pkg/liaison/manager/frontierbound"
	"github.com/liaisonio/liaison/pkg/liaison/manager/iam"
	"github.com/liaisonio/liaison/pkg/liaison/manager/traffic"
	"github.com/liaisonio/liaison/pkg/liaison/manager/web"
	"github.com/liaisonio/liaison/pkg/liaison/repo"
	"github.com/liaisonio/liaison/pkg/utils"
	"k8s.io/klog/v2"
)

type Liaison struct {
	web              web.Web
	frontierBound    frontierbound.FrontierBound
	entry            *entry.Entry
	repo             repo.Repo
	iamService       *iam.IAMService
	accessSessions   *accesssession.Registry
	trafficCollector *traffic.TrafficCollector
}

func NewLiaison() (*Liaison, error) {
	err := config.Init()
	if err != nil {
		return nil, err
	}
	// pprof & rlimit
	if config.Conf.Daemon.PProf.Enable {
		runtime.SetCPUProfileRate(config.Conf.Daemon.PProf.CPUProfileRate)
		go func() {
			http.ListenAndServe(config.Conf.Daemon.PProf.Addr, nil)
		}()
	}
	// rlimit
	if config.Conf.Daemon.RLimit.Enable {
		err = utils.SetRLimit(uint64(config.Conf.Daemon.RLimit.NumFile))
		if err != nil {
			klog.Errorf("set rlimit err: %s", err)
			return nil, err
		}
	}
	// Bind the public manager listener before registering RPCs with Frontier.
	// A duplicate manager process must fail here without touching Frontier's
	// service registry; otherwise its short-lived service connection can remove
	// RPC routes used by the healthy manager process.
	webListener, err := web.NewListener(config.Conf)
	if err != nil {
		return nil, err
	}
	listenerOwned := true
	defer func() {
		if listenerOwned {
			_ = webListener.Close()
		}
	}()

	// repo
	repo, err := repo.NewRepo(config.Conf)
	if err != nil {
		return nil, err
	}
	repoOwned := true
	defer func() {
		if repoOwned {
			_ = repo.Close()
		}
	}()
	// traffic collector
	trafficCollector := traffic.NewTrafficCollector(repo)
	trafficCollectorOwned := true
	defer func() {
		if trafficCollectorOwned {
			trafficCollector.Stop()
		}
	}()
	// frontier bound
	frontierBound, err := frontierbound.NewFrontierBound(config.Conf, repo, trafficCollector)
	if err != nil {
		return nil, err
	}
	frontierBoundOwned := true
	defer func() {
		if frontierBoundOwned {
			_ = frontierBound.Close()
		}
	}()
	// IAM service
	iamService, err := iam.NewIAMService(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize authorization: %w", err)
	}
	if err := iamService.EnsureOrganizationBootstrap(); err != nil {
		return nil, fmt.Errorf("failed to initialize organizations: %w", err)
	}
	controlPlane, err := controlplane.NewControlPlane(config.Conf, repo, frontierBound, trafficCollector, func(ctx context.Context, feature string) error {
		id, ok := ctx.Value("user_id").(uint)
		if !ok || id == 0 {
			return iam.ErrForbidden
		}
		actor, err := repo.GetUserByID(id)
		if err != nil {
			return err
		}
		return iamService.RequireFeature(actor, feature)
	})
	if err != nil {
		return nil, err
	}
	// 设置JWT密钥（必须从配置文件读取）
	if config.Conf.Manager.JWTSecret == "" {
		return nil, fmt.Errorf("JWT secret key is required in configuration file. Please set 'manager.jwt_secret' in your configuration")
	}
	if err := utils.SetJWTSecret(config.Conf.Manager.JWTSecret); err != nil {
		return nil, fmt.Errorf("failed to set JWT secret: %w", err)
	}
	// web layer
	accessSessions := accesssession.NewRegistry()
	agentStore, err := agentruntime.NewDurableStore(repo)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent store: %w", err)
	}
	agentBinder, err := agentexecutor.NewAttachmentBinder(accessSessions)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent attachment binder: %w", err)
	}
	agentEvents := agentruntime.NewEventBroker(128)
	agentConfig := config.Conf.Manager.Agent
	models, err := modelsettings.New(repo, config.Conf.Manager.JWTSecret, modelsettings.Config{
		Enabled: agentConfig.Enabled, BaseURL: agentConfig.BaseURL, Model: agentConfig.Model, APIKey: os.Getenv(agentConfig.APIKeyEnv),
	}, func(ctx context.Context, userID uint, action string) error {
		actor, err := iamService.GetUserByID(userID)
		if err != nil {
			return err
		}
		return iamService.RequireModelSettingsPermission(actor, action)
	})
	if err != nil {
		return nil, err
	}
	agentLoop, err := newAgentLoop(agentConfig, agentStore, repo, iamService, accessSessions, agentEvents, models, controlPlane)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent runtime: %w", err)
	}
	generator, err := assistance.NewModelGenerator(models)
	if err != nil {
		return nil, err
	}
	agentOptions := []agentapplication.Option{agentapplication.WithTurnRunner(agentLoop), agentapplication.WithAssistanceGenerator(generator), agentapplication.WithModelSettings(models), agentapplication.WithResourceReferences(controlPlane)}
	agentService, err := agentapplication.NewService(agentStore, agentBinder, repo, iamService, nil, agentOptions...)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent application service: %w", err)
	}
	webServer, err := web.NewWebServerWithListener(config.Conf, controlPlane, iamService, webListener, accessSessions, agentService, agentEvents)
	if err != nil {
		return nil, err
	}
	listenerOwned = false
	webOwned := true
	defer func() {
		if webOwned {
			_ = webServer.Close()
		}
	}()
	// entry layer
	entry, err := entry.NewEntry(config.Conf, controlPlane, trafficCollector)
	if err != nil {
		return nil, err
	}
	entryOwned := true
	defer func() {
		if entryOwned {
			_ = entry.Close()
		}
	}()
	// 把持久化的防火墙规则推回数据面 —— entry 此时已经把 proxies 起起来了，
	// 在这里恢复 CIDR 白名单可以避免重启后的短暂宽松窗口。
	controlPlane.RestoreFirewallRules()
	webOwned = false
	frontierBoundOwned = false
	entryOwned = false
	trafficCollectorOwned = false
	repoOwned = false
	return &Liaison{
		web:              webServer,
		frontierBound:    frontierBound,
		entry:            entry,
		repo:             repo,
		iamService:       iamService,
		accessSessions:   accessSessions,
		trafficCollector: trafficCollector,
	}, nil
}

func newAgentLoop(agentConfig config.Agent, store agentruntime.Store, repository repo.Repo, iamService *iam.IAMService,
	accessSessions *accesssession.Registry, events agentruntime.EventSink, provider agentruntime.ModelProvider, cp management.ControlPlane,
) (*agentruntime.Loop, error) {
	policy, err := agentpolicy.NewAuthorizationPolicy(func(_ context.Context, principal agenttool.Principal, resource, action string) error {
		actor, getErr := iamService.GetUserByID(principal.UserID)
		if getErr != nil {
			return getErr
		}
		return iamService.RequireOrganizationResourcePermission(actor, principal.OrganizationID, resource, action)
	})
	if err != nil {
		return nil, err
	}
	managementSource, err := management.NewSource(cp, iamService)
	if err != nil {
		return nil, err
	}
	engine := agenttool.NewEngine(managementSource, policy)
	if err := agenttool.NewSourceManager(engine).Load(context.Background(), managementSource); err != nil {
		return nil, fmt.Errorf("load management tools: %w", err)
	}
	router := agentexecutor.NewRouter()
	sessionExecutor, err := agentexecutor.NewSessionExecutor(accessSessions)
	if err != nil {
		return nil, err
	}
	if err := router.Register(sessionExecutor); err != nil {
		return nil, fmt.Errorf("register access session executor: %w", err)
	}
	if err := agenttool.NewSourceManager(engine).Load(context.Background(), agentexecutor.NewToolSource(router)); err != nil {
		return nil, fmt.Errorf("load protocol tools: %w", err)
	}
	if err := agenttool.RegisterDiscoveryTools(context.Background(), engine); err != nil {
		return nil, err
	}
	approvals, err := agentruntime.NewDurableApprovalCoordinator(repository)
	if err != nil {
		return nil, err
	}
	return agentruntime.NewLoop(store, engine, provider, approvals, events, nil, agentruntime.LoopConfig{
		MaxModelSteps:  agentConfig.MaxModelSteps,
		ApprovalExpiry: agentConfig.ApprovalExpiry,
	})
}

func (l *Liaison) Serve() error {
	return l.web.Serve()
}

func (l *Liaison) Close() error {
	err := l.web.Close()
	if err != nil {
		return err
	}
	err = l.frontierBound.Close()
	if err != nil {
		return err
	}
	err = l.entry.Close()
	if err != nil {
		return err
	}
	if l.trafficCollector != nil {
		l.trafficCollector.Stop()
	}
	err = l.repo.Close()
	if err != nil {
		return err
	}
	return nil
}
