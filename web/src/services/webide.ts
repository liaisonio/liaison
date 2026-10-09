import { request } from '@/api/client';

export type IDEApplication = {
  id: string;
  name: string;
  edge_id: number;
  installation_id: string;
  mode: 'managed' | 'external';
  port: number;
};
export type IDEAccess = {
  id: string;
  name: string;
  application_id: string;
  enabled: boolean;
  project: string;
  application_name?: string;
  application_mode?: string;
  connector_name?: string;
  connector_online?: boolean;
  connector_available?: boolean;
};
export type IDEConnector = { id: number; name: string; online: boolean };
export type IDEInstance = {
  id: string;
  access_id: string;
  installation_id: string;
  project: string;
  status: string;
  started_at: string;
};
export type IDEInstallation = {
  id: string;
  path: string;
  source: string;
  version: string;
};
export type IDEResult = {
  version: number;
  status: string;
  can_launch: boolean;
  platform: string;
  installations?: IDEInstallation[];
  instances?: IDEInstance[];
  directories?: { name: string; path: string }[];
  directory?: string;
  truncated?: boolean;
};
export type IDECapabilities = { enabled: boolean; ready: boolean };
export function ideID() {
  return crypto.randomUUID().replace(/-/g, '');
}
export function ideTheme(): 'dark' | 'light' {
  return document.documentElement.classList.contains('dark') ? 'dark' : 'light';
}
export async function ideRequest<T>(
  path: string,
  method: 'GET' | 'POST' | 'PUT' | 'DELETE' = 'GET',
  data?: unknown,
  signal?: AbortSignal,
  onRequestID?: (id:string) => void,
): Promise<T> {
  const response = await request<API.Response<T>>(`/api/v1/webide/${path}`, {
    method,
    data,
    signal,
    skipErrorHandler: true,
    onResponse: response => { const id=response.headers.get('X-Request-ID');if(id&&/^[a-f0-9]{32}$/.test(id))onRequestID?.(id); },
  });
  if (response.data === undefined && method !== 'DELETE')
    throw Error('Missing IDE response');
  return response.data as T;
}

// Do not silently truncate relationships or filtered results at the first page.
export async function ideListAll<T>(resource: 'applications' | 'accesses', signal?: AbortSignal): Promise<T[]> {
  const items: T[] = [];
  for (let page = 1; page <= 100; page++) {
    const result = await ideRequest<{items: T[]; total: number}>(`${resource}?page=${page}&page_size=100`, 'GET', undefined, signal);
    items.push(...result.items);
    if (items.length >= result.total) return items;
    if (!result.items.length) throw Error('Incomplete IDE list');
  }
  throw Error('IDE list limit exceeded');
}
export async function ideControl(
  edge: number,
  action: string,
  directory?: string,
  signal?: AbortSignal,
) {
  return ideRequest<IDEResult>(
    `connectors/${edge}`,
    'POST',
    { action, ...(directory === undefined ? {} : { directory }) },
    signal,
  );
}
export async function ideRuntime(
  access: string,
  action: 'start' | 'stop',
  project?: string,
  instance_id?: string,
) {
  return ideRequest<IDEResult>(`accesses/${access}/runtime`, 'POST', {
    action,
    project,
    instance_id,
  });
}
