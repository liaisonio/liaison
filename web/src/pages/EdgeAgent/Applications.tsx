import {agentKinds,agentLabel} from './providers';
import {useEffect,useState,type ReactNode} from 'react';
import {Plus} from 'lucide-react';
import {Button,DataTable,Field,Input,Modal,Notice,Pager,Select,StatusPill} from '@/components/ui';
import {useI18n} from '@/i18n';
import {agentApplications,agentConnectors,deleteAgentApplication,edgeAgent,saveAgentApplication,type AgentApplication,type AgentConnector,type Installation} from '@/services/edgeAgent';
import AccessForm from './AccessForm';
import ProtocolIcon from '@/components/icons/ProtocolIcon';
import './index.less';

export type AgentApplicationListView = {rows:AgentApplication[];connectors:AgentConnector[];loading:boolean;renderActions:(row:AgentApplication)=>ReactNode;createApplication:()=>void};
export default function AgentApplications({renderList}:{renderList?:(view:AgentApplicationListView)=>ReactNode}){
 const {tr}=useI18n();
 const [rows,setRows]=useState<AgentApplication[]>([]),[connectors,setConnectors]=useState<AgentConnector[]>([]),[page,setPage]=useState(1),[total,setTotal]=useState(0),[revision,setRevision]=useState(0),[loading,setLoading]=useState(true),[error,setError]=useState('');
 const [form,setForm]=useState<AgentApplication|null|undefined>(),[access,setAccess]=useState<AgentApplication>(),[remove,setRemove]=useState<AgentApplication>(),[pending,setPending]=useState(false);
 const combined=!!renderList;
 useEffect(()=>{const abort=new AbortController();setLoading(true);setError('');const loadApps=async()=>{const result=await agentApplications(combined?1:page,undefined,abort.signal);if(combined){let next=2;while(result.items.length<result.total){const more=await agentApplications(next++,undefined,abort.signal);if(!more.items.length)break;result.items.push(...more.items);}}return result;};void Promise.all([loadApps(),agentConnectors(abort.signal)]).then(([result,devices])=>{if(!abort.signal.aborted){setRows(result.items);setTotal(result.total);setConnectors(devices);}}).catch(()=>{if(!abort.signal.aborted)setError(tr('无法加载 Agent 应用，请检查权限或重试。','Cannot load Agent applications. Check permissions or retry.'));}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});return()=>abort.abort();},[page,revision,combined]);
 const renderActions=(r:AgentApplication)=><span className="liaison-table-actions"><button className="liaison-table-link" onClick={()=>setAccess(r)}>{tr('创建访问','Create access')}</button><button className="liaison-table-link" onClick={()=>setForm(r)}>{tr('编辑','Edit')}</button><button className="liaison-table-link is-danger" disabled={r.access_count>0} title={r.access_count?tr('请先移除关联访问','Remove linked accesses first'):undefined} onClick={()=>setRemove(r)}>{tr('删除','Delete')}</button></span>;
 async function removeApp(){if(!remove)return;setPending(true);setError('');try{await deleteAgentApplication(remove.id);setRemove(undefined);if(rows.length===1&&page>1)setPage(page-1);else setRevision(n=>n+1);}catch{setError(tr('无法删除应用，请先移除关联访问，或检查权限后重试。','Cannot delete application. Remove linked accesses first, or check permissions and retry.'));}finally{setPending(false);}}
 return <div className="liaison-page-stack">
  {error&&<Notice tone="danger">{error}<Button onClick={()=>setRevision(n=>n+1)}>{tr('重试','Retry')}</Button></Notice>}
  {renderList?renderList({rows,connectors,loading,renderActions,createApplication:()=>setForm(null)}):<section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('应用列表','Applications')}</h2><Button variant="primary" disabled={loading||!connectors.length} onClick={()=>setForm(null)}><Plus size={14}/>{tr('新建应用','Create application')}</Button></header>
  <DataTable rows={rows} loading={loading} rowKey={r=>r.id} emptyText={tr('暂无 Agent 应用，请登记设备上已安装的 Agent。','No Agent applications. Register an Agent already installed on your device.')} columns={[
   {key:'name',title:tr('应用名称','Application name'),render:r=>r.name},
   {key:'kind',title:tr('应用类型', 'Application type'),render:r=><span className="liaison-inline-name"><ProtocolIcon protocol={r.kind}/>{agentLabel(r.kind)}</span>},
   {key:'device',title:tr('连接器 / 设备','Connector / device'),render:r=>{const c=connectors.find(c=>c.id===r.edge_id);return c?`${c.name}${c.device?` · ${c.device}`:''}`:'—';}},
   {key:'status',title:tr('连接器状态','Connector status'),render:r=><StatusPill tone={connectors.find(c=>c.id===r.edge_id)?.online?'success':'neutral'}>{connectors.find(c=>c.id===r.edge_id)?.online?tr('在线','Online'):tr('离线','Offline')}</StatusPill>},
   {key:'access',title:tr('关联访问','Accesses'),render:r=>r.access_count},
   {key:'actions',title:tr('操作','Actions'),fixed:'right',render:renderActions},
  ]}/><Pager page={page} pageSize={100} total={total} onPageChange={setPage}/></section>}
  {form!==undefined&&<ApplicationForm entry={form||undefined} connectors={connectors} onClose={()=>setForm(undefined)} onSaved={()=>{setForm(undefined);setRevision(n=>n+1);}}/>}
  {access&&<AccessForm application={access} connectors={connectors} onClose={()=>setAccess(undefined)} onSaved={()=>{setAccess(undefined);setRevision(n=>n+1);}}/>}
  <Modal open={!!remove} title={tr('删除应用','Delete application')} onClose={()=>{if(!pending)setRemove(undefined);}} footer={<><Button disabled={pending} onClick={()=>setRemove(undefined)}>{tr('取消','Cancel')}</Button><Button variant="danger" loading={pending} onClick={()=>void removeApp()}>{tr('删除','Delete')}</Button></>}><p>{remove?.name}</p><p>{tr('仅删除登记记录，不卸载 Agent 或删除主机文件。','Removes the registration only, not the Agent installation or host files.')}</p></Modal>
 </div>;
}

