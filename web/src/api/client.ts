import { getLocale } from '@/i18n';
import { history } from '@/lib/runtime';
import { getToken, useSession } from '@/store/session';

type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE';
  data?: unknown;
  params?: object;
  headers?: Record<string, string>;
  signal?: AbortSignal;
  skipErrorHandler?: boolean;
  preserveLoginOnUnauthorized?: boolean;
};

export class RequestError extends Error {
  response?: { status: number; data: unknown };

  constructor(message: string, status?: number, data?: unknown) {
    super(message);
    this.name = 'RequestError';
    if (status !== undefined) this.response = { status, data };
  }
}

function withParams(path: string, params?: object) {
  if (!params) return path;
  const search = new URLSearchParams();
  Object.entries(params as Record<string, unknown>).forEach(([key, value]) => {
    if (value === undefined || value === null || value === '') return;
    if (Array.isArray(value)) value.forEach((item) => search.append(key, String(item)));
    else search.set(key, String(value));
  });
  const query = search.toString();
  return query ? `${path}${path.includes('?') ? '&' : '?'}${query}` : path;
}

export async function request<T>(
  path: string,
  options: RequestOptions = {},
): Promise<T> {
  const method = options.method || 'GET';
  const headers: Record<string, string> = {
    Accept: 'application/json',
    'Accept-Language': getLocale(),
    ...options.headers,
  };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  let body: BodyInit | undefined;
  if (options.data !== undefined) {
    if (options.data instanceof FormData) body = options.data;
    else {
      headers['Content-Type'] = 'application/json';
      body = JSON.stringify(options.data);
    }
  }

  let response: Response;
  try {
    response = await fetch(withParams(path, options.params), {
      method,
      headers,
      body,
      signal: options.signal,
    });
  } catch (error) {
    if ((error as Error).name === 'AbortError') throw error;
    throw new RequestError((error as Error).message || 'Network error');
  }

  const contentType = response.headers.get('content-type') || '';
  const payload = contentType.includes('application/json')
    ? await response.json().catch(() => null)
    : await response.text().catch(() => null);

  if (!response.ok) {
    const message =
      payload && typeof payload === 'object' && 'message' in payload
        ? String((payload as { message?: unknown }).message || `HTTP ${response.status}`)
        : `HTTP ${response.status}`;
    if (
      response.status === 401 &&
      !path.endsWith('/iam/login') &&
      !options.preserveLoginOnUnauthorized
    ) {
      useSession.getState().clear();
      history.replace(`/login?redirect=${encodeURIComponent(window.location.pathname + window.location.search)}`);
    }
    throw new RequestError(message, response.status, payload);
  }

  if (
    payload &&
    typeof payload === 'object' &&
    'code' in payload &&
    Number((payload as { code?: unknown }).code) === 401 &&
    !path.endsWith('/iam/login') &&
    !options.preserveLoginOnUnauthorized
  ) {
    useSession.getState().clear();
    history.replace('/login');
  }

  return payload as T;
}
