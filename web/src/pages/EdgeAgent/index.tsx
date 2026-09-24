import {useEffect,useState} from 'react';
import {Link,useSearchParams} from 'react-router-dom';
import {MessageSquare,Plus,RefreshCw} from 'lucide-react';
import OverflowTabs from '@/components/ui/OverflowTabs';
import {useDebouncedValue} from '@/hooks/useDebouncedValue';
import {OpenAIIcon} from '@/components/AgentWorkspace/ProviderIcon';
import {Button,DataTable,DangerConfirm,Modal,Notice,Pager,StatusPill} from '@/components/ui';
import {useI18n} from '@/i18n';
import {agentAccesses,agentConnectors,deleteAgentAccess,getAgentAccess,type AgentAccess,type AgentConnector} from '@/services/edgeAgent';
import Workspace from './SessionsWorkspace';
import AccessForm from './AccessForm';
import AccessEmptyState from '@/pages/Proxy/AccessEmptyState';
import {history} from '@/lib/runtime';
import './index.less';
import '@/pages/Proxy/connection.less';

export default function EdgeAgent(){
  const {tr}=useI18n();const [params,setParams]=useSearchParams();const id=params.get('access');
  const [name,setName]=useState(''),[kind,setKind]=useState('');
  const searchName=useDebouncedValue(name);
  const [rows,setRows]=useState<AgentAccess[]>([]),[connectors,setConnectors]=useState<AgentConnector[]>([]),[total,setTotal]=useState(0),[page,setPage]=useState(1),[revision,setRevision]=useState(0);
  const [loading,setLoading]=useState(true),[error,setError]=useState(''),[entry,setEntry]=useState<AgentAccess>(),[form,setForm]=useState<AgentAccess|null|undefined>(),[remove,setRemove]=useState<AgentAccess>(),[deleting,setDeleting]=useState(false);
  useEffect(()=>{
    const abort=new AbortController();setLoading(true);setError('');setEntry(undefined);
    const load=async()=>{
      try{
        if(id){const result=await getAgentAccess(id,abort.signal);if(!abort.signal.aborted)setEntry(result);}
        else {const [list,devices]=await Promise.all([agentAccesses(page,abort.signal,{name:searchName,kind}),agentConnectors(abort.signal)]);if(!abort.signal.aborted){setRows(list.items);setTotal(list.total);setConnectors(devices);}}
      }catch(e){if(!abort.signal.aborted)setError(tr('无法加载 Agent 访问，请检查权限或重试。','Cannot load Agent access. Check permissions or retry.'));}
      finally{if(!abort.signal.aborted)setLoading(false);}
    };void load();return()=>abort.abort();
  },[id,page,revision,searchName,kind]);
  async function removeEntry(){if(!remove)return;setDeleting(true);try{await deleteAgentAccess(remove.id);setRemove(undefined);if(rows.length===1&&page>1)setPage(page-1);else setRevision(n=>n+1);}catch{setError(tr('删除失败，请重试。','Could not delete access. Try again.'));}finally{setDeleting(false);}}
  const empty=!loading&&!error&&total===0;
  const filtered=Boolean(name.trim()||kind);
  if(id)return entry?<Workspace key={entry.id} access={entry}/>:<div className="edge-agent-page"><Link to="/access/agents">{tr('返回 Agent 访问','Back to Agent access')}</Link><Notice tone={error?'danger':'info'}>{error||tr('正在加载…','Loading…')}</Notice>{error&&<Button onClick={()=>setRevision(n=>n+1)}>{tr('重试','Retry')}</Button>}</div>;
  return <div className="liaison-page-stack liaison-access-list edge-agent-access-list">
    <OverflowTabs label={tr('访问类型','Access types')} value={kind} onChange={value=>{setKind(value);setPage(1);}} items={[{value:'',label:tr('全部','All')},{value:'codex',label:'Codex'}]}/>
    {error&&<Notice tone="danger">{error}</Notice>}
    <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('访问名称','Access')}</span><input value={name} maxLength={120} onChange={e=>{setName(e.target.value);setPage(1);}} placeholder={tr('输入访问名称','Access name')}/></label><div className="liaison-filter-actions"><Button onClick={()=>{setName('');setPage(1);}}>{tr('重置','Reset')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('访问列表','Access')}</h2><div className="edge-agent-actions"><Button aria-label={tr('刷新','Refresh')} onClick={()=>setRevision(n=>n+1)} disabled={loading}><RefreshCw size={14}/></Button>{!empty&&!error&&<Button variant="primary" onClick={()=>setForm(null)} disabled={loading||!connectors.length}><Plus size={14}/>{tr('新建访问','Create access')}</Button>}</div></header>{empty?<AccessEmptyState category="agent" onReset={filtered?()=>{setName('');setKind('');setPage(1);}:undefined} onCreate={()=>connectors.length?setForm(null):history.push('/connector?create=1')} actionLabel={!connectors.length?tr('创建连接器','Create connector'):undefined}/>:<><DataTable rows={rows} loading={loading} rowKey={r=>r.id} emptyText={tr('暂无 Agent 访问','No Agent access yet')} columns={[
      {key:'name',title:tr('访问名称','Access'),width:230,render:r=><Link className="liaison-access-app-link edge-agent-access-name" title={r.name} to={`?access=${r.id}`}>{r.name}</Link>},
      {key:'kind',title:tr('访问协议','Protocol'),width:140,render:()=> <span className="liaison-inline-name"><OpenAIIcon size={16}/><span>Codex</span></span>},
      {key:'sessions',title:tr('会话数量','Conversations'),width:130,render:r=>r.session_count===undefined?'—':<Link className="liaison-table-link edge-agent-conversation-count" to={`?access=${r.id}`} aria-label={tr(`查看 ${r.name} 的 ${r.session_count} 个会话`,`View ${r.session_count} conversations for ${r.name}`)}><MessageSquare size={13} aria-hidden/><span>{r.session_count}</span></Link>},
      {key:'state',title:tr('连接器状态','Connector status'),width:140,render:r=>{const c=connectors.find(c=>c.id===r.edge_id);return <StatusPill tone={c?.online?'success':'neutral'}>{c?c.online?tr('在线','Online'):tr('离线','Offline'):tr('不可用','Unavailable')}</StatusPill>;}},
      {key:'actions',title:tr('操作','Actions'),width:'1%',fixed:'right',render:r=><span className="liaison-table-actions liaison-access-actions"><button className="liaison-table-link" onClick={()=>setParams({access:r.id})}>{tr('去访问','Open')}</button><button className="liaison-table-link" onClick={()=>setForm(r)}>{tr('编辑','Edit')}</button><button className="liaison-table-link is-danger" onClick={()=>setRemove(r)}>{tr('删除','Delete')}</button></span>},
    ]}/><Pager page={page} pageSize={20} total={total} onPageChange={setPage}/></>}</section>
    {form!==undefined&&<AccessForm entry={form||undefined} connectors={connectors} onClose={()=>setForm(undefined)} onSaved={()=>{setForm(undefined);setPage(1);setRevision(n=>n+1);}}/>}
    <Modal open={Boolean(remove)} title={tr('删除访问','Delete access')} onClose={()=>{if(!deleting)setRemove(undefined);}} footer={<><Button disabled={deleting} onClick={()=>setRemove(undefined)}>{tr('取消','Cancel')}</Button><Button variant="danger" loading={deleting} onClick={()=>void removeEntry()}>{tr('删除','Delete')}</Button></>}><DangerConfirm title={remove?.name||''} description={tr('将删除此访问及其会话历史，无法恢复。不会卸载 Agent 或删除项目文件。','Permanently deletes this access and its conversation history. The Agent installation and project files are not removed.')}/></Modal>
  </div>;
}
