package edge

import (
	"net/http"
	_ "net/http/pprof"
	"runtime"

	"github.com/jumboframes/armorigo/log"
	"github.com/liaisonio/liaison/pkg/edge/config"
	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
	"github.com/liaisonio/liaison/pkg/edge/pinger"
	"github.com/liaisonio/liaison/pkg/edge/proxy"
	"github.com/liaisonio/liaison/pkg/edge/reporter"
	"github.com/liaisonio/liaison/pkg/edge/scanner"
	"github.com/liaisonio/liaison/pkg/utils"
	"k8s.io/klog/v2"
)

type Edge struct {
	frontierBound frontierbound.FrontierBound
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

	_, err = proxy.NewProxy(frontierBound)
	if err != nil {
		log.Errorf("init proxy error: %v", err)
		return nil, err
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

	return &Edge{
		frontierBound: frontierBound,
	}, nil
}

func (e *Edge) Close() error {
	return e.frontierBound.Close()
}
