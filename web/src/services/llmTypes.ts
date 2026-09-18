import {request} from '@/api/client';
import {LLM_PROTOCOLS} from '@/constants/llmProtocols';

// Resolve only legacy labels using consumer-safe workspace metadata. Never
// fetch upstream configuration or credentials to populate a list/filter.
export async function resolveLegacyLLMTypes(proxies:API.Proxy[]):Promise<API.Proxy[]> {
 const resolved=new Map<number,string>();
 const candidates=proxies.filter(p=>p.access_protocol==='aiapi'&&p.application?.application_type==='llm');
 const unique=[...new Map(candidates.map(p=>[p.application!.id,p])).values()];
 let next=0;
 await Promise.all(Array.from({length:Math.min(4,unique.length)},async()=>{
  while(next<unique.length){const p=unique[next++];try{
   const r=await request<API.Response<{upstream_protocol:string}>>(`/api/v1/ai/accesses/${p.id}/workspace`,{skipErrorHandler:true});
   if(r.code===200&&LLM_PROTOCOLS.some(x=>x.value===r.data?.upstream_protocol))resolved.set(p.application!.id,r.data!.upstream_protocol);
  }catch{/* Keep unconfirmed records visible without inventing a protocol. */}}
 }));
 return proxies.map(p=>p.application&&resolved.has(p.application.id)?{...p,application:{...p.application,application_type:resolved.get(p.application.id)!}}:p);
}

export function applyResolvedLLMTypes(apps:API.Application[],proxies:API.Proxy[]):API.Application[]{
 const byId=new Map(proxies.filter(p=>p.application?.application_type!=='llm').map(p=>[p.application?.id,p.application?.application_type]));
 return apps.map(a=>a.application_type==='llm'&&byId.get(a.id)?{...a,application_type:byId.get(a.id)!}:a);
}
