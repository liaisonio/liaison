/**
 * API 服务统一入口
 * 基于 Liaison 文档: http://49.232.250.11:8080
 */

import { request } from '@umijs/max';

/** 登录 POST /v1/iam/login */
export async function login(data: API.LoginParams) {
  return request<API.LoginResult>('/api/v1/iam/login', {
    method: 'POST',
    data,
  });
}

/** 获取当前用户信息 GET /v1/iam/profile */
export async function getCurrentUser() {
  return request<API.Response<API.CurrentUser>>('/api/v1/iam/profile', {
    method: 'GET',
  });
}

/** 修改密码 POST /v1/iam/password */
export async function changePassword(data: {
  old_password: string;
  new_password: string;
}) {
  return request<API.Response>('/api/v1/iam/password', {
    method: 'POST',
    data,
  });
}

/** 退出登录 POST /v1/iam/logout */
export async function logout() {
  return request<API.Response>('/api/v1/iam/logout', {
    method: 'POST',
    data: {},
  });
}

/** 获取应用列表 GET /v1/applications */
export async function getApplicationList(params?: API.ApplicationListParams) {
  return request<API.Response<API.ApplicationListResult>>(
    '/api/v1/applications',
    {
      method: 'GET',
      params,
    },
  );
}

/** 创建应用 POST /v1/applications */
export async function createApplication(data: API.ApplicationCreateParams) {
  return request<API.Response<API.Application>>('/api/v1/applications', {
    method: 'POST',
    data,
  });
}

/** 更新应用 PUT /v1/applications/:id */
export async function updateApplication(
  id: number,
  data: API.ApplicationUpdateParams,
) {
  return request<API.Response<API.Application>>(`/api/v1/applications/${id}`, {
    method: 'PUT',
    data,
  });
}

/** 删除应用 DELETE /v1/applications/:id */
export async function deleteApplication(id: number) {
  return request<API.Response>(`/api/v1/applications/${id}`, {
    method: 'DELETE',
  });
}

/** 获取设备列表 GET /v1/devices */
export async function getDeviceList(params?: API.DeviceListParams) {
  return request<API.Response<API.DeviceListResult>>('/api/v1/devices', {
    method: 'GET',
    params,
  });
}

/** 获取设备详情 GET /v1/devices/:id */
export async function getDeviceDetail(id: number) {
  return request<API.Response<API.Device>>(`/api/v1/devices/${id}`, {
    method: 'GET',
  });
}

/** 更新设备 PUT /v1/devices/:id */
export async function updateDevice(id: number, data: API.DeviceUpdateParams) {
  return request<API.Response<API.Device>>(`/api/v1/devices/${id}`, {
    method: 'PUT',
    data,
  });
}

/** 删除设备 DELETE /v1/devices/:id */
export async function deleteDevice(id: number) {
  return request<API.Response>(`/api/v1/devices/${id}`, {
    method: 'DELETE',
  });
}

/** 获取连接器列表 GET /v1/edges */
export async function getEdgeList(params?: API.EdgeListParams) {
  return request<API.Response<API.EdgeListResult>>('/api/v1/edges', {
    method: 'GET',
    params,
  });
}

/** 获取连接器详情 GET /v1/edges/:id */
export async function getEdgeDetail(id: number) {
  return request<API.Response<API.Edge>>(`/api/v1/edges/${id}`, {
    method: 'GET',
  });
}

/** 检测连接器在线状态 GET /v1/edges/:id */
export async function checkEdgeOnline(id: number) {
  return request<API.Response<API.Edge>>(`/api/v1/edges/${id}`, {
    method: 'GET',
  });
}

/** 创建连接器 POST /v1/edges */
export async function createEdge(data: API.EdgeCreateParams) {
  return request<API.Response<API.EdgeCreateResult>>('/api/v1/edges', {
    method: 'POST',
    data,
  });
}

/** 更新连接器 PUT /v1/edges/:id */
export async function updateEdge(id: number, data: API.EdgeUpdateParams) {
  return request<API.Response<API.Edge>>(`/api/v1/edges/${id}`, {
    method: 'PUT',
    data,
  });
}

/** 删除连接器 DELETE /v1/edges/:id */
export async function deleteEdge(id: number) {
  return request<API.Response>(`/api/v1/edges/${id}`, {
    method: 'DELETE',
  });
}

/** 获取扫描应用任务 GET /v1/edges/:edge_id/scan_application_tasks */
export async function getEdgeScanTask(edgeId: number) {
  return request<API.Response<API.EdgeScanApplicationTask>>(
    `/api/v1/edges/${edgeId}/scan_application_tasks`,
    {
      method: 'GET',
      params: { edge_id: edgeId },
    },
  );
}

/** 创建扫描应用任务 POST /v1/edges/:edge_id/scan_application_tasks */
export async function createEdgeScanTask(data: API.EdgeScanTaskCreateParams) {
  return request<API.Response>(
    `/api/v1/edges/${data.edge_id}/scan_application_tasks`,
    {
      method: 'POST',
      data,
    },
  );
}

