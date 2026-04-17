package main

import (
	"context"
	"errors"
	"os"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/sigaction"
	"github.com/liaisonio/liaison/pkg/edge"
	"github.com/liaisonio/liaison/pkg/edge/config"
)

func main() {
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
