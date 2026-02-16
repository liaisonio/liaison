/* eslint-disable */

declare namespace API {
  // ========== 通用响应 ==========
  interface Response<T = any> {
    code: number;
    message: string;
    data?: T;
  }

  // ========== 分页参数 ==========
  interface PageParams {
    page?: number;
    page_size?: number;
  }

  // ========== 认证相关 ==========
  interface LoginParams {
    email: string;
    password: string;
  }

  interface LoginResult {
    code: number;
    message: string;
    data?: {
      token: string;
      user?: CurrentUser;
    };
  }

  interface CurrentUser {
    id?: number;
    name?: string;
    email?: string;
    avatar?: string;
    role?: string;
    created_at?: string;
    last_login_at?: string;
    last_login_ip?: string;
  }

  // ========== 应用 (Application) ==========
  interface Application {
    id: number;
    name: string;
    application_type: string;
    ip: string;
    port: number;
    edge_id: number;
    device?: Device;
    proxy?: Proxy; // 已关联访问
    created_at: string;
    updated_at: string;
  }

  interface ApplicationListParams extends PageParams {
    device_id?: number;
    name?: string;
    application_type?: string;
    device_name?: string; // 设备名搜索
  }

  interface ApplicationListResult {
    applications: Application[];
    total: number;
  }

  interface ApplicationCreateParams {
    name: string;
    application_type: string;
    ip: string;
    port: number;
    edge_id: number;
    device_id?: number;
  }

  interface ApplicationUpdateParams {
    name?: string;
  }

  // ========== 设备 (Device) ==========
  interface Device {
    id: number;
    name: string;
    description?: string;
    os: string;
    version: string;
    cpu: number;
    memory: number;
    disk: number;
    online?: number; // 0: offline, 1: online
    interfaces?: Array<{
      name: string;
      mac: string;
      ip: string[];
      ipv4?: string[];
      ipv6?: string[];
    }>;
    created_at: string;
    updated_at: string;
  }

  interface DeviceListResult {
    devices: Device[];
    total: number;
  }

  interface DeviceListParams extends PageParams {
    name?: string;
    ip?: string; // 网卡IP搜索
  }

  interface DeviceUpdateParams {
    name?: string;
    description?: string;
  }

  // ========== 连接器/边缘节点 (Edge) ==========
  interface Edge {
    id: number;
    name: string;
    description?: string;
    status: number; // 1: running, 2: stopped
    online: number; // 0: offline, 1: online
    device?: Device; // 所在设备
    created_at: string;
    updated_at: string;
  }

  interface EdgeListResult {
    edges: Edge[];
    total: number;
  }

  interface EdgeListParams extends PageParams {
    name?: string;
    device_name?: string; // 设备名搜索
  }

  interface EdgeCreateParams {
    name: string;
    description?: string;
  }

  interface EdgeCreateResult {
    access_key: string;
    secret_key: string;
    command?: string; // 安装命令由后端返回
  }

  interface EdgeUpdateParams {
    name?: string;
    description?: string;
    status?: number; // 1: running, 2: stopped
  }

  // 扫描应用任务
  interface EdgeScanApplicationTask {
    id: number;
    edge_id: number;
    task_status: string;
    applications: string[];
    error?: string;
    created_at: string;
    updated_at: string;
  }

  interface EdgeScanTaskCreateParams {
    edge_id: number;
    port?: number;
    protocol?: string;
  }

  // ========== 访问 (Proxy) ==========
  interface Proxy {
    id: number;
    name: string;
    description?: string;
    port: number;
    status: string;
    application?: Application;
    created_at: string;
    updated_at: string;
    access_url?: string;
  }

  interface ProxyListResult {
    proxies: Proxy[];
    total: number;
  }

  interface ProxyListParams extends PageParams {
    name?: string;
  }

  interface ProxyCreateParams {
    name: string;
    description?: string;
    port?: number;
    application_id: number;
  }

  interface ProxyUpdateParams {
    name?: string;
    description?: string;
    port?: number;
    status?: string;
  }

  // ========== 流量监控 (Traffic Metric) ==========
  interface TrafficMetric {
    id: number;
    application_id: number;
    proxy_id: number;
    timestamp: string; // ISO 8601 格式
    bytes_in: number; // 入站流量（字节）
    bytes_out: number; // 出站流量（字节）
  }

  interface TrafficMetricsListParams {
    application_ids?: number[];
    proxy_ids?: number[];
    start_time?: string; // ISO 8601 格式
    end_time?: string; // ISO 8601 格式
    limit?: number; // 默认 100
  }

  interface TrafficMetricsListResult {
    metrics: TrafficMetric[];
  }
}
