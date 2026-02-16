package frontierbound

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"net"
	"sync"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/geminio"
	"github.com/singchia/geminio/client"
	"github.com/singchia/liaison/pkg/edge/config"
	"github.com/singchia/liaison/pkg/proto"
	"github.com/singchia/liaison/pkg/utils"
)

// FrontierBound 处于依赖链的最低端，负责与frontier通信
type FrontierBound interface {
	NewRequest(data []byte) geminio.Request
	// RPC
	RegisterRPCHandler(name string, f func(ctx context.Context, req geminio.Request, rsp geminio.Response)) error
	Call(ctx context.Context, name string, req geminio.Request) (geminio.Response, error)
	// Stream
	RegisterStreamHandler(handler func(ctx context.Context, stream geminio.Stream))
	// Close
	Close() error
	EdgeID() (uint64, error)
}

type frontierBound struct {
	end geminio.End

	// stream handler
	mu            sync.RWMutex
	streamHandler func(ctx context.Context, stream geminio.Stream)
}

func NewFrontierBound(conf *config.Configuration) (FrontierBound, error) {
	dial := conf.Manager.Dial
	if len(dial.Addrs) == 0 {
		return nil, errors.New("dial addr is empty")
	}

	meta := proto.Meta{
		AccessKey: conf.Manager.Auth.AccessKey,
		SecretKey: conf.Manager.Auth.SecretKey,
	}
	data, err := json.Marshal(meta)
	if err != nil {
		return nil, err
	}
	opt := client.NewEndOptions()
	opt.SetMeta(data)
	opt.SetBufferSize(512, 512)

	dialer := func() (net.Conn, error) {
		conn, err := utils.Dial(&dial, rand.Intn(len(dial.Addrs)))
		if err != nil {
			log.Errorf("frontlas new informer, dial err: %s", err)
			return nil, err
		}
		return conn, nil
	}
	end, err := client.NewRetryEndWithDialer(dialer, opt)
	if err != nil {
		log.Errorf("frontlas new retry end err: %s", err)
		return nil, err
	}

	fb := &frontierBound{
		end: end,
	}
	go fb.loopAccept(context.Background())

	return fb, nil
}

func (fb *frontierBound) NewRequest(data []byte) geminio.Request {
	return fb.end.NewRequest(data)
}

// register function to frontier
func (fb *frontierBound) RegisterRPCHandler(name string, f func(ctx context.Context, req geminio.Request, rsp geminio.Response)) error {

	err := fb.end.Register(context.Background(), name, f)
	if err != nil {
		log.Errorf("frontierbound register func err: %s, name: %s", err, name)
		return err
	}
	return nil
}

// call function to frontier
func (fb *frontierBound) Call(ctx context.Context, name string, req geminio.Request) (geminio.Response, error) {
	return fb.end.Call(ctx, name, req)
}

// stream
func (fb *frontierBound) RegisterStreamHandler(handler func(ctx context.Context, stream geminio.Stream)) {
	fb.mu.Lock()
	defer fb.mu.Unlock()
	fb.streamHandler = handler
}

func (fb *frontierBound) loopAccept(ctx context.Context) {
	for {
		stream, err := fb.end.AcceptStream()
		if err != nil {
			if err == io.EOF {
				log.Infof("frontierbound accept stream EOF")
				return
			}
			log.Errorf("frontierbound accept stream err: %s", err)
			continue
		}

		fb.mu.RLock()
		handler := fb.streamHandler
		fb.mu.RUnlock()
		if handler == nil {
			log.Errorf("frontierbound accept stream, handler is nil")
			continue
		}
		go handler(ctx, stream)
	}
}

func (fb *frontierBound) EdgeID() (uint64, error) {
	return fb.end.ClientID(), nil
}

func (fb *frontierBound) Close() error {
	return fb.end.Close()
}
