package http

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"strings"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/liaison/pkg/entry/frontierbound"
	"github.com/singchia/liaison/pkg/proto"
)

// Server HTTP/HTTPS 反向代理服务器
type Server struct {
	mu             sync.RWMutex
	proxies        map[int]*httpProxy // id -> proxy
	proxiesIdxPort map[int]int        // port -> id
	frontierBound  frontierbound.FrontierBound
}

type httpProxy struct {
	id       int
	port     int
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewServer 创建 HTTP 服务器
func NewServer(frontierBound frontierbound.FrontierBound) *Server {
	return &Server{
		proxies:        make(map[int]*httpProxy),
		proxiesIdxPort: make(map[int]int),
		frontierBound:  frontierBound,
	}
}

// CreateProxy 创建 HTTP/HTTPS 代理
func (s *Server) CreateProxy(ctx context.Context, protoproxy *proto.Proxy, certFile, keyFile string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在该代理
	_, exists := s.proxies[protoproxy.ID]
	if exists {
		log.Warnf("proxy %d is already exists", protoproxy.ID)
		return nil
	}

	// 检查端口冲突
	requestedPort := protoproxy.ProxyPort
	if requestedPort != 0 {
		id, exists := s.proxiesIdxPort[requestedPort]
		if exists && id != protoproxy.ID {
			return fmt.Errorf("port %d conflict with proxy %d", requestedPort, id)
		}
	}

	// 创建监听器
	var listener net.Listener
	var err error
	if certFile != "" && keyFile != "" {
		// HTTPS
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("failed to load certificate: %w", err)
		}
		config := &tls.Config{
			Certificates: []tls.Certificate{cert},
		}
		listener, err = tls.Listen("tcp", fmt.Sprintf(":%d", requestedPort), config)
	} else {
		// HTTP
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", requestedPort))
	}
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", requestedPort, err)
	}

	// 获取实际端口
	actualPort := listener.Addr().(*net.TCPAddr).Port
	if requestedPort == 0 {
		log.Infof("proxy %d: system allocated port %d", protoproxy.ID, actualPort)
		protoproxy.ProxyPort = actualPort
	}

	// 创建可取消的 context
	proxyCtx, cancel := context.WithCancel(context.Background())

	proxy := &httpProxy{
		id:       protoproxy.ID,
		port:     actualPort,
		listener: listener,
		ctx:      proxyCtx,
		cancel:   cancel,
	}

	// 启动处理 goroutine
	proxy.wg.Add(1)
	go proxy.serve(s, protoproxy)

	s.proxies[protoproxy.ID] = proxy
	s.proxiesIdxPort[actualPort] = protoproxy.ID

	log.Infof("HTTP proxy %d listening on port %d", protoproxy.ID, actualPort)
	return nil
}

// DeleteProxy 删除代理
func (s *Server) DeleteProxy(ctx context.Context, id int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	proxy, exists := s.proxies[id]
	if !exists {
		log.Warnf("proxy %d not found", id)
		return nil
	}

	// 取消 context
	proxy.cancel()

	// 关闭监听器
	if err := proxy.listener.Close(); err != nil {
		log.Errorf("failed to close listener for proxy %d: %s", id, err)
	}

	// 等待 goroutine 退出
	done := make(chan struct{})
	go func() {
		proxy.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// goroutine 已退出
	case <-time.After(5 * time.Second):
		log.Warnf("proxy %d goroutine did not exit within 5 seconds", id)
	}

	delete(s.proxies, id)
	delete(s.proxiesIdxPort, proxy.port)

	log.Infof("HTTP proxy %d deleted", id)
	return nil
}

// Close 关闭所有代理
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, proxy := range s.proxies {
		proxy.cancel()
		proxy.listener.Close()
	}
	for _, proxy := range s.proxies {
		proxy.wg.Wait()
	}
	s.proxies = make(map[int]*httpProxy)
	s.proxiesIdxPort = make(map[int]int)
}

// serve 处理连接
func (p *httpProxy) serve(s *Server, protoproxy *proto.Proxy) {
	defer p.wg.Done()

	for {
		select {
		case <-p.ctx.Done():
			return
		default:
		}

		conn, err := p.listener.Accept()
		if err != nil {
			select {
			case <-p.ctx.Done():
				return
			default:
				log.Errorf("failed to accept connection: %s", err)
				continue
			}
		}

		// 为每个连接启动 goroutine
		p.wg.Add(1)
		go func(clientConn net.Conn) {
			defer p.wg.Done()
			defer clientConn.Close()
			s.handleConnection(p.ctx, clientConn, protoproxy)
		}(conn)
	}
}

