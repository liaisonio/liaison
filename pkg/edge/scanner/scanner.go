package scanner

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/singchia/geminio"
	"github.com/singchia/liaison/pkg/edge/frontierbound"
	"github.com/singchia/liaison/pkg/proto"
)

type Scanner interface{}

type scanner struct {
	frontierBound frontierbound.FrontierBound

	// tasks
	mu    sync.Mutex
	tasks map[uint]*proto.ScanApplicationTaskRequest
}

func NewScanner(frontierBound frontierbound.FrontierBound) (Scanner, error) {

	s := &scanner{
		frontierBound: frontierBound,
		tasks:         make(map[uint]*proto.ScanApplicationTaskRequest),
	}

	// 注册函数
	err := frontierBound.RegisterRPCHandler("scan_application", s.scanApplication)
	if err != nil {
		log.Errorf("scanner register func err: %s", err)
		return nil, err
	}
	// 拉取扫描任务
	go s.pullTaskScanApplication()

	return s, nil
}

func (s *scanner) pullTaskScanApplication() {
	edgeID, _ := s.frontierBound.EdgeID()
	request := proto.PullTaskScanApplicationRequest{
		EdgeID: edgeID,
	}
	data, err := json.Marshal(request)
	if err != nil {
		log.Errorf("marshal pull task scan application request error: %s", err)
		return
	}
	req := s.frontierBound.NewRequest(data)
	rsp, err := s.frontierBound.Call(context.Background(), "pull_task_scan_application", req)
	if err != nil {
		log.Errorf("pull task scan application call error: %s", err)
		return
	}
	if rsp.Error() != nil {
		log.Errorf("pull task scan application return error: %s", rsp.Error())
		return
	}
	var response proto.PullTaskScanApplicationResponse
	err = json.Unmarshal(rsp.Data(), &response)
	if err != nil {
		log.Errorf("unmarshal pull task scan application response error: %s", err)
		return
	}
	log.Infof("pull task scan application response: %v", response)

	// 扫描
	err = s.scan(context.Background(), &proto.ScanApplicationTaskRequest{
		TaskID:   response.TaskID,
		Nets:     response.Nets,
		Port:     response.Port,
		Protocol: response.Protocol,
	})
	if err != nil {
		rsp.SetError(err)
		return
	}
}

func (s *scanner) scanApplication(ctx context.Context, req geminio.Request, rsp geminio.Response) {
	log.Infof("scanner scan application: %s", req.Data())
	var task proto.ScanApplicationTaskRequest
	if err := json.Unmarshal(req.Data(), &task); err != nil {
		rsp.SetError(err)
		log.Errorf("unmarshal scan application task request error: %s", err)
		return
	}

	// 扫描
	err := s.scan(ctx, &task)
	if err != nil {
		rsp.SetError(err)
		log.Errorf("scan application error: %s", err)
		return
	}
}

func (s *scanner) scan(ctx context.Context, task *proto.ScanApplicationTaskRequest) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.tasks[task.TaskID]
	if ok {
		log.Errorf("task already exists")
		return errors.New("task already exists")
	}
	s.tasks[task.TaskID] = task

	go func() {
		defer func() {
			s.mu.Lock()
			delete(s.tasks, task.TaskID)
			s.mu.Unlock()
		}()

		// 创建一个独立的 context，避免使用可能被取消的原始 context
		// 扫描任务应该在后台独立运行，不受原始 RPC 请求的生命周期影响
		scanCtx, cancel := context.WithCancel(context.Background())
		defer cancel()

		running := proto.ScanApplicationTaskResult{
			TaskID:              task.TaskID,
			ScannedApplications: make([]proto.ScannedApplication, 0),
			Status:              "running",
		}
		// 上报扫描中
		data, err := json.Marshal(running)
		if err != nil {
			log.Errorf("marshal report task scan application result error: %s", err)
			return
		}
		reportReq := s.frontierBound.NewRequest(data)
		// 使用独立的 context，避免原始 RPC context 被取消导致上报失败
		_, err = s.frontierBound.Call(scanCtx, "report_task_scan_application", reportReq)
		if err != nil {
			log.Errorf("call report task scan application error: %s", err)
			return
		}
		log.Infof("scan with go start: %v", task)

		// 使用纯 Go 实现的扫描（不依赖 libpcap）
		// 使用独立的 context，但保留对原始 context 的监听，以便在需要时取消扫描
		err = s.scanWithGo(scanCtx, task, &running)
		if err != nil {
			log.Errorf("scan with go error: %s", err)
			return
		}

		completed := proto.ScanApplicationTaskResult{
			TaskID:              task.TaskID,
			ScannedApplications: running.ScannedApplications,
			Status:              "completed",
		}
		data, err = json.Marshal(completed)
		if err != nil {
			log.Errorf("marshal report task scan application result error: %s", err)
			return
		}
		reportReq = s.frontierBound.NewRequest(data)
		_, err = s.frontierBound.Call(scanCtx, "report_task_scan_application", reportReq)
		if err != nil {
			log.Errorf("call report task scan application error: %s", err)
			return
		}
		log.Infof("scan with go completed: %v", string(data))
	}()
	return nil
}

