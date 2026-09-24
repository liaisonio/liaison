import {useEffect,useRef,useState} from 'react';
import {Link,useSearchParams} from 'react-router-dom';
import {ChevronRight,Folder,MessageSquare,PanelLeft,Plus,RefreshCw,X} from 'lucide-react';
import {Button,Notice,Modal,DangerConfirm,Field,Input} from '@/components/ui';
import ActionMenu from '@/components/ui/ActionMenu';
import {useI18n} from '@/i18n';
import {RequestError} from '@/api/client';
import {edgeAgent,type AgentAccess,type AgentSessionSummary} from '@/services/edgeAgent';
import Workspace from './Workspace';

export default function SessionsWorkspace({access}:{access:AgentAccess}){
 const {tr}=useI18n(),[params,setParams]=useSearchParams();
 const [items,setItems]=useState<AgentSessionSummary[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState(''),[denied,setDenied]=useState(false);
 const [selection,setSelection]=useState<{key:number;id?:string;directory?:string}>(),[current,setCurrent]=useState('');
 const [collapsed,setCollapsed]=useState<Set<string>>(()=>new Set());
 const [persistent,setPersistent]=useState(false),[page,setPage]=useState(1),[total,setTotal]=useState(0);
 const [management,setManagement]=useState(false),[operation,setOperation]=useState<{kind:'rename'|'delete';row:AgentSessionSummary}>(),[title,setTitle]=useState(''),[saving,setSaving]=useState(false),[operationError,setOperationError]=useState('');
 const [open,setOpen]=useState(()=>window.innerWidth>=1000),[refresh,setRefresh]=useState(0);
 const initialized=useRef(false),requested=useRef(params.get('session'));
 useEffect(()=>{
  const abort=new AbortController();let timer:ReturnType<typeof setTimeout>;
  const load=async()=>{
   try{
    const data=await edgeAgent(access.edge_id,'sessions',{access_id:access.id,...(page>1?{history_page:String(page)}:{})},abort.signal);
    if(abort.signal.aborted)return;
    if(data.status!=='ok'||!data.sessions_available)throw new Error('unavailable');
    const rows=data.sessions||[];setItems(rows);setError('');setManagement(Boolean(data.session_management));
    setPersistent(Boolean(data.history_persistent));setTotal(data.history_total||rows.length);
    if(!initialized.current){initialized.current=true;const first=rows.find(r=>r.session_id===requested.current)||rows.find(r=>!r.closed)||rows[0];const id=requested.current||first?.session_id;setSelection({key:0,id});setCurrent(id||'');}
   }catch(e){if(abort.signal.aborted)return;if(e instanceof RequestError&&[401,403,404].includes(e.response?.status||0)){setDenied(true);setItems([]);setSelection(undefined);}
    setError(tr('无法加载会话列表，请检查连接器版本、连接状态与访问权限。','Cannot load sessions. Check the connector version, connection and access permissions.'));
   }finally{if(!abort.signal.aborted){setLoading(false);timer=setTimeout(load,5000);}}
  };void load();return()=>{abort.abort();clearTimeout(timer);};
 },[access.id,access.edge_id,refresh,page]);
 function choose(id?:string,directory?:string){setCurrent(id||'');setSelection(s=>({key:(s?.key||0)+1,id,directory}));setParams(p=>{const next=new URLSearchParams(p);if(id)next.set('session',id);else next.delete('session');return next;},{replace:true});if(window.innerWidth<1000)setOpen(false);}
 const groups=new Map<string,AgentSessionSummary[]>();
 for(const item of items){const rows=groups.get(item.project)||[];rows.push(item);groups.set(item.project,rows);}
 const status=(s:AgentSessionSummary)=>s.closed?tr('已结束','Ended'):s.running?tr('运行中','Running'):tr('空闲','Idle');
 async function manage(){
  if(!operation||saving)return;
  setSaving(true);setOperationError('');
  const {kind,row}=operation;
  try{
   if(kind==='delete'&&!row.closed){const ended=await edgeAgent(access.edge_id,'stop',{access_id:access.id,session_id:row.session_id});if(!ended.closed)throw new Error('not closed');}
   const data=await edgeAgent(access.edge_id,kind,{access_id:access.id,session_id:row.session_id,...(kind==='rename'?{title:title.trim()}:{})});
   if(kind==='delete'?data.status!=='ok':!data.session_id)throw new Error('failed');
   if(kind==='delete'){
    setItems(previous=>previous.filter(s=>s.session_id!==row.session_id));
    if(current===row.session_id){setCurrent('');setSelection(undefined);setParams(p=>{const next=new URLSearchParams(p);next.delete('session');return next;},{replace:true});}
   }
   setOperation(undefined);setRefresh(n=>n+1);
  }catch{setOperationError(tr('操作未完成，请刷新状态后重试。','Operation did not complete. Refresh the status and retry.'));}
  finally{setSaving(false);}
 }
 return <div className="edge-agent-sessions-page">
  <div className="edge-agent-toolbar"><nav className="edge-agent-breadcrumb" aria-label={tr('面包屑','Breadcrumb')}><Link to="/access/agents">Agent</Link><ChevronRight size={12}/><span>{access.name}</span></nav><Button aria-expanded={open} aria-controls="agent-session-list" onClick={()=>setOpen(v=>!v)}><PanelLeft size={15}/>{open?tr('收起会话','Hide sessions'):tr('展开会话','Show sessions')}</Button></div>
  {error&&<Notice tone="danger">{error}<Button onClick={()=>setRefresh(v=>v+1)}>{tr('重试','Retry')}</Button></Notice>}
  <div className={`edge-agent-session-layout ${open?'is-open':''}`}>
   {open&&<aside id="agent-session-list" className="edge-agent-session-list" aria-label={tr('会话列表','Session list')}>
    <header><strong>{tr('会话','Sessions')}</strong><Button variant="ghost" aria-label={tr('刷新会话','Refresh sessions')} onClick={()=>setRefresh(v=>v+1)}><RefreshCw size={14}/></Button></header>
    <Button disabled={loading||denied||!initialized.current} onClick={()=>choose(undefined,items.find(s=>s.session_id===current)?.project)}><Plus size={14}/>{tr('新建会话','New conversation')}</Button>
    <div className="edge-agent-session-items">
     {loading?<p role="status">{tr('正在加载…','Loading…')}</p>:!items.length?<p>{tr('暂无会话','No sessions yet')}</p>:null}
     {[...groups].map(([path,rows],index)=>{const expanded=!collapsed.has(path),name=path.split(/[\\/]/).filter(Boolean).pop()||path;return <section key={path} className="edge-agent-project-group">
      <div className="edge-agent-project-group-heading"><Button variant="ghost" title={path} aria-expanded={expanded} aria-controls={`agent-project-${index}`} onClick={()=>setCollapsed(previous=>{const next=new Set(previous);if(next.has(path))next.delete(path);else next.add(path);return next;})}><ChevronRight size={12} className={expanded?'is-expanded':''}/><Folder size={14}/><strong>{name}</strong><small>{rows.length}</small></Button><Button variant="ghost" disabled={denied} title={tr('在此项目新建会话','New conversation in this project')} aria-label={`${tr('新建会话','New conversation')} · ${path}`} onClick={()=>choose(undefined,path)}><Plus size={13}/></Button></div>
      <code className="edge-agent-group-path" title={path}>{path}</code>
      {expanded&&<div id={`agent-project-${index}`}>{rows.map(s=><div key={s.session_id} className="edge-agent-session-row"><Button key={s.session_id} variant="ghost" className={`edge-agent-session-item ${current===s.session_id?'is-selected':''}`} aria-current={current===s.session_id?'true':undefined} onClick={()=>choose(s.session_id)}><span className="edge-agent-session-title"><MessageSquare size={13}/><strong title={s.title}>{s.title||tr('新会话','New conversation')}</strong></span><small><span className={s.running?'is-running':''}>{status(s)}</span><time dateTime={s.updated_at}>{new Date(s.updated_at).toLocaleString(tr('zh-CN','en-US'),{month:'numeric',day:'numeric',hour:'2-digit',minute:'2-digit',hour12:false})}</time></small></Button>{management&&<ActionMenu label={tr('会话操作','Conversation actions')} disabled={saving} items={[{label:tr('重命名','Rename'),onClick:()=>{setTitle(s.title);setOperationError('');setOperation({kind:'rename',row:s});}},{label:tr('删除','Delete'),danger:true,onClick:()=>{setOperationError('');setOperation({kind:'delete',row:s});}}]}/>}</div>)}</div>}
     </section>;})}
    </div>
    {persistent&&total>50&&<div className="edge-agent-history-pager"><Button disabled={page===1} onClick={()=>setPage(p=>p-1)}>{tr('上一页','Previous')}</Button><span>{page} / {Math.ceil(total/50)}</span><Button disabled={page*50>=total} onClick={()=>setPage(p=>p+1)}>{tr('下一页','Next')}</Button></div>}
    <footer>{persistent?tr('历史保存在服务端，不自动过期。空闲时仅结束运行实例。','History is stored on the server without automatic expiration. Idle instances may stop.'):tr('当前连接器使用临时历史，请升级服务端启用持久化。','Temporary history. Upgrade the server to enable persistence.')}</footer>
   </aside>}
<div className="edge-agent-session-main">{selection&&!denied?<Workspace key={selection.key} access={access} initialSessionID={selection.id} initialDirectory={selection.directory} onSessionRemoved={id=>{setItems(rows=>rows.filter(row=>row.session_id!==id));setCurrent('');setParams(p=>{const next=new URLSearchParams(p);next.delete('session');return next;},{replace:true});setRefresh(v=>v+1);}} onSessionReady={id=>{setCurrent(id);setParams(p=>{const next=new URLSearchParams(p);next.set('session',id);return next;},{replace:true});setRefresh(v=>v+1);}}/>:loading?<Notice>{tr('正在加载会话…','Loading sessions…')}</Notice>:<Notice>{tr('选择一个会话，或新建会话开始。','Select a conversation or start a new one.')}</Notice>}</div>
  </div>
  <Modal open={Boolean(operation)} title={operation?.kind==='rename'?tr('重命名会话','Rename conversation'):tr('删除会话','Delete conversation')} onClose={()=>{if(!saving)setOperation(undefined);}} footer={<><Button disabled={saving} onClick={()=>setOperation(undefined)}>{tr('取消','Cancel')}</Button><Button variant={operation?.kind==='delete'?'danger':'primary'} loading={saving} disabled={saving||(operation?.kind==='rename'&&!title.trim())} onClick={()=>void manage()}>{operation?.kind==='delete'?tr('删除','Delete'):tr('保存','Save')}</Button></>}>
   {operationError&&<Notice tone="danger">{operationError}</Notice>}
   {operation?.kind==='rename'?<Field label={tr('会话名称','Conversation name')}><Input value={title} maxLength={120} onChange={e=>setTitle(e.target.value)}/></Field>:<DangerConfirm title={operation?.row.title||tr('新会话','New conversation')} description={tr('将结束本次实例并删除 Liaison 中的会话记录，无法恢复。不会删除项目文件或 Codex 桌面端的其他会话。','Ends this instance and permanently removes its Liaison conversation. Project files and other Codex desktop conversations are unaffected.')}/>}
  </Modal>
 </div>;
}
