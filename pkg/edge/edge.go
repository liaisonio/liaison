package edge

import (
	"context"
	"errors"
	"net/http"
	_ "net/http/pprof"
	"path/filepath"
	"runtime"

	"github.com/jumboframes/armorigo/log"
	agentbridge "github.com/liaisonio/liaison/pkg/edge/agent/bridge"
	"github.com/liaisonio/liaison/pkg/edge/config"
	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
	"github.com/liaisonio/liaison/pkg/edge/lifecycle"
	"github.com/liaisonio/liaison/pkg/edge/pinger"
	"github.com/liaisonio/liaison/pkg/edge/probe"
	"github.com/liaisonio/liaison/pkg/edge/proxy"
	"github.com/liaisonio/liaison/pkg/edge/reporter"
	"github.com/liaisonio/liaison/pkg/edge/scanner"
	"github.com/liaisonio/liaison/pkg/utils"
	"k8s.io/klog/v2"
)

type Edge struct {
	frontierBound frontierbound.FrontierBound
	agents        *agentbridge.Bridge
}

func NewEdge() (*Edge, error) {
	// pprof & rlimit
	log.Infof("pprof config: enable=%v, addr=%s", config.Conf.Daemon.PProf.Enable, config.Conf.Daemon.PProf.Addr)
	if config.Conf.Daemon.PProf.Enable {
		// 如果 CPUProfileRate 为 0，使用默认值 100
		cpuProfileRate := config.Conf.Daemon.PProf.CPUProfileRate
		if cpuProfileRate == 0 {
			cpuProfileRate = 100
		}
		runtime.SetCPUProfileRate(cpuProfileRate)
		go func() {
			klog.Infof("starting pprof server on %s", config.Conf.Daemon.PProf.Addr)
			log.Infof("starting pprof server on %s", config.Conf.Daemon.PProf.Addr)
			if err := http.ListenAndServe(config.Conf.Daemon.PProf.Addr, nil); err != nil {
				klog.Errorf("pprof server error: %v", err)
				log.Errorf("pprof server error: %v", err)
			}
		}()
	}
	// rlimit
	if config.Conf.Daemon.RLimit.Enable {
		err := utils.SetRLimit(uint64(config.Conf.Daemon.RLimit.NumFile))
		if err != nil {
			klog.Errorf("set rlimit err: %s", err)
			return nil, err
		}
	}

	frontierBound, err := frontierbound.NewFrontierBound(config.Conf)
	if err != nil {
		log.Errorf("init frontier bound error: %v", err)
		return nil, err
	}

	// Lifecycle metadata is additive. Failure to expose it must not break an
	// otherwise valid legacy connection or grant any uninstall capability.
	if identity, identityErr := lifecycle.RuntimeIdentity(config.ConfigFile(), config.Conf.InstanceID); identityErr == nil {
		if registerErr := lifecycle.RegisterStatus(frontierBound, identity, config.Conf.AllowRemoteUninstall, config.Conf.Manager.Dial.Addrs, config.Conf.Manager.Dial.TLS.InsecureSkipVerify); registerErr != nil {
			log.Warnf("installation status capability unavailable")
		}
	}

	_, err = proxy.NewProxy(frontierBound)
	if err != nil {
		log.Errorf("init proxy error: %v", err)
		return nil, err
	}
	if registerErr := probe.Register(frontierBound); registerErr != nil {
		log.Warnf("application probe capability unavailable")
	}

	_, err = reporter.NewReporter(frontierBound)
	if err != nil {
		log.Errorf("init reporter error: %v", err)
		return nil, err
	}

	_, err = scanner.NewScanner(frontierBound)
	if err != nil {
		log.Errorf("init scanner error: %v", err)
		return nil, err
	}

	_, err = pinger.NewPinger(frontierBound)
	if err != nil {
		log.Errorf("init pinger error: %v", err)
		return nil, err
	}

	configPath, pathErr := filepath.Abs(config.ConfigFile())
	var agents *agentbridge.Bridge
	var agentErr error
	if pathErr == nil {
		agents, agentErr = agentbridge.New(context.Background(), filepath.Join(filepath.Dir(configPath), "agent-bindings.json"))
	} else {
		agentErr = pathErr
	}
	if agentErr == nil {
		if agentErr = agents.Register(frontierBound); agentErr != nil {
			if closeErr := agents.Close(); closeErr != nil {
				log.Warnf("agent preview cleanup failed")
			}
			agents = nil
		}
	}
	if agentErr != nil {
		log.Warnf("agent preview capability unavailable")
	}
	return &Edge{
		frontierBound: frontierBound,
		agents:        agents,
	}, nil
}

func (e *Edge) Close() error {
	var agentErr error
	if e.agents != nil {
		agentErr = e.agents.Close()
	}
	return errors.Join(agentErr, e.frontierBound.Close())
}
