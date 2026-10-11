import {useEffect,useRef,useState} from 'react';
import {Plus,Info} from 'lucide-react';
import {Notice,StatusPill} from '@/components/ui';
import ProtocolIcon from '@/components/icons/ProtocolIcon';
import {useI18n} from '@/i18n';
import {agentApplications,edgeAgent,saveAgentApplication,type AgentApplication,type Installation} from '@/services/edgeAgent';
import {agentLabel} from '@/pages/EdgeAgent/providers';

export default function AgentProbe({edge,revision}:{edge:number;revision:number}){
 const {tr}=useI18n();
 const [items,setItems]=useState<Installation[]>([]),[apps,setApps]=useState<AgentApplication[]>([]);
 const [loading,setLoading]=useState(true),[error,setError]=useState(''),[adding,setAdding]=useState('');
 const generation=useRef(0);
 useEffect(()=>{
  const abort=new AbortController();const current=++generation.current;
  setLoading(true);setError('');setItems([]);setApps([]);setAdding('');
  const registered=async()=>{const result=await agentApplications(1,edge,abort.signal);let page=2;while(result.items.length<result.total){const more=await agentApplications(page++,edge,abort.signal);if(!more.items.length)break;result.items.push(...more.items);}return result.items;};
  void Promise.all([edgeAgent(edge,'discover',{},abort.signal),registered()]).then(([result,existing])=>{
   if(abort.signal.aborted||generation.current!==current)return;
   if(result.status!=='ok')throw new Error('discovery unavailable');
   setItems((result.installations||[]).filter(i=>i.kind==='codex'||i.kind==='claude'));setApps(existing);
  }).catch(()=>{if(!abort.signal.aborted)setError(tr('Agent 探测失败，请检查连接器版本和权限后重试。','Agent discovery failed. Check the connector version and permissions, then retry.'));}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>{abort.abort();generation.current++;};
 },[edge,revision]);
 async function add(item:Installation){
  if(adding)return;const current=generation.current;setAdding(item.id);setError('');
  try{const app=await saveAgentApplication({name:agentLabel(item.kind),kind:item.kind,edge_id:edge,installation_id:item.id});if(current===generation.current)setApps(previous=>[...previous,app]);}
  catch{if(current===generation.current)setError(tr('添加 Agent 应用失败，请检查权限后重试。','Could not add Agent application. Check permissions and retry.'));}
  finally{if(current===generation.current)setAdding('');}
 }
 return <section aria-label={tr('Agent 探测','Agent discovery')}>
  <div className="liaison-scan-toolbar"><span>Agent <b>{items.length}</b></span><StatusPill tone={loading?'info':error?'danger':'success'}>{loading?tr('探测中','Discovering'):error?tr('探测失败','Failed'):tr('探测完成','Completed')}</StatusPill></div>
  {error&&<Notice tone="danger">{error}</Notice>}
  <div className="liaison-scan-list" aria-busy={loading}>
   {items.map(item=>{const registered=apps.some(a=>a.kind===item.kind&&a.installation_id===item.id);return <div className="liaison-scan-row" style={{gridTemplateColumns:'minmax(0, 1fr) 24px 66px'}} key={item.id}>
    <span className="liaison-scan-target"><i><ProtocolIcon protocol={item.kind}/></i><span><strong>{agentLabel(item.kind)}</strong><small>{tr('本机安装','Local installation')}</small></span></span>
    <button type="button" className="liaison-table-link" title={item.path} aria-label={tr('安装路径：','Installation path: ')+item.path}><Info size={14}/></button>
    <button type="button" className="liaison-table-link" disabled={loading||registered||!!adding} onClick={()=>void add(item)}>{registered?tr('已添加','Added'):adding===item.id?tr('添加中','Adding'):<><Plus size={13}/>{tr('添加','Add')}</>}</button>
   </div>})}
   {loading?<div className="liaison-scan-empty" role="status">{tr('正在探测已安装的 Agent…','Discovering installed Agents…')}</div>:!error&&!items.length?<div className="liaison-scan-empty">{tr('未发现已安装的 Codex 或 Claude Code。','No installed Codex or Claude Code found.')}</div>:null}
  </div>
 </section>;
}