// handleConnection 处理单个连接（支持 HTTP keep-alive 和 WebSocket）
func (s *Server) handleConnection(ctx context.Context, clientConn net.Conn, protoproxy *proto.Proxy) {
	defer clientConn.Close()

	reader := bufio.NewReader(clientConn)

	// 处理 keep-alive 连接，循环读取多个请求
	for {
		// 设置读取超时
		clientConn.SetReadDeadline(time.Now().Add(30 * time.Second))

		// 读取 HTTP 请求
		req, err := http.ReadRequest(reader)
		if err != nil {
			if err == io.EOF {
				// 连接关闭
				return
			}
			log.Errorf("failed to read request: %s", err)
			return
		}

		// 检查是否是 WebSocket 升级请求
		if s.isWebSocketUpgrade(req) {
			// WebSocket 处理（会接管整个连接，不会返回）
			// 移除读取超时限制，WebSocket 需要保持长时间连接
			clientConn.SetReadDeadline(time.Time{})
			s.handleWebSocket(ctx, clientConn, reader, req, protoproxy)
			return
		}

		// 处理普通 HTTP 请求
		keepAlive := s.handleRequest(ctx, clientConn, reader, req, protoproxy)

		// 如果不是 keep-alive，关闭连接
		if !keepAlive {
			return
		}
	}
}

// handleRequest 处理单个 HTTP 请求
func (s *Server) handleRequest(ctx context.Context, clientConn net.Conn, reader *bufio.Reader, req *http.Request, protoproxy *proto.Proxy) bool {
	defer req.Body.Close()

	// 检查是否是 keep-alive 连接
	keepAlive := req.ProtoAtLeast(1, 1) && req.Header.Get("Connection") != "close"

	// 打开到 edge 的 stream
	stream, err := s.frontierBound.OpenStream(ctx, protoproxy.EdgeID)
	if err != nil {
		log.Errorf("failed to open stream: %s", err)
		// 写入错误响应
		resp := &http.Response{
			StatusCode: http.StatusInternalServerError,
			Status:     "Internal Server Error",
			Proto:      "HTTP/1.1",
			ProtoMajor: 1,
			ProtoMinor: 1,
			Header:     make(http.Header),
			Body:       http.NoBody,
		}
		if !keepAlive {
			resp.Header.Set("Connection", "close")
		}
		_ = resp.Write(clientConn)
		return false
	}
	defer stream.Close()

	// 写入目标地址信息（类似 gatekeeper 的 preWrite）
	if err := s.writeDstInfo(stream, protoproxy); err != nil {
		log.Errorf("failed to write dst info: %s", err)
		return false
	}

	// 构建并发送 HTTP 请求
	if err := s.sendRequest(ctx, stream, req, protoproxy); err != nil {
		log.Errorf("failed to send request: %s", err)
		return false
	}

	// 读取响应
	resp, err := s.readResponse(ctx, stream, req)
	if err != nil {
		log.Errorf("failed to read response: %s", err)
		return false
	}
	defer resp.Body.Close()

	// 设置 keep-alive 响应头
	if keepAlive {
		resp.Header.Set("Connection", "keep-alive")
	} else {
		resp.Header.Set("Connection", "close")
	}

	// 写入响应到客户端
	if err := resp.Write(clientConn); err != nil {
		log.Errorf("failed to write response: %s", err)
		return false
	}

	return keepAlive
}

// writeDstInfo 写入目标地址信息
func (s *Server) writeDstInfo(stream io.Writer, protoproxy *proto.Proxy) error {
	dst := proto.Dst{
		Addr:          protoproxy.Dst,
		ApplicationID: protoproxy.ApplicationID,
		ProxyID:       uint(protoproxy.ID),
	}
	data, err := json.Marshal(dst)
	if err != nil {
		return fmt.Errorf("failed to marshal dst: %w", err)
	}

	// 写入长度（4字节）
	lengthBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lengthBuf, uint32(len(data)))
	if _, err := stream.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write dst length: %w", err)
	}

	// 写入数据
	if _, err := stream.Write(data); err != nil {
		return fmt.Errorf("failed to write dst: %w", err)
	}

	return nil
}