func (s *scanner) Close() error {
	return nil
}

// scanTask 扫描任务
type scanTask struct {
	IP       string
	Port     int
	Protocol string
}

// scanWithGo 使用纯 Go 实现的端口扫描（不依赖 libpcap）
func (s *scanner) scanWithGo(ctx context.Context, task *proto.ScanApplicationTaskRequest, result *proto.ScanApplicationTaskResult) error {
	// 获取要扫描的端口列表
	ports, err := s.getPorts(task.Port)
	if err != nil {
		return fmt.Errorf("get ports error: %s", err)
	}

	// 获取要扫描的 IP 列表
	ips, err := s.getIPsFromNets(task.Nets)
	if err != nil {
		return fmt.Errorf("get IPs from nets error: %s", err)
	}

	// 创建任务队列（带缓冲，避免阻塞）
	taskChan := make(chan scanTask, 1000)
	protocol := strings.ToLower(task.Protocol)

	// 任务分发：独立 goroutine 负责将任务放入队列
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer close(taskChan) // 分发完成后关闭 channel

		for _, ip := range ips {
			for _, port := range ports {
				select {
				case <-ctx.Done():
					return
				case taskChan <- scanTask{
					IP:       ip,
					Port:     port,
					Protocol: protocol,
				}:
				}
			}
		}
	}()

	// 使用 worker pool 模式，限制 goroutine 数量
	var mu sync.Mutex
	timeout := 2 * time.Second
	maxWorkers := 100 // 限制最多 100 个 worker goroutine

	// 启动 worker
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case scanTask, ok := <-taskChan:
					if !ok {
						// 任务队列已关闭
						return
					}

					var open bool
					switch scanTask.Protocol {
					case "udp":
						open = scanUDP(scanTask.IP, scanTask.Port, timeout)
					default: // tcp 或其他
						open = scanTCP(scanTask.IP, scanTask.Port, timeout)
					}

					if open {
						// 忽略 IPv6 地址
						ip := net.ParseIP(scanTask.IP)
						if ip != nil && ip.To4() == nil {
							// IPv6 地址，跳过
							continue
						}

						// 检测是否是 HTTP 应用
						protocol := scanTask.Protocol
						if scanTask.Protocol == "tcp" {
							// 尝试检测是否是 HTTP
							if isHTTP(scanTask.IP, scanTask.Port, timeout) {
								protocol = "http"
							}
						}

						mu.Lock()
						result.ScannedApplications = append(result.ScannedApplications, proto.ScannedApplication{
							IP:       scanTask.IP,
							Port:     scanTask.Port,
							Protocol: protocol,
						})
						mu.Unlock()
					}
				}
			}
		}()
	}

	wg.Wait()
	return nil
}

// getPorts 获取要扫描的端口列表
func (s *scanner) getPorts(port int) ([]int, error) {
	if port > 0 {
		return []int{port}, nil
	}
	// 返回常见端口列表（前100个）
	return getTopPorts(100), nil
}

// getIPsFromNets 从网段列表中获取所有 IP 地址
func (s *scanner) getIPsFromNets(nets []string) ([]string, error) {
	var ips []string
	for _, netStr := range nets {
		// 尝试解析为 CIDR
		ip, ipNet, err := net.ParseCIDR(netStr)
		if err != nil {
			// 如果不是 CIDR，尝试作为单个 IP
			if ip := net.ParseIP(netStr); ip != nil {
				ips = append(ips, netStr)
				continue
			}
			return nil, fmt.Errorf("invalid network format: %s", netStr)
		}

		// 遍历 CIDR 中的所有 IP
		for ip := ip.Mask(ipNet.Mask); ipNet.Contains(ip); inc(ip) {
			// 跳过网络地址和广播地址
			if isNetworkOrBroadcast(ip, ipNet) {
				continue
			}
			ips = append(ips, ip.String())
		}
	}
	return ips, nil
}

