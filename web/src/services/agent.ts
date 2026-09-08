import { request, RequestError } from '@/api/client';
import { getToken } from '@/store/session';

export type AgentModelSelection = {provider_id:string;model:string};
export type AgentModelChoice = AgentModelSelection & {provider_type:string;is_default:boolean};
export async function getAgentStatus() {
  return request<API.Response<{ enabled: boolean; models?:AgentModelChoice[] }>>('/api/v1/agent/status');
}

const normalizeDetail = (response: API.Response<API.AgentSessionDetail>) => {
  if (response.data) {
    response.data = {
      ...response.data,
      attachments: response.data.attachments || [],
      turns: response.data.turns || [],
      steps: response.data.steps || [],
      messages: response.data.messages || [],
      approvals: response.data.approvals || [],
    };
  }
  return response;
};

export async function createAgentSession(handleId: string, title: string) {
  return normalizeDetail(await request<API.Response<API.AgentSessionDetail>>('/api/v1/agent/sessions', {
    method: 'POST',
    data: { handle_id: handleId, title },
  }));
}

export async function createManagementSession(title: string) {
  return normalizeDetail(await request<API.Response<API.AgentSessionDetail>>('/api/v1/agent/sessions', {
    method: 'POST', data: { kind: 'management', title },
  }));
}

export async function listManagementSessions() {
  const response = await request<API.Response<{ items: API.AgentSession[] }>>('/api/v1/agent/sessions');
  return (response.data?.items || []).filter(session => session.kind === 'management' && session.status === 0);
}

export async function getAgentSession(sessionId: string) {
  return normalizeDetail(await request<API.Response<API.AgentSessionDetail>>(`/api/v1/agent/sessions/${sessionId}`));
}

export type AgentResourceReference = {type:'connector'|'device'|'application';id:string;name?:string};

export async function runAgentTurn(sessionId: string, prompt: string, selection?:AgentModelSelection, references?:AgentResourceReference[]) {
  return request<API.Response<API.AgentRunResult>>(`/api/v1/agent/sessions/${sessionId}/turns`, {
    method: 'POST',
    data: { prompt, model_selection:selection, references:references?.map(({type,id})=>({type,id})) },
  });
}

export async function resolveAgentApproval(
  sessionId: string,
  approvalId: string,
  decision: 'approve' | 'deny',
  note = '',
) {
  return request<API.Response<API.AgentRunResult>>(
    `/api/v1/agent/sessions/${sessionId}/approvals/${approvalId}`,
    { method: 'POST', data: { decision, note } },
  );
}

const parseSSEBlock = (block: string): API.AgentEvent | undefined => {
  const data = block
    .split('\n')
    .filter((line) => line.startsWith('data:'))
    .map((line) => line.slice(5).trimStart())
    .join('\n');
  if (!data) return undefined;
  try {
    return JSON.parse(data) as API.AgentEvent;
  } catch {
    return undefined;
  }
};

export async function streamAgentEvents(
  sessionId: string,
  signal: AbortSignal,
  onEvent: (event: API.AgentEvent) => void,
) {
  const headers: Record<string, string> = { Accept: 'text/event-stream' };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;
  const response = await fetch(`/api/v1/agent/sessions/${sessionId}/events`, { headers, signal });
  if (!response.ok || !response.body) {
    throw new RequestError('Agent event stream unavailable', response.status);
  }
  const reader = response.body.getReader();
  const decoder = new TextDecoder();
  let buffer = '';
  while (!signal.aborted) {
    const { value, done } = await reader.read();
    if (done) break;
    buffer += decoder.decode(value, { stream: true }).replace(/\r\n/g, '\n');
    let boundary = buffer.indexOf('\n\n');
    while (boundary >= 0) {
      const event = parseSSEBlock(buffer.slice(0, boundary));
      buffer = buffer.slice(boundary + 2);
      if (event) onEvent(event);
      boundary = buffer.indexOf('\n\n');
    }
  }
}
