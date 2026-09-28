import {useCallback,useEffect,useRef,useState} from 'react';
import {Link,useSearchParams} from 'react-router-dom';
import {ChevronRight,Folder,LoaderCircle,Maximize2,Minimize2,PanelLeft,PanelLeftClose,Plus,RefreshCw,X, CircleAlert, CircleHelp} from 'lucide-react';
import {OpenAIIcon} from '@/components/AgentWorkspace/ProviderIcon';
import {Button,Notice,Modal,DangerConfirm,Field,Input} from '@/components/ui';
import ActionMenu from '@/components/ui/ActionMenu';
import {useI18n} from '@/i18n';
import {RequestError} from '@/api/client';
import {edgeAgent,type AgentAccess,type AgentSessionSummary} from '@/services/edgeAgent';
import Workspace from './Workspace';
import {useSessionResize} from './SessionResize';
import {ProjectPath} from './ProjectPath';
import {useProjectListState} from './useProjectListState';
import {useUnreadSessions} from './useUnreadSessions';
import {DirectoryBrowser} from './DirectoryBrowser';
import {useSession} from '@/store/session';
import {agentDraftKey,writeAgentDraft} from '@/store/agentDraft';

export default function SessionsWorkspace({access}:{access:AgentAccess}){
 const {tr}=useI18n(),[params,setParams]=useSearchParams();
 const resize=useSessionResize();
 const [items,setItems]=useState<AgentSessionSummary[]>([]),[loading,setLoading]=useState(true),[error,setError]=useState(''),[denied,setDenied]=useState(false);
 const [search,setSearch]=useState(''),[query,setQuery]=useState(''),[attention,setAttention]=useState<AgentSessionSummary[]>([]);
 const unread=useUnreadSessions(access.id);
 const owner=useSession(s=>s.initialState.currentUser?.id);
 const [newDirectory,setNewDirectory]=useState<string>();
 function requestNew(directory?:string){setNewDirectory(directory||items.find(s=>s.session_id===current)?.project||access.project);}
 useEffect(()=>{const timer=setTimeout(()=>{setQuery(search.trim());setPage(1);},250);return()=>clearTimeout(timer);},[search]);
 const [selection,setSelection]=useState<{key:number;id?:string;directory?:string}>(),[current,setCurrent]=useState('');
 const [collapsed,setCollapsed]=useProjectListState(access.id,'collapsed');
 const [showAll,setShowAll]=useProjectListState(access.id,'expanded');
 const [persistent,setPersistent]=useState(false),[page,setPage]=useState(1),[total,setTotal]=useState(0);
 const [management,setManagement]=useState(false),[operation,setOperation]=useState<{kind:'rename'|'delete';row:AgentSessionSummary}>(),[title,setTitle]=useState(''),[saving,setSaving]=useState(false),[operationError,setOperationError]=useState('');
 const [open,setOpen]=useState(()=>window.innerWidth>=1000),[refresh,setRefresh]=useState(0);
 // Keep the view in the URL so reloads restore it without affecting other tabs.
 const expanded=params.get('view')==='full';
 const setExpanded=useCallback((value:boolean|((previous:boolean)=>boolean))=>{
  setParams(previous=>{
   const next=new URLSearchParams(previous);
   const full=typeof value==='function'?value(previous.get('view')==='full'):value;
   if(full)next.set('view','full');else next.delete('view');
   return next;
  },{replace:true});
 },[setParams]);
 const pageRoot=useRef<HTMLDivElement>(null);
 useEffect(()=>{
  if(!expanded)return;
  // Keep the workspace mounted: expanding must not resume or recreate a session.
  const siblings:HTMLElement[]=[];
  let node:HTMLElement|null=pageRoot.current;
  while(node&&node!==document.body){
   for(const sibling of Array.from(node.parentElement?.children||[])){
    if(sibling!==node&&sibling instanceof HTMLElement&&!sibling.inert){sibling.inert=true;siblings.push(sibling);}
   }
   node=node.parentElement;
  }
  const escape=(event:KeyboardEvent)=>{
   if(event.key!=='Escape'||event.defaultPrevented||document.querySelector('[role="dialog"], [role="listbox"], [role="menu"]'))return;
   setExpanded(false);pageRoot.current?.querySelector<HTMLButtonElement>('[data-page-expand]')?.focus();
  };
  // Inspect overlays before their own Escape handlers synchronously unmount them.
  window.addEventListener('keydown',escape,true);
  return()=>{window.removeEventListener('keydown',escape,true);siblings.forEach(sibling=>{sibling.inert=false;});};
 },[expanded,setExpanded]);
 const initialized=useRef(false),requested=useRef(params.get('session'));
 useEffect(()=>{
  const abort=new AbortController();let timer:ReturnType<typeof setTimeout>,lastSearch=0;
  setLoading(true);
  const load=async(searchResults=true)=>{
   try{
    const data=await edgeAgent(access.edge_id,'sessions',{access_id:access.id,...(searchResults&&page>1?{history_page:String(page)}:{}),...(searchResults&&query?{history_search:query}:{})},abort.signal);
    if(abort.signal.aborted)return;
    if(data.status!=='ok'||!data.sessions_available)throw new Error('unavailable');
    if(query&&searchResults&&!data.history_search_available){setItems([]);setTotal(0);setError(tr('当前服务端不支持全历史搜索，请升级后使用。','This server does not support history search. Upgrade to use it.'));return;}
    const rows=data.sessions||[];setAttention(data.attention_sessions||[]);unread.observe(rows);
    if(!searchResults&&query){setError('');setItems(previous=>previous.map(row=>{const live=rows.find(s=>s.session_id===row.session_id);return live?{...row,reply_token:live.reply_token,attention:live.attention,running:live.running,closed:live.closed,status:live.status}:row;}));return;}
    if(query)lastSearch=Date.now();
    setItems(rows);setError('');setManagement(Boolean(data.session_management));
    setPersistent(Boolean(data.history_persistent));setTotal(data.history_total||rows.length);
    if(!initialized.current){initialized.current=true;const first=rows.find(r=>r.session_id===requested.current)||rows.find(r=>!r.closed)||rows[0];const id=requested.current||first?.session_id;setSelection({key:0,id});setCurrent(id||'');}
   }catch(e){if(abort.signal.aborted)return;setAttention([]);if(query&&searchResults)setItems([]);if(e instanceof RequestError&&[401,403,404].includes(e.response?.status||0)){setDenied(true);setItems([]);setSelection(undefined);}
    setError(tr('无法加载会话列表，请检查连接器版本、连接状态与访问权限。','Cannot load sessions. Check the connector version, connection and access permissions.'));
   }finally{if(!abort.signal.aborted){setLoading(false);timer=setTimeout(()=>void load(!query||Date.now()-lastSearch>=30000),5000);}}
  };void load();return()=>{abort.abort();clearTimeout(timer);};
 },[access.id,access.edge_id,refresh,page,query]);
 function choose(id?:string,directory?:string){setCurrent(id||'');setSelection(s=>({key:(s?.key||0)+1,id,directory}));setParams(p=>{const next=new URLSearchParams(p);if(id)next.set('session',id);else next.delete('session');return next;},{replace:true});if(window.innerWidth<1000)setOpen(false);}
 const groups=new Map<string,AgentSessionSummary[]>();
 for(const item of items){const rows=groups.get(item.project)||[];rows.push(item);groups.set(item.project,rows);}
 const status=(s:AgentSessionSummary)=>s.closed?tr('已结束','Ended'):s.running?tr('运行中','Running'):tr('空闲','Idle');
 const attentionLabel=(s:AgentSessionSummary)=>s.attention==='approval'?tr('等待确认','Awaiting approval'):s.attention==='input'?tr('等待回答','Awaiting answer'):tr('执行失败','Failed');
 async function manage(){
  if(!operation||saving)return;
  setSaving(true);setOperationError('');
  const {kind,row}=operation;
  try{
   if(kind==='delete'&&!row.closed){const ended=await edgeAgent(access.edge_id,'stop',{access_id:access.id,session_id:row.session_id});if(!ended.closed)throw new Error('not closed');}
   const data=await edgeAgent(access.edge_id,kind,{access_id:access.id,session_id:row.session_id,...(kind==='rename'?{title:title.trim()}:{})});
   if(kind==='delete'?data.status!=='ok':!data.session_id)throw new Error('failed');
   if(kind==='delete'){
    writeAgentDraft(agentDraftKey(owner,access.id,row.session_id),'');
    setItems(previous=>previous.filter(s=>s.session_id!==row.session_id));
    if(current===row.session_id){setCurrent('');setSelection(undefined);setParams(p=>{const next=new URLSearchParams(p);next.delete('session');return next;},{replace:true});}
   }
   setOperation(undefined);setRefresh(n=>n+1);
  }catch{setOperationError(tr('操作未完成，请刷新状态后重试。','Operation did not complete. Refresh the status and retry.'));}
  finally{setSaving(false);}
 }
 return <div ref={pageRoot} className={`edge-agent-sessions-page${expanded?' is-page-expanded':''}`}>
  <div className="edge-agent-toolbar"><nav className="edge-agent-breadcrumb" aria-label={tr('面包屑','Breadcrumb')}><Link to="/access/agents">Agent</Link><ChevronRight size={12}/><span className="edge-agent-kind"><OpenAIIcon size={14}/>Codex</span><ChevronRight size={12}/><span title={access.name}>{access.name}</span></nav><div className="edge-agent-view-actions"><Button aria-expanded={open} aria-controls="agent-session-list" onClick={()=>setOpen(v=>!v)}>{open?<PanelLeftClose size={15}/>:<PanelLeft size={15}/>} {open?tr('收起会话列表','Hide conversations'):tr('显示会话列表','Show conversations')}</Button><Button data-page-expand aria-pressed={expanded} title={expanded?tr('退出全页面 · Esc','Exit full page · Esc'):tr('全页面展开','Expand to full page')} onClick={()=>setExpanded(v=>!v)}>{expanded?<Minimize2 size={15}/>:<Maximize2 size={15}/>} {expanded?tr('退出全页面','Exit full page'):tr('全页面展开','Expand to full page')}</Button></div></div>
  {error&&<Notice tone="danger">{error}<Button onClick={()=>setRefresh(v=>v+1)}>{tr('重试','Retry')}</Button></Notice>}
  <div ref={resize.layout} style={resize.style} className={`edge-agent-session-layout ${open?'is-open':''} ${resize.dragging?'is-resizing':''}`}>
   {open&&<aside id="agent-session-list" className="edge-agent-session-list" aria-label={tr('会话列表','Session list')}>
    <header><strong>{tr('会话','Sessions')}</strong><Button variant="ghost" aria-label={tr('刷新会话','Refresh sessions')} onClick={()=>setRefresh(v=>v+1)}><RefreshCw size={14}/></Button></header>
    <Button disabled={loading||denied||!initialized.current} onClick={()=>requestNew()}><Plus size={14}/>{tr('新建会话','New conversation')}</Button>
    <Input type="search" maxLength={100} value={search} aria-label={tr('搜索全部会话','Search all conversations')} placeholder={tr('搜索会话或项目','Search conversations or projects')} onChange={e=>setSearch(e.target.value)}/>
    {!!attention.length&&<div className="edge-agent-attention" aria-label={tr('待处理会话','Conversations needing attention')}>{attention.map(s=><Button key={s.session_id} variant="ghost" title={`${s.title} · ${attentionLabel(s)}`} onClick={()=>choose(s.session_id)}><CircleAlert size={13}/><span>{s.title||tr('新会话','New conversation')}</span><small>{attentionLabel(s)}</small></Button>)}</div>}
    <div className="edge-agent-session-items">
     {loading||search.trim()!==query?<p role="status">{tr('正在加载…','Loading…')}</p>:!items.length?<p>{query?tr('没有匹配的会话','No matching conversations'):tr('暂无会话','No sessions yet')}</p>:null}
     {[...groups].map(([path,rows],index)=>{const expanded=!!query||!collapsed.has(path),name=path.split(/[\\/]/).filter(Boolean).pop()||path;const visible=query||showAll.has(path)?rows:rows.filter((s,i)=>i<8||s.session_id===current);return <section key={path} className="edge-agent-project-group">
      <div className="edge-agent-project-group-heading"><Button variant="ghost" aria-label={`${expanded?tr('收起项目','Collapse project'):tr('展开项目','Expand project')} · ${name}`} aria-expanded={expanded} aria-controls={`agent-project-${index}`} onClick={()=>setCollapsed(previous=>{const next=new Set(previous);if(next.has(path))next.delete(path);else next.add(path);return next;})}><ChevronRight size={12} className={expanded?'is-expanded':''}/><Folder size={14}/></Button><ProjectPath path={path} name={name}/><Button variant="ghost" disabled={denied} title={tr('在此项目新建会话','New conversation in this project')} aria-label={`${tr('新建会话','New conversation')} · ${path}`} onClick={()=>requestNew(path)}><Plus size={13}/></Button></div>
      {expanded&&<div id={`agent-project-${index}`}>{visible.map(s=><div key={s.session_id} className="edge-agent-session-row"><Button variant="ghost" className={`edge-agent-session-item ${current===s.session_id?'is-selected':''}`} title={`${s.title||tr('新会话','New conversation')}\n${s.attention?attentionLabel(s):status(s)} · ${new Date(s.updated_at).toLocaleString(tr('zh-CN','en-US'))}`} aria-current={current===s.session_id?'true':undefined} onClick={()=>choose(s.session_id)}><span className="edge-agent-session-title"><strong>{s.title||tr('新会话','New conversation')}</strong>{s.attention?<CircleHelp size={13} aria-label={attentionLabel(s)}/>:s.running?<LoaderCircle size={13} className="is-running" aria-label={tr('运行中','Running')}/>:null}{unread.isUnread(s)&&<span className="edge-agent-unread" role="img" aria-label={tr('有新回复','New reply')} title={tr('有新回复','New reply')}/>}</span></Button>{management&&<ActionMenu label={tr('会话操作','Conversation actions')} disabled={saving} items={[{label:tr('重命名','Rename'),onClick:()=>{setTitle(s.title);setOperationError('');setOperation({kind:'rename',row:s});}},{label:tr('删除','Delete'),danger:true,onClick:()=>{setOperationError('');setOperation({kind:'delete',row:s});}}]}/>}</div>)}{!query&&rows.length>8&&<Button variant="ghost" className="edge-agent-show-more" aria-expanded={showAll.has(path)} onClick={()=>setShowAll(previous=>{const next=new Set(previous);if(next.has(path))next.delete(path);else next.add(path);return next;})}>{showAll.has(path)?tr('收起','Show less'):tr('展开显示','Show more')}</Button>}</div>}
     </section>;})}
    </div>
    {persistent&&total>50&&<div className="edge-agent-history-pager"><Button disabled={page===1} onClick={()=>setPage(p=>p-1)}>{tr('上一页','Previous')}</Button><span>{page} / {Math.ceil(total/50)}</span><Button disabled={page*50>=total} onClick={()=>setPage(p=>p+1)}>{tr('下一页','Next')}</Button></div>}
   </aside>}
   {open&&resize.separator}
<div className="edge-agent-session-main">{selection&&!denied?<Workspace key={selection.key} access={access} initialSessionID={selection.id} initialDirectory={selection.directory} onNew={requestNew} onRead={unread.markRead} onSessionRemoved={id=>{setItems(rows=>rows.filter(row=>row.session_id!==id));setCurrent('');setParams(p=>{const next=new URLSearchParams(p);next.delete('session');return next;},{replace:true});setRefresh(v=>v+1);}} onSessionReady={id=>{setCurrent(id);setParams(p=>{const next=new URLSearchParams(p);next.set('session',id);return next;},{replace:true});setRefresh(v=>v+1);}}/>:loading?<Notice>{tr('正在加载会话…','Loading sessions…')}</Notice>:<Notice>{tr('选择一个会话，或新建会话开始。','Select a conversation or start a new one.')}</Notice>}</div>
  </div>
  {newDirectory!==undefined&&!denied&&<DirectoryBrowser creating edge={access.edge_id} accessID={access.id} current={newDirectory} onClose={()=>setNewDirectory(undefined)} onSelect={path=>{setNewDirectory(undefined);choose(undefined,path);}}/>}
  <Modal open={Boolean(operation)} title={operation?.kind==='rename'?tr('重命名会话','Rename conversation'):tr('删除会话','Delete conversation')} onClose={()=>{if(!saving)setOperation(undefined);}} footer={<><Button disabled={saving} onClick={()=>setOperation(undefined)}>{tr('取消','Cancel')}</Button><Button variant={operation?.kind==='delete'?'danger':'primary'} loading={saving} disabled={saving||(operation?.kind==='rename'&&!title.trim())} onClick={()=>void manage()}>{operation?.kind==='delete'?tr('删除','Delete'):tr('保存','Save')}</Button></>}>
   {operationError&&<Notice tone="danger">{operationError}</Notice>}
   {operation?.kind==='rename'?<Field label={tr('会话名称','Conversation name')}><Input value={title} maxLength={120} onChange={e=>setTitle(e.target.value)}/></Field>:<DangerConfirm title={operation?.row.title||tr('新会话','New conversation')} description={tr('将结束本次实例并删除 Liaison 中的会话记录，无法恢复。不会删除项目文件或 Codex 桌面端的其他会话。','Ends this instance and permanently removes its Liaison conversation. Project files and other Codex desktop conversations are unaffected.')}/>}
  </Modal>
 </div>;
}
