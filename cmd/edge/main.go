package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/sigaction"
	"github.com/liaisonio/liaison/pkg/edge"
	agentcommand "github.com/liaisonio/liaison/pkg/edge/agent/command"
	"github.com/liaisonio/liaison/pkg/edge/config"
	"github.com/liaisonio/liaison/pkg/edge/lifecycle"
)

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--agent-discover" || os.Args[1] == "--agent-check") {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		if err := agentcommand.Run(ctx, os.Args[1:], os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "Agent diagnostic failed:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && (os.Args[1] == "--edge-install-new" || os.Args[1] == "--edge-upgrade-instance") {
		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()
		if err := lifecycle.RunInstallCommand(ctx, os.Args[1:], os.Stdin, os.Stdout); err != nil {
			fmt.Fprintln(os.Stderr, "Edge installation failed:", err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) == 3 && os.Args[1] == "--edge-uninstall-worker" {
		ctx, cancel := context.WithTimeout(context.Background(), 80*time.Second)
		defer cancel()
		if err := lifecycle.RunWorker(ctx, os.Args[2]); err != nil {
			os.Exit(1)
		}
		return
	}
	err := config.Init()
	if err != nil {
		// 如果是显示指纹的错误，正常退出
		if errors.Is(err, config.ErrShowFingerprint) {
			os.Exit(0)
		}
		log.Errorf("init config error: %v", err)
		return
	}

	edge, err := edge.NewEdge()
	if err != nil {
		log.Errorf("new edge err: %s", err)
		return
	}

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	edge.Close()
}
