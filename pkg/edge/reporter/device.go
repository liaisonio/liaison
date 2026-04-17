package reporter

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/jumboframes/armorigo/log"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/mem"
	"github.com/liaisonio/liaison/pkg/proto"
	"github.com/liaisonio/liaison/pkg/utils"
)

func (r *reporter) loopReportDevice(ctx context.Context) {
	for {
		device, err := getDevice()
		if err != nil {
			log.Errorf("get device error: %v", err)
			// 失败后等待 5 分钟再重试
			time.Sleep(5 * time.Minute)
			continue
		}
		device.EdgeID, _ = r.frontierBound.EdgeID()
		err = r.reportDevice(ctx, device)
		if err != nil {
			log.Errorf("report device error: %v", err)
			// 失败后等待 5 分钟再重试
			time.Sleep(5 * time.Minute)
			continue
		}
		// 成功后等待 1 小时
		time.Sleep(time.Hour)
	}
}

func (r *reporter) loopReportDeviceUsage(ctx context.Context) {
	for {
		deviceUsage, err := getDeviceUsage()
		if err != nil {
			log.Errorf("get device usage error: %v", err)
			// 失败后等待 1 分钟再重试
			time.Sleep(time.Minute)
			continue
		}
		err = r.reportDeviceUsage(ctx, deviceUsage)
		if err != nil {
			log.Errorf("report device usage error: %v", err)
			// 失败后等待 1 分钟再重试
			time.Sleep(time.Minute)
			continue
		}
		// 成功后等待 1 分钟
		time.Sleep(time.Minute)
	}
}

func (r *reporter) reportDevice(ctx context.Context, device *proto.Device) error {
	data, err := json.Marshal(device)
	if err != nil {
		return err
	}

	req := r.frontierBound.NewRequest(data)
	rsp, err := r.frontierBound.Call(ctx, "report_device", req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return rsp.Error()
	}
	log.Infof("report device success: %s", string(data))
	return nil
}

func (r *reporter) reportDeviceUsage(ctx context.Context, deviceUsage *proto.DeviceUsage) error {
	data, err := json.Marshal(deviceUsage)
	if err != nil {
		return err
	}
	req := r.frontierBound.NewRequest(data)
	rsp, err := r.frontierBound.Call(ctx, "report_device_usage", req)
	if err != nil {
		return err
	}
	if rsp.Error() != nil {
		return rsp.Error()
	}
	log.Infof("report device usage success: %s", string(data))
	return nil
}

// 注意：指纹才是唯一标识一台主机，而不是边缘和AKSK
// 如果当前edge扫描的指纹发生改变，则会重新建立edge和device的关联
func getDevice() (*proto.Device, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("failed to get hostname: %w", err)
	}

	// 基础信息
	cpuCount, err := cpu.Counts(true)
	if err != nil {
		// 如果无法获取 CPU 数量，使用默认值 1
		cpuCount = 1
		log.Warnf("failed to get CPU count, using default: 1, error: %v", err)
	}

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, fmt.Errorf("failed to get memory info: %w", err)
	}
	memMB := memory.Total / 1024 / 1024

	// OS
	info, err := host.Info()
	if err != nil {
		return nil, fmt.Errorf("failed to get host info: %w", err)
	}
	os := info.OS
	osVersion := info.KernelVersion

	// 根磁盘大小
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return nil, fmt.Errorf("failed to get disk usage: %w", err)
	}
	diskMB := diskUsage.Total / 1024 / 1024

	// 网络接口
	interfaces, err := getDeviceEthernetInterface()
	if err != nil {
		// 如果无法获取网络接口，使用空列表
		interfaces = []*proto.DeviceEthernetInterface{}
		log.Warnf("failed to get network interfaces, using empty list, error: %v", err)
	}
	// 指纹
	fingerprint, components, err := utils.GetFingerprint()
	if err != nil {
		return nil, fmt.Errorf("failed to get fingerprint: %w", err)
	}
	if components != nil {
		log.Debugf("getDevice fingerprint: %s, MACs: %v, CPU: %s, DiskIDs: %v, Raw: %s",
			fingerprint, components.MACs, components.CPUInfo, components.DiskIDs, components.Raw)
	}

	return &proto.Device{
		Fingerprint: fingerprint,
		HostName:    hostname,
		CPU:         cpuCount,
		Memory:      int(memMB),
		Disk:        int(diskMB),
		OS:          os,
		OSVersion:   osVersion,
		Interfaces:  interfaces,
	}, nil
}