/** 获取访问列表 GET /v1/proxies */
export async function getProxyList(params?: API.ProxyListParams) {
  return request<API.Response<API.ProxyListResult>>('/api/v1/proxies', {
    method: 'GET',
    params,
  });
}

/** 创建访问 POST /v1/proxies */
export async function createProxy(data: API.ProxyCreateParams) {
  return request<API.Response<API.Proxy>>('/api/v1/proxies', {
    method: 'POST',
    data,
  });
}

/** 更新访问 PUT /v1/proxies/:id */
export async function updateProxy(id: number, data: API.ProxyUpdateParams) {
  return request<API.Response<API.Proxy>>(`/api/v1/proxies/${id}`, {
    method: 'PUT',
    data,
  });
}

/** 删除访问 DELETE /v1/proxies/:id */
export async function deleteProxy(id: number) {
  return request<API.Response>(`/api/v1/proxies/${id}`, {
    method: 'DELETE',
  });
}

/** 获取 WebSSH 目标信息 GET /v1/webssh/proxies/:id */
export async function getWebSSHTarget(id: number) {
  return request<API.Response<API.WebSSHTarget>>(
    `/api/v1/webssh/proxies/${id}`,
    {
      method: 'GET',
    },
  );
}

/** 创建 WebSSH 会话 POST /v1/webssh/proxies/:id/session */
export async function createWebSSHSession(
  id: number,
  data: API.CreateWebSSHSessionRequest,
) {
  return request<API.Response<API.CreateWebSSHSessionResponse>>(
    `/api/v1/webssh/proxies/${id}/session`,
    {
      method: 'POST',
      data,
    },
  );
}

/** 重置 WebSSH host key DELETE /v1/webssh/proxies/:id/host-key */
export async function deleteWebSSHHostKey(id: number) {
  return request<API.Response>(`/api/v1/webssh/proxies/${id}/host-key`, {
    method: 'DELETE',
  });
}

/** 清除 WebSSH 保存凭据 DELETE /v1/webssh/proxies/:id/credential */
export async function deleteWebSSHCredential(id: number, username?: string) {
  return request<API.Response>(`/api/v1/webssh/proxies/${id}/credential`, {
    method: 'DELETE',
    params: username ? { username } : undefined,
  });
}

/** 获取 WebDesktop 目标信息 GET /v1/webdesktop/proxies/:id */
export async function getWebDesktopTarget(id: number) {
  return request<API.Response<API.WebDesktopTarget>>(
    `/api/v1/webdesktop/proxies/${id}`,
    {
      method: 'GET',
    },
  );
}

/** 创建 WebDesktop 会话 POST /v1/webdesktop/proxies/:id/session */
export async function createWebDesktopSession(
  id: number,
  data: API.CreateWebDesktopSessionRequest,
) {
  return request<API.Response<API.CreateWebDesktopSessionResponse>>(
    `/api/v1/webdesktop/proxies/${id}/session`,
    {
      method: 'POST',
      data,
    },
  );
}

/** 清除 WebDesktop 保存凭据 DELETE /v1/webdesktop/proxies/:id/credential */
export async function deleteWebDesktopCredential(
  id: number,
  params: API.DeleteWebDesktopCredentialParams,
) {
  return request<API.Response>(`/api/v1/webdesktop/proxies/${id}/credential`, {
    method: 'DELETE',
    params,
  });
}

/** 获取 WebData 目标信息 GET /v1/webdata/proxies/:id */
export async function getWebDataTarget(id: number) {
  return request<API.Response<API.WebDataTarget>>(
    `/api/v1/webdata/proxies/${id}`,
    {
      method: 'GET',
    },
  );
}

/** 创建 WebData 会话 POST /v1/webdata/proxies/:id/session */
export async function createWebDataSession(
  id: number,
  data: API.CreateWebDataSessionRequest,
) {
  return request<API.Response<API.CreateWebDataSessionResponse>>(
    `/api/v1/webdata/proxies/${id}/session`,
    {
      method: 'POST',
      data,
    },
  );
}

/** 测试 WebData 连接 POST /v1/webdata/proxies/:id/test */
export async function testWebDataConnection(
  id: number,
  data: API.CreateWebDataSessionRequest,
): Promise<API.Response> {
  try {
    return (await request<API.Response>(`/api/v1/webdata/proxies/${id}/test`, {
      method: 'POST',
      data,
      skipErrorHandler: true,
    } as any)) as unknown as API.Response;
  } catch (error: any) {
    return {
      code: error?.response?.status || 500,
      message: '连接测试失败',
    };
  }
}

/** 保存/更新 WebData 连接 POST /v1/webdata/proxies/:id/credential */
export async function saveWebDataCredential(
  id: number,
  data: API.SaveWebDataCredentialRequest,
) {
  return request<API.Response<API.WebDataCredential>>(
    `/api/v1/webdata/proxies/${id}/credential`,
    {
      method: data.id ? 'PUT' : 'POST',
      data,
    },
  );
}

