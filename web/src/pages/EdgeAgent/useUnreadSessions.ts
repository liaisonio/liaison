import {useCallback,useRef,useState} from 'react';
import type {AgentSessionSummary} from '@/services/edgeAgent';

// Read markers belong to this browser tab and access, never another user's feed.
export function useUnreadSessions(access:string){
 const key=`liaison-agent-read:${access}`;
 const [read,setRead]=useState<Record<string,string>>(()=>{try{const value=JSON.parse(sessionStorage.getItem(key)||'{}');return value&&typeof value==='object'&&!Array.isArray(value)?value:{};}catch{return {};}});
 const seen=useRef(read);
 const save=useCallback((next:Record<string,string>)=>{seen.current=next;setRead(next);try{sessionStorage.setItem(key,JSON.stringify(next));}catch{/* Markers still work without storage. */}},[key]);
 const markRead=useCallback((id:string,token:string)=>{if(seen.current[id]!==token)save({...seen.current,[id]:token});},[save]);
 const observe=useCallback((rows:AgentSessionSummary[])=>{
  const next={...seen.current};let changed=false;
  for(const row of rows)if(row.reply_token&&next[row.session_id]===undefined){next[row.session_id]=row.reply_token;changed=true;}
  if(changed)save(next);
 },[save]);
 return {markRead,observe,isUnread:(row:AgentSessionSummary)=>!!row.reply_token&&read[row.session_id]!==undefined&&read[row.session_id]!==row.reply_token};
}
