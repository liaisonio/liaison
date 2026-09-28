import {request} from '@/api/client';

export type AgentConnector = {id:number;name:string;online:boolean;device?:string};
export type Installation = {id:string;kind:string;path:string;source:string};
export type AgentSkill = {id:string;name:string;description:string};
export type AgentActivity = {label?:string;plan?:{step:string;status:string}[];id:string;kind:string;status:string;message_index:number;duration_ms:number;command?:string;directory?:string;output?:string;exit_code?:number;truncated?:boolean;changes?:{path:string;kind:string;diff:string;move_path?:string}[]};
export type AgentInputRequest={id:string;blocking:boolean;questions:{id:string;header:string;question:string;is_other:boolean;is_secret:boolean;options?:{label:string;description:string}[]}[]};
export type AgentDirectory = {name:string;path:string};
export type AgentModel = {id:string;name:string};
export type AgentSessionSummary = {reply_token?:string;attention?:'approval'|'input'|'failed';session_id:string;thread_id:string;title:string;project:string;updated_at:string;running:boolean;closed:boolean;status:string};
export type AgentSnapshot = {reply_token?:string;attention_sessions?:AgentSessionSummary[];files_available?:boolean;files_upload_available?:boolean;history_pages?:AgentSnapshot[];history_before?:string;window?:number;history_windowing?:boolean;input_requests?:AgentInputRequest[];approvals?:{id:string;kind?:string;changes?:AgentActivity['changes'];command:string;directory:string;reason:string}[];permissions_available?:boolean;permission_mode?:string;history_persistent?:boolean;archived?:boolean;history_total?:number;models?:AgentModel[];models_available?:boolean;title?:string;session_management?:boolean;sessions?:AgentSessionSummary[];sessions_available?:boolean;revision?:number;activities?:AgentActivity[];directory?:string;parent_directory?:string;directories?:AgentDirectory[];directory_roots?:string[];version:number;status:string;installations?:Installation[];default_project?:string;session_id?:string;thread_id?:string;agent_version?:string;model?:string;project?:string;started_at?:string;skills?:AgentSkill[];skills_available?:boolean;running:boolean;closed:boolean;messages?:{role:'user'|'assistant';text:string;attachments?:import('@/pages/EdgeAgent/Files').AgentFile[]}[];truncated?:boolean};
export type AgentApplication = {id:string;name:string;kind:string;edge_id:number;installation_id:string;access_count:number;created_at?:string};
export type AgentAccess = {application_name?:string;application_id?:string;id:string;name:string;kind:string;edge_id:number;installation_id:string;project:string;session_count?:number};
export async function agentApplications(page=1,edge?:number,signal?:AbortSignal){
 const query=new URLSearchParams({page:String(page),page_size:'100'});if(edge)query.set('edge_id',String(edge));
 const r=await request<API.Response<{items:AgentApplication[];total:number}>>(`/api/v1/agent-applications?${query}`,{signal,skipErrorHandler:true});return r.data!;
}
export async function saveAgentApplication(input:{name:string;kind?:string;edge_id?:number;installation_id?:string},id?:string){
 const r=await request<API.Response<AgentApplication>>(`/api/v1/agent-applications${id?`/${encodeURIComponent(id)}`:''}`,{method:id?'PUT':'POST',data:input,skipErrorHandler:true});return r.data!;
}
export async function deleteAgentApplication(id:string){await request(`/api/v1/agent-applications/${encodeURIComponent(id)}`,{method:'DELETE',skipErrorHandler:true});}
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
  const response=await request<API.Response<AgentSnapshot & {history_search_available?:boolean}>>('/api/v1/edge-agents',{method:'POST',data:{edge_id:edge,action,...fields},signal,skipErrorHandler:true});
  if (!response.data || response.data.version!==1) throw new Error('invalid agent response');
  return response.data;
}