/** 执行 WebData 命令 POST /v1/webdata/sessions/:token/execute */
export async function executeWebDataStatement(
  token: string,
  statement: string,
) {
  return request<API.Response<API.WebDataExecuteResult>>(
    `/api/v1/webdata/sessions/${token}/execute`,
    {
      method: 'POST',
      data: { statement },
    },
  );
}

/** 获取 WebData metadata GET /v1/webdata/sessions/:token/metadata */
export async function getWebDataMetadata(token: string) {
  return request<API.Response<API.WebDataMetadataResult>>(
    `/api/v1/webdata/sessions/${token}/metadata`,
    {
      method: 'GET',
    },
  );
}

/** 获取 WebData 对象详情 GET /v1/webdata/sessions/:token/object */
export async function getWebDataObject(
  token: string,
  params: API.WebDataObjectParams,
) {
  return request<API.Response<API.WebDataObjectResult>>(
    `/api/v1/webdata/sessions/${token}/object`,
    {
      method: 'GET',
      params,
    },
  );
}

/** 获取 WebData 审计列表 GET /v1/webdata/proxies/:id/audits */
export async function getWebDataAudits(
  id: number,
  params?: API.WebDataAuditListParams,
) {
  return request<API.Response<API.WebDataAuditListResult>>(
    `/api/v1/webdata/proxies/${id}/audits`,
    {
      method: 'GET',
      params,
    },
  );
}

/** 获取 WebData 全局审计列表 GET /v1/audits/webdata */
export async function getWebDataAuditList(params?: API.WebDataAuditListParams) {
  return request<API.Response<API.WebDataAuditListResult>>(
    '/api/v1/audits/webdata',
    {
      method: 'GET',
      params,
    },
  );
}

/** 获取访问审计列表 GET /v1/audits/access */
export async function getAccessAuditList(params?: API.WebDataAuditListParams) {
  return request<API.Response<API.WebDataAuditListResult>>(
    '/api/v1/audits/access',
    {
      method: 'GET',
      params,
    },
  );
}

/** 关闭 WebData 会话 DELETE /v1/webdata/sessions/:token */
export async function deleteWebDataSession(token: string) {
  return request<API.Response>(`/api/v1/webdata/sessions/${token}`, {
    method: 'DELETE',
  });
}

/** 清除 WebData 保存凭据 DELETE /v1/webdata/proxies/:id/credential */
export async function deleteWebDataCredential(
  id: number,
  params: API.DeleteWebDataCredentialParams,
) {
  return request<API.Response>(`/api/v1/webdata/proxies/${id}/credential`, {
    method: 'DELETE',
    params,
  });
}

/** 获取流量监控列表 GET /v1/traffic-metrics */
export async function getTrafficMetricsList(
  params?: API.TrafficMetricsListParams,
) {
  return request<API.Response<API.TrafficMetricsListResult>>(
    '/api/v1/traffic-metrics',
    {
      method: 'GET',
      params,
    },
  );
}

/** 获取调用方的出口 IP（公开接口）GET /v1/iam/client_ip */
export async function getClientIP() {
  return request<API.Response<{ ip: string }>>('/api/v1/iam/client_ip', {
    method: 'GET',
  });
}

/** 获取 PAT 列表 GET /v1/iam/tokens */
export async function listAPITokens() {
  return request<API.Response<API.APITokenListResult>>('/api/v1/iam/tokens', {
    method: 'GET',
  });
}

/** 创建 PAT POST /v1/iam/tokens — 响应中的 token 明文只会返回一次 */
export async function createAPIToken(data: API.APITokenCreateParams) {
  return request<API.Response<API.APITokenCreated>>('/api/v1/iam/tokens', {
    method: 'POST',
    data,
  });
}

/** 撤销 PAT DELETE /v1/iam/tokens/:id */
export async function revokeAPIToken(id: number) {
  return request<API.Response>(`/api/v1/iam/tokens/${id}`, {
    method: 'DELETE',
  });
}

/** 获取代理防火墙 GET /v1/proxies/:id/firewall */
export async function getProxyFirewall(proxyId: number) {
  return request<API.Response<API.ProxyFirewall>>(
    `/api/v1/proxies/${proxyId}/firewall`,
    {
      method: 'GET',
    },
  );
}

/** 设置代理防火墙 PUT /v1/proxies/:id/firewall —— allowed_cidrs 为 [] 表示全部拒绝 */
export async function upsertProxyFirewall(
  proxyId: number,
  data: API.ProxyFirewallUpsertParams,
) {
  return request<API.Response<API.ProxyFirewall>>(
    `/api/v1/proxies/${proxyId}/firewall`,
    {
      method: 'PUT',
      data,
    },
  );
}

/** 删除代理防火墙 DELETE /v1/proxies/:id/firewall —— 恢复为默认放行 */
export async function deleteProxyFirewall(proxyId: number) {
  return request<API.Response>(`/api/v1/proxies/${proxyId}/firewall`, {
    method: 'DELETE',
  });
}
