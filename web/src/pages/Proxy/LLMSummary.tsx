import {uniqueProtocolFamilies} from '@/constants/llmProtocols';
import {request} from '@/api/client';
import LLMProtocol from '@/components/icons/LLMProtocol';
import {useSession} from '@/store/session';
import {useI18n} from '@/i18n';
import {useEffect,useState} from 'react';
import {Link,useLocation} from 'react-router-dom';

type Summary={upstream_protocol:string;external_protocol:string;external_protocols?:string[];models:string[];enabled:boolean;can_manage?:boolean};
const pending=new Map<string,Promise<Summary>>();
export default function LLMSummary({id,revision,field}:{id:number;revision?:string;field:'protocol'|'models'|'external'}){
 const {tr}=useI18n(),owner=useSession(s=>s.token);
 const location=useLocation();
 const [state,setState]=useState<{key:string;value?:Summary;failed?:boolean}>();
 const [retry,setRetry]=useState(0);
 useEffect(()=>{const refresh=()=>setRetry(n=>n+1);window.addEventListener('liaison-llm-config-updated',refresh);return()=>window.removeEventListener('liaison-llm-config-updated',refresh);},[]);
 const key=JSON.stringify([owner,id,revision,retry]);
 useEffect(()=>{
  let alive=true;
  let read=pending.get(key);
  if(!read){read=request<API.Response<Summary>>(`/api/v1/ai/accesses/${id}/workspace`).then(r=>{if(r.code!==200||!r.data)throw Error('summary');return r.data;}).finally(()=>pending.delete(key));pending.set(key,read);}
  read.then(value=>{if(alive)setState({key,value});}).catch(()=>{if(alive)setState({key,failed:true});});
  return()=>{alive=false;};
 },[key,id]);
 if(state?.key!==key)return <span className="liaison-connection-muted" aria-busy="true">{tr('加载中…','Loading…')}</span>;
 if(state.failed)return <button className="liaison-table-link" onClick={()=>setRetry(v=>v+1)}>{tr('重试','Retry')}</button>;
 const value=state.value!;
 if(field==='models'?!value.models.length:field==='external'?!value.external_protocol:!value.upstream_protocol){
  if(!value.can_manage)return <span className="liaison-connection-muted">{tr('待配置','Not configured')}</span>;
  const search=new URLSearchParams(location.search);search.set('configure',String(id));
  return <Link className="liaison-table-link" to={`${location.pathname}?${search}`}>{tr('待配置','Set up')}</Link>;
 }
 if(field==='models')return <span title={value.models.join(', ')}>{value.models.length?<><code>{value.models.slice(0,2).join(', ')}</code>{value.models.length>2?` +${value.models.length-2}`:''}</>:tr('未配置','Not configured')}</span>;
 const protocol=field==='external'?value.external_protocol:value.upstream_protocol;
 if(field==='external' && value.external_protocols?.length)return <span className="liaison-inline-name">{uniqueProtocolFamilies(value.external_protocols).map(p=><LLMProtocol key={p} protocol={p}/>)}</span>;
 return <LLMProtocol protocol={protocol}/>;
}
