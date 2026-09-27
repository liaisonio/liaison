import {Fragment,memo,useCallback,useEffect,useLayoutEffect,useRef,useState,type ReactNode} from 'react';
import {ArrowDown} from 'lucide-react';
import {Button,Notice} from '@/components/ui';
import {MessageContent} from '@/components/AgentWorkspace/MessageContent';
import {useI18n} from '@/i18n';
import {edgeAgent,type AgentSnapshot} from '@/services/edgeAgent';
import {Activity} from './Activity';
import {FileAttachments} from './Files';
import {FileLinkContext} from '@/components/AgentWorkspace/FileLinkContext';
import {CodePreview} from './CodePreview';

const Round=memo(function Round({snapshot,live=false}:{snapshot:AgentSnapshot;live?:boolean}){
 const {tr}=useI18n();
 const running=live&&snapshot.running;
 return <>
  {snapshot.truncated&&<Notice>{tr('本轮内容较长，仅显示部分记录；不会因此中断任务。','This round is long; some display content is omitted. The task is not interrupted.')}</Notice>}
{snapshot.messages?.map((m,index)=><article key={index} className={`agent-message ${m.role==='user'?'is-user':''}${m.role==='assistant'&&running&&index===snapshot.messages!.length-1?' is-streaming':''}`}><span className="edge-agent-codex-label">{m.role==='user'?tr('你','You'):'Codex'}</span>{!!m.attachments?.length&&<FileAttachments files={m.attachments}/>}<MessageContent text={m.text||(m.role==='assistant'&&running&&!snapshot.activities?.some(a=>a.message_index===index)?tr('正在准备回复…','Preparing a response…'):'')}/><Activity activeTurn={running} items={snapshot.activities?.filter(a=>a.message_index===index)||[]}/></article>)}
 </>;
});

// Mounted per session: delayed responses cannot append another session's history.
export function Transcript({session,edge,accessID,children}:{session:AgentSnapshot;edge:number;accessID:string;children:ReactNode}){
 const {tr}=useI18n();
 const [preview,setPreview]=useState<string>();
 const openFile=useCallback((href:string)=>setPreview(href),[]);
 const [pages,setPages]=useState<AgentSnapshot[]>([]),[before,setBefore]=useState('');
 const [loading,setLoading]=useState(false),[failed,setFailed]=useState(false),[away,setAway]=useState(false);
 const container=useRef<HTMLDivElement>(null),stick=useRef(true),request=useRef<AbortController>();
 const anchor=useRef<{height:number;top:number}>();
 const retry=useRef<{cursor:string;initial:boolean}>();
 const previous=useRef(session);
 const window=session.window||0;
 useLayoutEffect(()=>{
  const last=previous.current;
  if((last.window||0)<window){
   setPages(items=>items.some(p=>(p.window||0)===(last.window||0))?items:[...items,{...last,running:false}].sort((a,b)=>(a.window||0)-(b.window||0)));
  }
  previous.current=session;
 },[session]);
 async function load(cursor:string,initial=false){
  request.current?.abort();
  const abort=new AbortController();request.current=abort;
  retry.current={cursor,initial};setLoading(true);setFailed(false);
  try{
   const result=await edgeAgent(edge,'transcript',{access_id:accessID,session_id:session.session_id,history_before:cursor,history_limit:initial?'19':'20'},abort.signal);
   if(abort.signal.aborted)return;
   if(result.status!=='ok'||result.session_id!==session.session_id)throw new Error('history unavailable');
   const incoming=result.history_pages||[];
   const el=container.current;
   if(el&&!stick.current)anchor.current={height:el.scrollHeight,top:el.scrollTop};
   setPages(previous=>{
    const merged=new Map(previous.map(p=>[p.window||0,p]));
    for(const page of incoming)merged.set(page.window||0,page);
    return [...merged.values()].sort((a,b)=>(a.window||0)-(b.window||0));
   });
   setBefore(previous=>pages.length&&(pages[0].window||0)<(incoming.at(-1)?.window??Number(cursor))?previous:result.history_before||'');
  }catch{if(!abort.signal.aborted)setFailed(true);}
  finally{if(!abort.signal.aborted)setLoading(false);}
 }
 useEffect(()=>{
  if(window)void load(String(window),true);
  return()=>request.current?.abort();
 },[window]);
 useLayoutEffect(()=>{
  const el=container.current;if(!el)return;
  if(stick.current)el.scrollTop=el.scrollHeight;
  else if(anchor.current)el.scrollTop=anchor.current.top+el.scrollHeight-anchor.current.height;
  anchor.current=undefined;
 },[pages,session.messages]);
 return <FileLinkContext.Provider value={openFile}>
  <div className="edge-agent-messages" ref={container} onScroll={()=>{const el=container.current;if(el){stick.current=el.scrollHeight-el.scrollTop-el.clientHeight<80;setAway(!stick.current);}}}>
   {(loading||failed||before)&&<nav className="edge-agent-history-nav" aria-label={tr('对话历史','Conversation history')}>
    {failed&&<span role="status">{tr('历史加载失败，当前对话仍可使用。','History could not load. You can still use this conversation.')}</span>}
    <Button variant="ghost" loading={loading} disabled={loading} onClick={()=>{const next=failed?retry.current:{cursor:before,initial:false};if(next)void load(next.cursor,next.initial);}}>{loading?tr('正在加载历史…','Loading history…'):failed?tr('重试','Retry'):tr('加载更早记录','Load earlier messages')}</Button>
   </nav>}
   {pages.filter(p=>(p.window||0)<window).map((p,index)=><Fragment key={p.window||0}>
    {index>0&&(p.window||0)>(pages[index-1].window||0)+1&&<div className="edge-agent-history-nav"><Button variant="ghost" disabled={loading} onClick={()=>void load(String(p.window))}>{tr('加载中间记录','Load missing messages')}</Button></div>}
    <Round snapshot={p}/>
   </Fragment>)}
   {!session.messages?.length&&!pages.length&&children}
   <Round key={window} snapshot={session} live/>
  </div>
  {away&&!session.approvals?.length&&!session.input_requests?.length&&<Button className="edge-agent-jump" aria-label={tr('回到最新消息','Jump to latest')} onClick={()=>{stick.current=true;setAway(false);container.current?.scrollTo({top:container.current.scrollHeight,behavior:'smooth'});}}><ArrowDown size={15}/></Button>}
  {preview!==undefined&&session.session_id&&<CodePreview key={preview} href={preview} project={session.project||''} edge={edge} access={accessID} session={session.session_id} available={Boolean(session.files_available)} onClose={()=>setPreview(undefined)}/>}
 </FileLinkContext.Provider>;
}
