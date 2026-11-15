import {useEffect,useRef,useState} from 'react';
import {Plus,Info,RefreshCw,TriangleAlert} from 'lucide-react';
import {Notice,StatusPill,Button} from '@/components/ui';
import ProtocolIcon from '@/components/icons/ProtocolIcon';
import {useI18n} from '@/i18n';
import {agentApplications,edgeAgent,saveAgentApplication,type AgentApplication,type Installation} from '@/services/edgeAgent';
import './agentProbe.less';
import {agentLabel} from '@/pages/EdgeAgent/providers';
import {agentProbeError,AgentProbeStatusError} from '@/services/agent-probe-error';

export default function AgentProbe({edge,revision,onState,onLoading}:{onLoading?:(loading:boolean)=>void;edge:string|number;revision:number;onState?:(state:'loading'|'failed'|'completed')=>void}){
 const {tr}=useI18n();
 const [items,setItems]=useState<Installation[]>([]),[apps,setApps]=useState<AgentApplication[]>([]);
 const [loading,setLoading]=useState(true),[error,setError]=useState(''),[adding,setAdding]=useState('');
 const [retry,setRetry]=useState(0),[addError,setAddError]=useState('');
 useEffect(()=>{onLoading?.(loading);onState?.(loading?'loading':error?'failed':'completed');},[loading,error,onState,onLoading]);
 const generation=useRef(0);
 useEffect(()=>{
  const abort=new AbortController();const current=++generation.current;
  setLoading(true);setError('');setAddError('');setItems([]);setApps([]);setAdding('');
  const registered=async()=>{const result=await agentApplications(1,edge,abort.signal);let page=2;while(result.items.length<result.total){const more=await agentApplications(page++,edge,abort.signal);if(!more.items.length)break;result.items.push(...more.items);}return result.items;};
  void Promise.all([edgeAgent(edge,'discover',{},abort.signal),registered()]).then(([result,existing])=>{
   if(abort.signal.aborted||generation.current!==current)return;
   if(result.status!=='ok')throw new AgentProbeStatusError(result.status);
   setItems((result.installations||[]).filter(i=>i.kind==='codex'||i.kind==='claude'));setApps(existing);
  }).catch(error=>{if(!abort.signal.aborted&&generation.current===current)setError(agentProbeError(error,tr));}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>{abort.abort();generation.current++;};
 },[edge,revision,retry]);
 async function add(item:Installation){
  if(adding)return;const current=generation.current;setAdding(item.id);setAddError('');
  try{const app=await saveAgentApplication({name:agentLabel(item.kind),kind:item.kind,edge_id:edge,installation_id:item.id});if(current===generation.current)setApps(previous=>[...previous,app]);}
  catch{if(current===generation.current)setAddError(tr('添加 Agent 应用失败，请检查权限后重试。','Could not add Agent application. Check permissions and retry.'));}
  finally{if(current===generation.current)setAdding('');}
 }
 return <section aria-label={tr('Agent 探测','Agent discovery')}>
  <div className="liaison-scan-toolbar"><span>Agent {!loading&&!error&&<b>{items.length}</b>}</span><StatusPill tone={loading?'info':error?'warning':'success'}>{loading?tr('探测中','Discovering'):error?tr('未完成','Incomplete'):tr('探测完成','Completed')}</StatusPill></div>
  {error&&<div className="liaison-agent-probe-feedback" role="status"><TriangleAlert size={17}/><div><strong>{tr('暂时无法获取 Agent 列表','Agent list is unavailable')}</strong><p>{error}</p><small>{tr('不影响下方网络服务的查看和添加。','You can still view and add network services below.')}</small></div><Button disabled={loading||!!adding} onClick={()=>setRetry(value=>value+1)}><RefreshCw size={14}/>{tr('重试 Agent 探测','Retry Agent discovery')}</Button></div>}
  {addError&&<Notice tone="danger">{addError}</Notice>}
  {(!error||items.length>0)&&<div className="liaison-scan-list" aria-busy={loading}>
   {items.map(item=>{const registered=apps.some(a=>a.kind===item.kind&&a.installation_id===item.id);return <div className="liaison-scan-row" style={{gridTemplateColumns:'minmax(0, 1fr) 24px 66px'}} key={item.id}>
    <span className="liaison-scan-target"><i><ProtocolIcon protocol={item.kind}/></i><span><strong>{agentLabel(item.kind)}</strong><small>{tr('本机安装','Local installation')}</small></span></span>
    <button type="button" className="liaison-table-link" title={item.path} aria-label={tr('安装路径：','Installation path: ')+item.path}><Info size={14}/></button>
    <button type="button" className="liaison-table-link" disabled={loading||registered||!!adding} onClick={()=>void add(item)}>{registered?tr('已添加','Added'):adding===item.id?tr('添加中','Adding'):<><Plus size={13}/>{tr('添加','Add')}</>}</button>
   </div>})}
   {loading?<div className="liaison-scan-empty" role="status">{tr('正在探测已安装的 Agent…','Discovering installed Agents…')}</div>:!error&&!items.length?<div className="liaison-scan-empty">{tr('未发现已安装的 Codex 或 Claude Code。','No installed Codex or Claude Code found.')}</div>:null}
  </div>}
 </section>;
}