// inc 增加 IP 地址
func inc(ip net.IP) {
	for j := len(ip) - 1; j >= 0; j-- {
		ip[j]++
		if ip[j] > 0 {
			break
		}
	}
}

// isNetworkOrBroadcast 检查是否是网络地址或广播地址
func isNetworkOrBroadcast(ip net.IP, ipNet *net.IPNet) bool {
	// 简化处理：对于小网段（/24 及以上），跳过第一个和最后一个
	ones, bits := ipNet.Mask.Size()
	if ones >= 24 && bits == 32 {
		ip4 := ip.To4()
		if ip4 != nil {
			lastOctet := ip4[3]
			return lastOctet == 0 || lastOctet == 255
		}
	}
	return false
}

// getTopPorts 返回常见端口列表（基于 Nmap top 100 ports）
func getTopPorts(count int) []int {
	// Nmap top 100 ports: 7,9,13,21-23,25-26,37,53,79-81,88,106,110-111,113,119,135,139,143-144,179,199,389,427,443-445,465,513-515,543-544,548,554,587,631,646,873,990,993,995,1025-1029,1110,1433,1720,1723,1755,1900,2000-2001,2049,2121,2717,3000,3128,3306,3389,3986,4899,5000,5009,5051,5060,5101,5190,5357,5432,5631,5666,5800,5900,6000-6001,6646,7070,8000,8008-8009,8080-8081,8443,8888,9100,9999-10000,32768,49152-49157
	// 添加自定义端口: 8096 (Jellyfin), 18789
	commonPorts := []int{
		7, 9, 13, 21, 22, 23, 25, 26, 37, 53,
		79, 80, 81, 88, 106, 110, 111, 113, 119, 135,
		139, 143, 144, 179, 199, 389, 427, 443, 444, 445,
		465, 513, 514, 515, 543, 544, 548, 554, 587, 631,
		646, 873, 990, 993, 995, 1025, 1026, 1027, 1028, 1029,
		1110, 1433, 1720, 1723, 1755, 1900, 2000, 2001, 2049, 2121,
		2717, 3000, 3128, 3306, 3389, 3986, 4899, 5000, 5009, 5051,
		5060, 5101, 5190, 5357, 5432, 5631, 5666, 5800, 5900, 6000,
		6001, 6646, 7070, 8000, 8008, 8009, 8080, 8081, 8096, 8443,
		8888, 9100, 9999, 10000, 18789, 32768, 49152, 49153, 49154, 49155, 49156, 49157,
	}

	if count > len(commonPorts) {
		count = len(commonPorts)
	}
	return commonPorts[:count]
}

func scanTCP(ip string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	// 尝试读取一点数据（非必须）
	_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	reader := bufio.NewReader(conn)
	_, _ = reader.Peek(1)

	return true
}

func scanUDP(ip string, port int, timeout time.Duration) bool {
	conn, err := net.DialTimeout("udp", net.JoinHostPort(ip, strconv.Itoa(port)), timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	// 发个探测包
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	_, _ = conn.Write([]byte("\n"))

	// 看能不能收到回应
	buf := make([]byte, 1)
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	_, err = conn.Read(buf)
	if err != nil {
		return false
	}
	return true
}

// isHTTP 检测端口是否是 HTTP 服务
func isHTTP(ip string, port int, timeout time.Duration) bool {
	addr := net.JoinHostPort(ip, strconv.Itoa(port))
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	defer conn.Close()

	// 发送 HTTP 请求
	_ = conn.SetWriteDeadline(time.Now().Add(timeout))
	_, err = conn.Write([]byte("GET / HTTP/1.1\r\nHost: " + ip + "\r\n\r\n"))
	if err != nil {
		return false
	}

	// 读取响应
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}

	// 检查是否是 HTTP 响应（以 HTTP/ 开头）
	return len(line) >= 5 && line[:5] == "HTTP/"
}
