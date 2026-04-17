package proxy

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"io"
	"net"
	"strings"
	"sync"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/geminio"
	"github.com/liaisonio/liaison/pkg/edge/frontierbound"
	"github.com/liaisonio/liaison/pkg/proto"
)

type Proxy interface{}

type proxy struct {
	frontierBound frontierbound.FrontierBound
}

func NewProxy(frontierBound frontierbound.FrontierBound) (Proxy, error) {
	proxy := &proxy{
		frontierBound: frontierBound,
	}

	proxy.frontierBound.RegisterStreamHandler(proxy.proxy)

	return proxy, nil
}

func (p *proxy) proxy(ctx context.Context, stream geminio.Stream) {
	// 读取前4个字节获取meta长度
	lengthBuf := make([]byte, 4)
	_, err := io.ReadFull(stream, lengthBuf)
	if err != nil {
		log.Errorf("proxy stream read meta length err: %s", err)
		return
	}
	length := binary.BigEndian.Uint32(lengthBuf)
	dataBuf := make([]byte, length)
	_, err = io.ReadFull(stream, dataBuf)
	if err != nil {
		log.Errorf("proxy stream read meta data err: %s", err)
		return
	}

	var dst proto.Dst
	if err := json.Unmarshal(dataBuf, &dst); err != nil {
		log.Errorf("proxy stream meta unmarshal err: %s", err)
		return
	}

	conn, err := net.Dial("tcp", dst.Addr)
	if err != nil {
		log.Errorf("proxy stream dial err: %s", err)
		return
	}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// 从 stream 读取到 conn（从客户端到服务器）
		_, err := io.Copy(conn, stream)
		if err != nil && !IsErrClosed(err) {
			log.Errorf("read stream, src: %s, dst: %s; to conn, src: %s, dst: %s; err: %s",
				stream.RemoteAddr(), stream.LocalAddr(), conn.LocalAddr(), conn.RemoteAddr(), err)
		}
		_ = stream.Close()
		_ = conn.Close()
	}()

	go func() {
		defer wg.Done()
		// 从 conn 读取到 stream（从服务器到客户端）
		_, err := io.Copy(stream, conn)
		if err != nil && !IsErrClosed(err) {
			log.Errorf("read conn, src: %s, dst: %s; to stream, src: %s, dst: %s; err: %s",
				conn.LocalAddr(), conn.RemoteAddr(), stream.RemoteAddr(), stream.LocalAddr(), err)
		}
		_ = stream.Close()
		_ = conn.Close()
	}()

	wg.Wait()
}

func IsErrClosed(err error) bool {
	if strings.Contains(err.Error(), net.ErrClosed.Error()) {
		return true
	}
	return false
}