func getDeviceEthernetInterface() ([]*proto.DeviceEthernetInterface, error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}

	ethernetInterfaces := make([]*proto.DeviceEthernetInterface, 0)

	for _, iface := range interfaces {
		// 检查是否是 loopback 接口（lo、lo0 等）
		isLoopback := iface.Flags&net.FlagLoopback != 0 || iface.Name == "lo" || iface.Name == "lo0"

		// 如果不是 loopback 接口，使用物理接口检查
		if !isLoopback {
			// 在 macOS、Windows 和 Linux 上，只使用物理接口
			if runtime.GOOS == "darwin" {
				if !utils.IsMacPhysicalInterface(&iface) {
					continue
				}
			} else if runtime.GOOS == "windows" {
				if !utils.IsWindowsPhysicalInterface(&iface) {
					continue
				}
			} else if runtime.GOOS == "linux" {
				if !utils.IsLinuxPhysicalInterface(&iface) {
					continue
				}
			} else {
				// 其他系统，过滤掉常见的虚拟接口
				name := strings.ToLower(iface.Name)
				if strings.Contains(name, "utun") || // VPN 接口
					strings.Contains(name, "bridge") || // 桥接接口
					strings.Contains(name, "vmnet") || // VMware 虚拟接口
					strings.Contains(name, "vboxnet") || // VirtualBox 虚拟接口
					strings.Contains(name, "awdl") || // Apple Wireless Direct Link (可能不稳定)
					strings.Contains(name, "anpi") { // Apple Network Packet Injection
					continue
				}
			}
		}
		// 跳过没有MAC或未启用的接口（loopback 接口可能没有 MAC，允许通过）
		if iface.Flags&net.FlagUp == 0 && !isLoopback {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			return nil, err
		}

		ipMasks := make([]*proto.IPMask, 0)
		hasIPv4 := false

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet: // 带掩码的情况
				ip := v.IP.String()
				// 如果是 loopback 接口，允许 loopback 地址；否则跳过 loopback 和 link-local 地址
				if !isLoopback {
					if v.IP.IsLoopback() || v.IP.IsLinkLocalUnicast() {
						continue
					}
				} else if v.IP.IsLinkLocalUnicast() {
					// loopback 接口上，只跳过 link-local，保留 loopback 地址
					continue
				}
				// 检查是否为 IPv4
				if v.IP.To4() != nil {
					hasIPv4 = true
				}
				mask := ""
				if v.Mask != nil {
					// 统一使用前缀长度（CIDR notation）
					ones, _ := v.Mask.Size()
					mask = fmt.Sprintf("%d", ones)
				}
				ipMasks = append(ipMasks, &proto.IPMask{
					IP:      ip,
					Netmask: mask,
				})

			case *net.IPAddr: // 只有IP没有掩码
				// 如果是 loopback 接口，允许 loopback 地址；否则跳过 loopback 和 link-local 地址
				if !isLoopback {
					if v.IP.IsLoopback() || v.IP.IsLinkLocalUnicast() {
						continue
					}
				} else if v.IP.IsLinkLocalUnicast() {
					// loopback 接口上，只跳过 link-local，保留 loopback 地址
					continue
				}
				// 检查是否为 IPv4
				if v.IP.To4() != nil {
					hasIPv4 = true
				}
				ipMasks = append(ipMasks, &proto.IPMask{
					IP:      v.IP.String(),
					Netmask: "",
				})
			}
		}

		// 添加有IP地址的网卡（loopback 接口允许没有 IPv4，只要有 IP 地址即可）
		if len(ipMasks) > 0 && (hasIPv4 || isLoopback) {
			mac := ""
			if len(iface.HardwareAddr) > 0 {
				mac = iface.HardwareAddr.String()
			}
			ethernetInterfaces = append(ethernetInterfaces, &proto.DeviceEthernetInterface{
				Name:    iface.Name,
				MAC:     mac,
				IPMasks: ipMasks,
			})
		}
	}
	return ethernetInterfaces, nil
}

func getDeviceUsage() (*proto.DeviceUsage, error) {
	// 获取指纹
	fingerprint, components, err := utils.GetFingerprint()
	if err != nil {
		return nil, fmt.Errorf("failed to get fingerprint: %w", err)
	}
	if components != nil {
		log.Debugf("getDeviceUsage fingerprint: %s, MACs: %v, CPU: %s, DiskIDs: %v, Raw: %s",
			fingerprint, components.MACs, components.CPUInfo, components.DiskIDs, components.Raw)
	}

	memory, err := mem.VirtualMemory()
	if err != nil {
		return nil, err
	}
	// 资源使用
	cpuUsage, err := cpu.Percent(0, false)
	if err != nil {
		return nil, err
	}
	diskUsage, err := disk.Usage("/")
	if err != nil {
		return nil, err
	}
	deviceUsage := &proto.DeviceUsage{
		Fingerprint: fingerprint,
		CPUUsage:    float32(cpuUsage[0]),
		MemoryUsage: float32(memory.UsedPercent),
		DiskUsage:   float32(diskUsage.UsedPercent),
	}
	return deviceUsage, nil
}

// GetFingerprint 获取设备指纹（导出函数，供命令行使用）
func GetFingerprint() (string, error) {
	fingerprint, components, err := utils.GetFingerprint()
	if err == nil && components != nil {
		log.Debugf("GetFingerprint fingerprint: %s, MACs: %v, CPU: %s, DiskIDs: %v, Raw: %s",
			fingerprint, components.MACs, components.CPUInfo, components.DiskIDs, components.Raw)
	}
	return fingerprint, err
}