function ApplicationForm({entry,connectors,onClose,onSaved}:{entry?:AgentApplication;connectors:AgentConnector[];onClose:()=>void;onSaved:()=>void}){
 const {tr}=useI18n();const [kind,setKind]=useState(entry?.kind||'codex');const [name,setName]=useState(entry?.name||'Codex'),[edge,setEdge]=useState(String(entry?.edge_id||connectors.find(c=>c.online)?.id||'')),[installation,setInstallation]=useState(''),[items,setItems]=useState<Installation[]>([]),[pending,setPending]=useState(false),[error,setError]=useState('');
 async function discover(){setPending(true);setError('');try{const result=await edgeAgent(Number(edge),'discover');if(result.status!=='ok')throw new Error('unavailable');const found=(result.installations||[]).filter(i=>i.kind===kind);setItems(found);setInstallation(found[0]?.id||'');if(!found.length)setError(tr('未找到此类型的 Agent。','No installation of this Agent type found.'));}catch{setError(tr('发现失败，请检查连接器后重试。','Discovery failed. Check the connector and retry.'));}finally{setPending(false);}}
 async function save(){setPending(true);setError('');try{await saveAgentApplication(entry?{name:name.trim()}:{name:name.trim(),kind,edge_id:Number(edge),installation_id:installation},entry?.id);onSaved();}catch{setError(tr('无法保存应用，请检查安装和连接器权限。','Cannot save application. Check the installation and connector permissions.'));}finally{setPending(false);}}
 return <Modal open title={entry?tr('编辑应用','Edit application'):tr('新建应用','Create application')} onClose={()=>{if(!pending)onClose();}} footer={<><Button disabled={pending} onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" loading={pending} disabled={pending||!name.trim()||(!entry&&(!edge||!installation))} onClick={()=>void save()}>{tr('保存','Save')}</Button></>}><div className="edge-agent-form">
 <Field label={tr('应用类型','Application type')}><Select value={kind} disabled={pending||!!entry} onChange={e=>{setKind(e.target.value);setName(agentLabel(e.target.value));setInstallation('');setItems([]);setError('');}}>{agentKinds.map(k=><option key={k.value} value={k.value}>{k.label}</option>)}</Select></Field>
 <Field label={tr('应用名称','Application name')} required><Input value={name} maxLength={120} disabled={pending} onChange={e=>setName(e.target.value)}/></Field>
 {!entry&&<><Field label={tr('连接器','Connector')} required><Select value={edge} disabled={pending} onChange={e=>{setEdge(e.target.value);setInstallation('');setItems([]);setError('');}}><option value="">{tr('选择连接器','Choose connector')}</option>{connectors.map(c=><option key={c.id} value={c.id}>{c.name} · {c.device}</option>)}</Select></Field><Button disabled={pending||!connectors.find(c=>c.id===Number(edge))?.online} loading={pending} onClick={()=>void discover()}>{tr('发现已安装的 Agent','Discover installed Agents')}</Button>{!!items.length&&<Field label={tr('Agent 安装','Agent installation')} required><Select disabled={pending} value={installation} onChange={e=>setInstallation(e.target.value)}>{items.map(i=><option key={i.id} value={i.id}>{i.path}</option>)}</Select></Field>}<Notice>{tr('只登记已有安装，不安装软件或启动会话。','Registers an existing installation without installing software or starting a session.')}</Notice></>}
 {error&&<Notice tone="danger">{error}</Notice>}</div></Modal>;
}