// sendRequest 发送 HTTP 请求到 stream
func (s *Server) sendRequest(ctx context.Context, stream io.Writer, req *http.Request, protoproxy *proto.Proxy) error {
	// 创建请求副本，避免修改原始请求
	reqCopy := req.Clone(ctx)
	if reqCopy == nil {
		reqCopy = &http.Request{
			Method:     req.Method,
			URL:        req.URL,
			Proto:      req.Proto,
			ProtoMajor: req.ProtoMajor,
			ProtoMinor: req.ProtoMinor,
			Header:     make(http.Header),
			Body:       req.Body,
			Host:       req.Host,
		}
		// 复制 header
		for k, v := range req.Header {
			reqCopy.Header[k] = v
		}
	}

	// 修改请求的 Host 和目标地址
	reqCopy.Host = protoproxy.Dst
	if reqCopy.URL != nil {
		// 解析目标地址
		dstURL := fmt.Sprintf("http://%s%s", protoproxy.Dst, req.URL.RequestURI())
		parsedURL, err := req.URL.Parse(dstURL)
		if err == nil {
			reqCopy.URL = parsedURL
		} else {
			// 如果解析失败，直接设置 Host
			reqCopy.URL.Host = protoproxy.Dst
		}
	}

	// 构建请求（类似参考代码的 buildReqReader）
	reqData, err := httputil.DumpRequest(reqCopy, true)
	if err != nil {
		return fmt.Errorf("failed to dump request: %w", err)
	}

	// 写入请求到 stream
	_, err = stream.Write(reqData)
	if err != nil {
		return fmt.Errorf("failed to write request: %w", err)
	}

	return nil
}

// readResponse 从 stream 读取 HTTP 响应
func (s *Server) readResponse(ctx context.Context, stream io.Reader, req *http.Request) (*http.Response, error) {
	// 创建带超时的 reader
	reader := bufio.NewReader(stream)

	// 读取响应（类似参考代码的 http.ReadResponse）
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return resp, nil
}

// isWebSocketUpgrade 检查是否是 WebSocket 升级请求
func (s *Server) isWebSocketUpgrade(req *http.Request) bool {
	connection := strings.ToLower(req.Header.Get("Connection"))
	upgrade := strings.ToLower(req.Header.Get("Upgrade"))

	// 检查是否包含 upgrade 和 websocket
	return strings.Contains(connection, "upgrade") && strings.Contains(upgrade, "websocket")
}

// handleWebSocket 处理 WebSocket 连接
func (s *Server) handleWebSocket(ctx context.Context, clientConn net.Conn, reader *bufio.Reader, req *http.Request, protoproxy *proto.Proxy) {
	log.Infof("handling WebSocket connection for proxy %d", protoproxy.ID)

	// 打开到 edge 的 stream
	stream, err := s.frontierBound.OpenStream(ctx, protoproxy.EdgeID)
	if err != nil {
		log.Errorf("failed to open stream for WebSocket: %s", err)
		return
	}
	defer stream.Close()

	// 写入目标地址信息
	if err := s.writeDstInfo(stream, protoproxy); err != nil {
		log.Errorf("failed to write dst info for WebSocket: %s", err)
		return
	}

	// 构建并发送 HTTP 请求（包含 WebSocket 升级头）
	if err := s.sendRequest(ctx, stream, req, protoproxy); err != nil {
		log.Errorf("failed to send WebSocket request: %s", err)
		return
	}

	// 读取 WebSocket 升级响应
	resp, err := s.readResponse(ctx, stream, req)
	if err != nil {
		log.Errorf("failed to read WebSocket upgrade response: %s", err)
		return
	}

	// 确保响应体被完全读取（WebSocket 升级响应通常没有响应体，但为了安全起见）
	if resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}

	// 将升级响应写入客户端
	if err := resp.Write(clientConn); err != nil {
		log.Errorf("failed to write WebSocket upgrade response: %s", err)
		return
	}

	// 移除所有超时限制，WebSocket 需要保持长时间连接
	clientConn.SetReadDeadline(time.Time{})
	clientConn.SetWriteDeadline(time.Time{})

	// 双向复制数据：客户端 <-> stream
	errChan := make(chan error, 2)

	// 从客户端读取，写入 stream
	go func() {
		_, err := io.Copy(stream, clientConn)
		if err != nil && !isErrClosed(err) {
			log.Errorf("WebSocket copy client->stream error: %s", err)
		}
		errChan <- err
	}()

	// 从 stream 读取，写入客户端
	go func() {
		_, err := io.Copy(clientConn, stream)
		if err != nil && !isErrClosed(err) {
			log.Errorf("WebSocket copy stream->client error: %s", err)
		}
		errChan <- err
	}()

	// 等待任一方向出错或关闭
	<-errChan
	log.Debugf("WebSocket connection closed for proxy %d", protoproxy.ID)
}

// isErrClosed 检查是否是连接关闭错误
func isErrClosed(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "closed") ||
		strings.Contains(errStr, "EOF") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "connection reset")
}
