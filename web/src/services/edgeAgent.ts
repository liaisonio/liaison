import {request} from '@/api/client';

export type AgentConnector = {id:number;name:string;online:boolean;device?:string};
export type Installation = {id:string;kind:string;path:string;source:string};
export type AgentSkill = {id:string;name:string;description:string};
export type AgentActivity = {id:string;kind:string;status:string;message_index:number;duration_ms:number;command?:string;directory?:string;output?:string;exit_code?:number;truncated?:boolean;changes?:{path:string;kind:string;diff:string;move_path?:string}[]};
export type AgentInputRequest={id:string;blocking:boolean;questions:{id:string;header:string;question:string;is_other:boolean;is_secret:boolean;options?:{label:string;description:string}[]}[]};
export type AgentDirectory = {name:string;path:string};
export type AgentModel = {id:string;name:string};
export type AgentSessionSummary = {session_id:string;thread_id:string;title:string;project:string;updated_at:string;running:boolean;closed:boolean;status:string};
export type AgentSnapshot = {window?:number;history_windowing?:boolean;input_requests?:AgentInputRequest[];approvals?:{id:string;command:string;directory:string;reason:string}[];permissions_available?:boolean;permission_mode?:string;history_persistent?:boolean;archived?:boolean;history_total?:number;models?:AgentModel[];models_available?:boolean;title?:string;session_management?:boolean;sessions?:AgentSessionSummary[];sessions_available?:boolean;revision?:number;activities?:AgentActivity[];directory?:string;parent_directory?:string;directories?:AgentDirectory[];directory_roots?:string[];version:number;status:string;installations?:Installation[];default_project?:string;session_id?:string;thread_id?:string;agent_version?:string;model?:string;project?:string;started_at?:string;skills?:AgentSkill[];skills_available?:boolean;running:boolean;closed:boolean;messages?:{role:'user'|'assistant';text:string}[];truncated?:boolean};
export type AgentAccess = {id:string;name:string;kind:string;edge_id:number;installation_id:string;project:string;session_count?:number};
export async function agentAccesses(page=1,signal?:AbortSignal,filters?:{name:string;kind:string}){
  const query=new URLSearchParams({page:String(page),page_size:'20',...filters});
  const r=await request<API.Response<{items:AgentAccess[];total:number}>>(`/api/v1/agent-accesses?${query}`,{signal,skipErrorHandler:true});return r.data!;
}
export async function getAgentAccess(id:string,signal?:AbortSignal){
  const r=await request<API.Response<AgentAccess>>(`/api/v1/agent-accesses/${encodeURIComponent(id)}`,{signal,skipErrorHandler:true});return r.data!;
}
export async function saveAgentAccess(entry:Omit<AgentAccess,'id'>,id?:string){
  const r=await request<API.Response<AgentAccess>>(`/api/v1/agent-accesses${id?`/${encodeURIComponent(id)}`:''}`,{method:id?'PUT':'POST',data:entry,skipErrorHandler:true});return r.data!;
}
export async function deleteAgentAccess(id:string){await request(`/api/v1/agent-accesses/${encodeURIComponent(id)}`,{method:'DELETE',skipErrorHandler:true});}
export async function agentConnectors(signal?:AbortSignal) {
  const response=await request<API.Response<AgentConnector[]>>('/api/v1/edge-agents/connectors',{signal,skipErrorHandler:true});
  return response.data || [];
}
export async function edgeAgent(edge:number,action:string,fields:Record<string,unknown>={},signal?:AbortSignal) {
  const response=await request<API.Response<AgentSnapshot>>('/api/v1/edge-agents',{method:'POST',data:{edge_id:edge,action,...fields},signal,skipErrorHandler:true});
  if (!response.data || response.data.version!==1) throw new Error('invalid agent response');
  return response.data;
}
