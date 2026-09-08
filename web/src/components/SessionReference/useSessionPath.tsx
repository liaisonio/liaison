import { useCallback, useEffect, useRef, useState } from 'react';
import { useLocation, useNavigate } from 'react-router-dom';
import { useI18n } from '@/i18n';

export async function connectionReference(handle: string) {
 const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(handle));
 return `conn_${Array.from(new Uint8Array(digest)).map(v=>v.toString(16).padStart(2,'0')).join('').slice(0,24)}`;
}
export function useSessionPath(handle:string|undefined,localOpen:boolean,setOpen:(open:boolean)=>void){
 const location=useLocation(),navigate=useNavigate();
 const current=useRef(location);current.current=location;
 const navigation=useRef(navigate);navigation.current=navigate;
 const match=location.pathname.match(/\/sessions\/(conn_[a-f0-9]{24})(?:\/agent\/(session_[a-f0-9]+))?$/);
 const connectionId=match?.[1];
 const candidate=new URLSearchParams(location.search).get('agent')||match?.[2];
 const agentSessionId=candidate&&/^session_[a-f0-9]+$/.test(candidate)?candidate:undefined;
 const short=location.pathname.startsWith('/webssh/sessions/');
 const basePath=short?(location.state?.sessionRoute?.basePath||'/proxy?access_type=webssh'):location.pathname.split('/sessions/')[0];
 const [liveId,setLiveId]=useState('');
 const update=useCallback((connection:string,agent?:string)=>{
  const loc=current.current;const ssh=loc.pathname.startsWith('/webssh/');
  const search=new URLSearchParams(loc.search);search.delete('agent');if(agent)search.set('agent',agent);
  const oldBase=loc.pathname.split('/sessions/')[0];
  const oldRoute=oldBase.match(/^\/webssh\/(\d+)(?:\/connections\/(\d+)|\/session)?$/);
  const state=oldRoute?{...loc.state,sessionRoute:{proxyId:Number(oldRoute[1]),credentialId:Number(oldRoute[2]||0),basePath:oldBase}}:loc.state;
  const path=ssh?`/webssh/sessions/${connection}`:`${oldBase}/sessions/${connection}`;
  const url=path+(search.size?`?${search}`:'');
  if(url!==loc.pathname+loc.search)navigation.current(url,{replace:true,state});
 },[]);
 useEffect(()=>{if(connectionId)update(connectionId,agentSessionId);},[location.pathname,connectionId,agentSessionId,update]);
 useEffect(()=>{
  let active=true;setLiveId('');
  if(handle)void connectionReference(handle).then(id=>{if(!active)return;setLiveId(id);const loc=current.current;const same=loc.pathname.includes(`/sessions/${id}`);update(id,same?new URLSearchParams(loc.search).get('agent')||undefined:undefined);}).catch(()=>{/* Never expose bearer handles. */});
  return()=>{active=false;};
 },[handle,update]);
 const onAgentSessionReady=useCallback((id:string)=>{if(!/^session_[a-f0-9]+$/.test(id))return;const ref=current.current.pathname.match(/\/sessions\/(conn_[a-f0-9]{24})/)?.[1];if(ref)update(ref,id);},[update]);
 const closeAgent=()=>{setOpen(false);if(connectionId)update(connectionId);};
 const search=new URLSearchParams(location.search);search.delete('agent');
 return {connectionId,agentSessionId,basePath,agentOpen:localOpen||!!agentSessionId,onAgentSessionReady,closeAgent,
  toggleAgent:()=>localOpen||agentSessionId?closeAgent():setOpen(true),matching:!!handle&&!!liveId&&(!connectionId||liveId===connectionId),
  reconnectURL:basePath+(search.size?(basePath.includes('?')?'&':'?')+search:'')};
}
export function SessionPathNotice({show,href}:{show:boolean;href:string}){
 const {tr}=useI18n();if(!show)return null;
 return <div className="session-path-notice" role="status">{tr('此连接会话不在当前页面运行。历史对话仍可查看，继续操作请重新连接。','This connection is not running in this page. You can read its chat history; reconnect to continue.')} <a href={href}>{tr('重新连接','Reconnect')}</a></div>;
}
