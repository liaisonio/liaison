package frontierbound

import (
	"context"
	"errors"
	"math/rand"
	"net"

	"github.com/singchia/frontier/api/dataplane/v1/service"
	"github.com/singchia/geminio"
	"github.com/singchia/liaison/pkg/liaison/config"
	"github.com/singchia/liaison/pkg/utils"
)

type FrontierBound interface {
	OpenStream(ctx context.Context, edgeID uint64) (geminio.Stream, error)
	Close() error
}

// 这是edge向frontier注册的连接
type frontierBound struct {
	svc service.Service
}

func NewFrontierBound(conf *config.Configuration) (*frontierBound, error) {
	dial := conf.Frontier.Dial
	if len(dial.Addrs) == 0 {
		return nil, errors.New("dial addr is empty")
	}
	dialer := func() (net.Conn, error) {
		return utils.Dial(&dial, rand.Intn(len(dial.Addrs)))
	}

	svc, err := service.NewService(dialer, service.OptionServiceBufferSize(512, 512))
	if err != nil {
		return nil, err
	}
	return &frontierBound{
		svc: svc,
	}, nil
}

func (fb *frontierBound) Close() error {
	return fb.svc.Close()
}
