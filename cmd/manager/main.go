package main

import (
	"context"

	"github.com/jumboframes/armorigo/log"
	"github.com/jumboframes/armorigo/sigaction"
	"github.com/liaisonio/liaison/pkg/lerrors"
	"github.com/liaisonio/liaison/pkg/liaison"
)

func main() {
	liaison, err := liaison.NewLiaison()
	if err != nil {
		if err != lerrors.ErrInvalidUsage {
			log.Errorf("new liaison err: %s", err)
		}
		return
	}

	go liaison.Serve()

	sig := sigaction.NewSignal()
	sig.Wait(context.TODO())

	liaison.Close()
}
