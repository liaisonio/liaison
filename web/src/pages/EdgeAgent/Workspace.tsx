import {useEffect,useRef,useState} from 'react';
import {Link} from 'react-router-dom';
import {ArrowDown,ArrowUp,ChevronRight,Folder,Info,Plus,ShieldCheck,Square,Terminal,X} from 'lucide-react';
import {Button,DangerConfirm,Modal,Notice,StatusPill} from '@/components/ui';
import ActionMenu from '@/components/ui/ActionMenu';
import {MessageContent} from '@/components/AgentWorkspace/MessageContent';
import {useI18n} from '@/i18n';
import {RequestError} from '@/api/client';
import {agentConnectors,edgeAgent as callAgent,type AgentAccess,type AgentSkill,type AgentConnector,type AgentSnapshot} from '@/services/edgeAgent';
import {SessionDetails} from './SessionDetails';
import {SlashMenu} from './SlashMenu';
import {DirectoryBrowser} from './DirectoryBrowser';
import {Activity} from './Activity';
import {ModelPicker} from './ModelPicker';
import {InputRequest} from './InputRequest';
import '@/components/AgentWorkspace/index.less';
import './index.less';
import './interaction.less';

export default function Workspace({access,initialSessionID,initialDirectory,onSessionReady,onSessionRemoved}:{access:AgentAccess;initialSessionID?:string;initialDirectory?:string;onSessionReady?:(id:string)=>void;onSessionRemoved?:(id:string)=>void}){
  const edgeAgent=(edge:number,action:string,fields:Record<string,unknown>={},signal?:AbortSignal)=>callAgent(edge,action,{...fields,access_id:access.id},signal);
  const {tr}=useI18n();
  const [connectors,setConnectors]=useState<AgentConnector[]>([]),[loading,setLoading]=useState(true);
  const edge=String(access.edge_id),installation=access.installation_id,project=access.project;
  const [details,setDetails]=useState(false),[skill,setSkill]=useState<AgentSkill>(),[commandsOpen,setCommandsOpen]=useState(false);
  const started=useRef(false);
  const retiring=useRef('');
  const [browsing,setBrowsing]=useState(false),[nextDirectory,setNextDirectory]=useState('');
  const workingDirectory=useRef(initialDirectory||access.project),revision=useRef(0);
  const [pending,setPending]=useState(false),[error,setError]=useState('');
  const [session,setSession]=useState<AgentSnapshot>(),[text,setText]=useState(''),[confirm,setConfirm]=useState(false),[pollFailed,setPollFailed]=useState(false);
  const live=useRef<{edge:number;id:string}>(),mounted=useRef(true),sequence=useRef(0),applied=useRef(0),busy=useRef(false);
  const input=useRef<HTMLTextAreaElement>(null),messages=useRef<HTMLDivElement>(null),stick=useRef(true);
  const [away,setAway]=useState(false);
  const [historyView,setHistoryView]=useState<AgentSnapshot>(),[historyLoading,setHistoryLoading]=useState(false);
  const shown=historyView||session;
  const historySequence=useRef(0);
  useEffect(()=>{historySequence.current++;setHistoryView(undefined);setHistoryLoading(false);},[session?.session_id]);
  const loadHistory=async(before:number)=>{
    if(!session?.session_id)return;
    const id=++historySequence.current;
    setHistoryLoading(true);
    try{
      const next=await edgeAgent(Number(edge),'transcript',{session_id:session.session_id,history_before:String(before)});
      if(!mounted.current||id!==historySequence.current)return;
      if(next.session_id){setHistoryView(next);stick.current=false;messages.current?.scrollTo({top:0});}
      else setError(tr('没有更早的记录。','No earlier history available.'));
    }catch{if(mounted.current&&id===historySequence.current)setError(tr('历史加载失败，请重试。','Could not load history. Try again.'));}
    finally{if(mounted.current&&id===historySequence.current)setHistoryLoading(false);}
  };
  const [modelPicker,setModelPicker]=useState(false);
  const active=Boolean(session?.session_id&&!session.closed);
  const resumable=Boolean(session?.session_id&&session.closed&&session.history_persistent&&session.messages?.length);
  const emptySession=Boolean(session?.session_id&&!session.running&&!session.messages?.length&&!session.activities?.length);
  const selected=connectors.find(c=>String(c.id)===edge);
  const statusText=(status:string)=>({
    busy:tr('Agent 正忙，请稍后重试。','Agent is busy. Try again shortly.'),
    unavailable:tr('无法连接 Agent，请检查连接器是否在线。','Cannot reach the Agent. Check the connector.'),
    upgrade_required:tr('请先升级此连接器，当前版本不支持 Agent。','Upgrade this connector to enable Agent access.'),
    not_found:tr('会话或安装已不可用，请重新发现并连接。','Session or installation unavailable. Discover and connect again.'),
    unsupported_user:tr('当前 Edge 运行账号或平台不支持启动 Agent。请使用 macOS 或 Linux 的普通用户实例。','This Edge account or platform cannot launch Agents. Use a non-root macOS or Linux instance.'),
    launch_failed:tr('启动失败。请确认此设备已安装 Codex，且项目目录可访问。','Could not start Codex. Check its installation and project directory.'),
    external_tools:tr('此 Codex 配置启用了外部工具，暂不支持只读预览。未发送模型请求，也未修改本机配置。','This Codex configuration enables external tools, which are not supported in read-only preview. No model request was sent or local configuration changed.'),
    authentication_required:tr('请先在连接器所在设备登录 Codex 或配置其本地 API 凭据。','Sign in to Codex or configure its API credentials on the connector device first.'),
    session_closed:tr('会话已结束。','Session ended.'),
    turn_failed:tr('本次回复未完成，不会自动重试。','The response did not complete. It will not retry automatically.'),
    approval_declined:tr('此操作的授权类型暂不支持，已安全拒绝。','This approval type is not supported and was safely declined.'),
    output_limit:tr('已达到本次会话的输出上限，请新建会话。','Session output limit reached. Start a new session.'),
    expired:tr('会话已超时并关闭，请重新连接。','Session timed out and closed. Connect again.'),
    invalid_request:tr('请检查项目路径和输入内容。','Check the project path and input.'),
    resume_unavailable:tr('无法恢复此会话。它可能由旧版连接器创建、本机记录已删除，或项目与 Codex 安装已改变。历史仍可查看。','This conversation cannot be resumed. It may come from an older connector, its local record may be missing, or the project or Codex installation may have changed. Its history remains available.'),
  }[status]||tr('操作失败，请重试。','The operation failed. Try again.'));

  useEffect(()=>{
    mounted.current=true;
    const abort=new AbortController();
    void agentConnectors(abort.signal).then(items=>{setConnectors(items);}).catch(e=>{
      if(e.name!=='AbortError')setError(tr('无法加载自己的连接器，请检查权限或重试。','Cannot load your connectors. Check permissions or retry.'));
    }).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
    return()=>{mounted.current=false;abort.abort();};
  },[]);

  const apply=(next:AgentSnapshot,id:number)=>{
    if(!mounted.current||id<applied.current)return;
    applied.current=id;revision.current=next.revision||0;setSession(next);
    if(next.closed)live.current=undefined;
  };
  useEffect(()=>{
    if(!active||!session?.session_id)return;
    let stopped=false,timer:ReturnType<typeof setTimeout>;
    const watch=session.revision!==undefined;
    const abort=new AbortController();
    const poll=async()=>{
      if(stopped)return;
      let delay=watch?60:1000;
      if(!busy.current){
        const id=++sequence.current;
        try{
          const next=await edgeAgent(Number(edge),watch?'watch':'poll',{session_id:session.session_id!,...(watch?{cursor:String(revision.current)}:{})},abort.signal);
          if(stopped)return;
          if(retiring.current===session.session_id){timer=setTimeout(poll,60);return;}
          if(!next.session_id){setError(statusText(next.status));live.current=undefined;setSession(previous=>previous?{...previous,closed:true,running:false}:previous);return;}
          setPollFailed(false);apply(next,id);
        }catch(e){
          if(retiring.current===session.session_id){if(!stopped)timer=setTimeout(poll,60);return;}
          if(!stopped&&e instanceof RequestError&&[401,403,404].includes(e.response?.status||0)){
            live.current=undefined;setSession(previous=>previous?{...previous,closed:true,running:false}:previous);
            setError(tr('会话访问权限已失效，请检查权限后重新连接。','Session access is no longer available. Check permissions and reconnect.'));return;
          }
          if(!stopped&&(e as Error).name!=='AbortError'){setPollFailed(true);delay=1500;}
        }
      }
      if(!stopped)timer=setTimeout(poll,delay);
    };
    timer=setTimeout(poll,0);
    return()=>{stopped=true;clearTimeout(timer);abort.abort();};
  },[active,edge,session?.session_id]);
  useEffect(()=>{if(stick.current&&messages.current)messages.current.scrollTop=messages.current.scrollHeight;},[session?.messages]);
  useEffect(()=>{const el=input.current;if(el){el.style.height='auto';el.style.height=`${Math.min(el.scrollHeight,160)}px`;}},[text,session?.session_id]);
  useEffect(()=>{if(active)input.current?.focus();},[active]);
  useEffect(()=>{if(!loading&&(selected?.online||initialSessionID)&&!started.current){started.current=true;if(initialSessionID)void resume(initialSessionID);else void act('start');}},[loading,selected?.online]);


  async function resume(sessionID:string){
    if(busy.current)return;
    busy.current=true;setPending(true);const id=++sequence.current;
    try{
      const next=await edgeAgent(Number(edge),'poll',{session_id:sessionID});
      if(!mounted.current)return;
      if(!next.session_id){setError(statusText(next.status));return;}
      workingDirectory.current=next.project||project;apply(next,id);onSessionReady?.(next.session_id);
    }catch{if(mounted.current)setError(tr('无法加载会话，请重试。','Could not load the session. Try again.'));}
    finally{busy.current=false;if(mounted.current)setPending(false);}
  }

  async function permissionAction(action:'approve'|'permissions'|'answer',fields:Record<string,unknown>){
    if(busy.current||!session?.session_id||!active)return;
    busy.current=true;setPending(true);setError('');const id=++sequence.current;
    try{
      const next=await edgeAgent(Number(edge),action,{session_id:session.session_id,...fields});
      if(!mounted.current)return;
      if(!next.session_id){setError(statusText(next.status));return;}
      apply(next,id);
    }catch{if(mounted.current)setError(tr('操作结果未确认，请等待状态刷新，不会自动重试。','Operation outcome is unconfirmed. Wait for a status update; it will not retry automatically.'));}
    finally{busy.current=false;if(mounted.current)setPending(false);}
  }

  async function resumeNative(){
    if(busy.current||!session?.session_id)return;
    busy.current=true;setPending(true);setError('');const id=++sequence.current;
    try{
      const next=await edgeAgent(Number(edge),'resume',{session_id:session.session_id});
      if(!mounted.current)return;
      if(!next.session_id){setError(statusText(next.status));return;}
      workingDirectory.current=next.project||project;live.current={edge:Number(edge),id:next.session_id};apply(next,id);onSessionReady?.(next.session_id);input.current?.focus();
    }catch{if(mounted.current)setError(tr('恢复结果未确认，请刷新会话状态后重试。','The resume result is unconfirmed. Refresh the conversation state and retry.'));}
    finally{busy.current=false;if(mounted.current)setPending(false);}
  }

  async function act(action:'start'|'send'|'interrupt'|'stop',directory?:string){
    if(busy.current)return;
    if(action==='send'&&/^\/[\w-]*$/.test(text.trim())){setError(tr('请从 / 菜单选择命令或技能。','Choose a command or skill from the / menu.'));return;}
    busy.current=true;setPending(true);setError('');
    const id=++sequence.current;
    const target=directory||workingDirectory.current;
    const fields:Record<string,string>=action==='start'?{installation_id:installation,project:project.trim(),...(target!==project?{working_directory:target}:{})}:{session_id:session?.session_id||'',...(action==='send'?{text:text.trim(),...(skill?{skill_id:skill.id}:{})}:{})};
    try{
      if(action==='start'&&directory&&emptySession&&session?.session_id){
        const previous=session.session_id;retiring.current=previous;
        const discarded=await edgeAgent(Number(edge),'discard',{session_id:previous});
        if(discarded.status!=='ok'){
          retiring.current='';
          setError(discarded.status==='busy'?tr('原会话已有内容或正在运行，已保留。请刷新后重试。','The original conversation has content or is running and was retained. Refresh and retry.'):tr('未能清理空会话，请检查连接器版本或重试。','Could not remove the empty conversation. Check the connector version or retry.'));
          return;
        }
        if(!mounted.current)return;
        applied.current=id;live.current=undefined;workingDirectory.current=target;setSession(undefined);onSessionRemoved?.(previous);
      }
      const next=await edgeAgent(Number(edge),action,fields);
      if(!mounted.current)return;
      if(!next.session_id){if(action==='stop'){setNextDirectory('');}setError(statusText(next.status));return;}
      if(action==='start'&&!next.closed)live.current={edge:Number(edge),id:next.session_id};
      if(action==='start'){workingDirectory.current=next.project||target;setNextDirectory('');if(!directory)setText('');setSkill(undefined);onSessionReady?.(next.session_id);}
      apply(next,id);setPollFailed(false);
      if(action==='send'&&next.status==='ok'){historySequence.current++;setHistoryView(undefined);setHistoryLoading(false);setText('');setSkill(undefined);input.current?.focus();}
    }catch{retiring.current='';if(action==='stop'){setNextDirectory('');}if(mounted.current)setError(tr('操作结果未确认，请等待状态刷新，不会自动重试。','Operation outcome is unconfirmed. Wait for a status update; it will not retry automatically.'));}
    finally{busy.current=false;if(mounted.current){setPending(false);setConfirm(false);}}
  }

  return <div className="edge-agent-page edge-agent-session-page">
    {error&&<div role="alert"><Notice tone="danger">{error}</Notice></div>}
    {session?.archived&&<Notice>{tr('正在查看服务端保存的历史。若原设备上的 Codex 会话仍在，可以恢复续聊。','Viewing server history. You can resume if the native Codex conversation is still available on the original device.')}</Notice>}
    {loading?<Notice>{tr('正在加载连接器…','Loading connectors…')}</Notice>:!connectors.length?<Notice>{tr('暂无可用的自有连接器。','No owned connectors available.')} <Link to="/connector">{tr('创建连接器','Create connector')}</Link></Notice>:null}
    {!active&&!loading&&<section className="edge-agent-setup"><div className="edge-agent-start"><span>{selected?.name} · <code>{session?.project||project}</code></span><div>{resumable&&<Button disabled={pending||!selected?.online} loading={pending} onClick={()=>void resumeNative()}>{tr('恢复会话','Resume conversation')}</Button>}<Button variant="primary" disabled={pending||!selected?.online} loading={pending&&!resumable} onClick={()=>void act('start')}>{session?.session_id?tr('新建会话','New conversation'):tr('开始会话','Start session')}</Button></div></div></section>}
    {session?.session_id&&<section className={`edge-agent-workspace${active&&session.input_requests?.length?' has-input':''}`}>
      <header><div className="edge-agent-project"><Button variant="ghost" aria-label={tr('选择工作目录','Choose working directory')} title={tr('选择工作目录','Choose working directory')} disabled={pending} onClick={()=>setBrowsing(true)}><Folder size={16}/></Button><div><strong title={session.project||project}>{(session.project||project).split(/[\\/]/).filter(Boolean).pop()||project}</strong><span title={selected?.name}>{selected?.device||selected?.name}</span></div></div><div className="edge-agent-session-actions"><StatusPill tone={active?'success':'neutral'}>{active?session.input_requests?.some(r=>r.blocking)?tr('等待回答','Waiting for input'):session.approvals?.length?tr('等待审批','Waiting for approval'):session.running?tr('正在回复','Responding'):tr('已连接','Connected'):tr('已结束','Ended')}</StatusPill><Button variant="ghost" title={tr('会话详情','Session details')} aria-label={tr('会话详情','Session details')} onClick={()=>setDetails(true)}><Info size={16}/></Button>{active&&<><Button variant="primary" title={tr('新建会话','New conversation')} aria-label={tr('新建会话','New conversation')} disabled={pending} onClick={()=>{void act('start');}}><Plus size={16}/>{tr('新建会话','New conversation')}</Button><Button variant="secondary" title={tr('结束会话','End session')} aria-label={tr('结束会话','End session')} disabled={pending} onClick={()=>{setConfirm(true);}}><Square size={14}/>{tr('结束会话','End session')}</Button></>}</div></header>
      {pollFailed&&<Notice tone="warning">{tr('连接暂时中断，正在重连。不会重发未确认的操作；切换页面不会中断任务。','Connection interrupted. Reconnecting without replaying operations. Switching pages does not stop a running task.')}</Notice>}
      {session.status!=='ok'&&<Notice tone={session.closed?'info':'warning'}>{statusText(session.status)}</Notice>}
      {Boolean(session.window)&&<nav className="edge-agent-history-nav" aria-label={tr('对话历史','Conversation history')}>
        <Button disabled={historyLoading||!(shown?.window)} loading={historyLoading} onClick={()=>void loadHistory(shown?.window||0)}>{tr('更早一轮','Previous round')}</Button>
        {historyView&&<><span>{tr('历史记录','History')} · {(historyView.window||0)+1}</span><Button disabled={historyLoading} onClick={()=>{if((historyView.window||0)+1>=(session.window||0)){setHistoryView(undefined);}else void loadHistory((historyView.window||0)+2);}}>{tr('较新一轮','Next round')}</Button><Button onClick={()=>{historySequence.current++;setHistoryLoading(false);setHistoryView(undefined);stick.current=true;}}>{tr('返回最新','Back to latest')}</Button></>}
      </nav>}
      {shown?.truncated&&<Notice>{tr('本轮内容较长，仅显示部分记录；不会因此中断任务。','This round is long; some display content is omitted. The task is not interrupted.')}</Notice>}
      <div className="edge-agent-messages" ref={messages} onScroll={()=>{const el=messages.current;if(el){stick.current=el.scrollHeight-el.scrollTop-el.clientHeight<80;setAway(!stick.current);}}}>
        {!session.messages?.length&&<div className="edge-agent-empty"><div className="edge-agent-empty-icon"><Terminal size={24}/></div><strong>{tr('从这个项目开始','Start with this project')}</strong><p>{tr('Codex 已就绪。想先了解什么？','Codex is ready. What would you like to explore?')}</p><div className="edge-agent-starters">{[tr('梳理项目结构','Explore the codebase'),tr('解释核心流程','Explain the core flow'),tr('检查潜在问题','Review potential issues')].map(prompt=><Button key={prompt} disabled={!active||pending} onClick={()=>{setText(prompt);input.current?.focus();}}>{prompt}<ChevronRight size={13}/></Button>)}</div></div>}
        {shown?.messages?.map((m,index)=><article key={`${shown?.window||0}:${index}`} className={`agent-message ${m.role==='user'?'is-user':''}${m.role==='assistant'&&(!historyView&&session.running)&&index===shown?.messages!.length-1?' is-streaming':''}`}><span>{m.role==='user'?tr('你','You'):'Codex'}</span><MessageContent text={m.text||((!historyView&&session.running)&&!shown?.activities?.some(a=>a.message_index===index)?tr('正在准备回复…','Preparing a response…'):'')}/><Activity activeTurn={!historyView&&session.running} items={shown?.activities?.filter(a=>a.message_index===index)||[]}/></article>)}
      </div>
      {away&&<Button className="edge-agent-jump" aria-label={tr('回到最新消息','Jump to latest')} onClick={()=>{stick.current=true;setAway(false);messages.current?.scrollTo({top:messages.current.scrollHeight,behavior:'smooth'});}}><ArrowDown size={15}/></Button>}
      {active&&Boolean(session.input_requests?.length)&&<section className="edge-agent-input-requests" aria-label={tr('待回答的问题','Pending questions')}>
        {session.input_requests?.map(request=><InputRequest key={request.id} request={request} disabled={pending} onAnswer={answers=>void permissionAction('answer',{input_id:request.id,answers})}/>)}
      </section>}
      {active&&Boolean(session.approvals?.length)&&<section className="edge-agent-approvals" aria-label={tr('等待审批','Waiting for approval')}>
        {session.approvals?.map(approval=><div key={approval.id} className="edge-agent-approval">
          <strong>{tr('允许执行此命令？','Allow this command?')}</strong>
          <p>{tr('此命令需要越出当前沙箱。请检查命令与目录，仅授权本次执行。','This command needs to run beyond the current sandbox. Review the command and directory; approval applies once.')}</p>
          {approval.reason&&<p>{approval.reason}</p>}
          <code>{approval.directory}</code><pre>{approval.command}</pre>
          <div><Button disabled={pending} onClick={()=>void permissionAction('approve',{approval_id:approval.id,decision:'decline'})}>{tr('拒绝','Decline')}</Button><Button variant="primary" disabled={pending} onClick={()=>void permissionAction('approve',{approval_id:approval.id,decision:'accept'})}>{tr('允许一次','Allow once')}</Button></div>
        </div>)}
      </section>}
      <form className="edge-agent-composer" onSubmit={e=>{e.preventDefault();if(active&&!session.running&&!pending&&text.trim())void act('send');}}>
        {skill&&<div className="edge-agent-selected-skill"><span>{skill.name}</span><Button variant="ghost" aria-label={tr('移除技能','Remove skill')} onClick={()=>{setSkill(undefined);input.current?.focus();}}><X size={13}/></Button></div>}
        <SlashMenu text={text} setText={setText} input={input} skills={session.skills||[]} available={Boolean(session.skills_available)} disabled={!active||pending||session.running} onSkill={setSkill} onStatus={()=>setDetails(true)} onNew={()=>{void act('start');}} forcedOpen={commandsOpen} onClose={()=>setCommandsOpen(false)}/>
<textarea ref={input} rows={2} aria-label={tr('消息','Message')} value={text} disabled={!active} readOnly={pending} maxLength={16000} onChange={e=>setText(e.target.value)} placeholder={active?tr('询问当前项目，输入 / 选择技能','Ask about this project, or type / for skills'):tr('会话已结束','Session ended')} onKeyDown={e=>{if(e.key==='Enter'&&!e.shiftKey&&!e.nativeEvent.isComposing){e.preventDefault();if(active&&!session.running&&!pending&&text.trim())void act('send');}}}/>
        <div className="edge-agent-composer-tools"><ActionMenu label={tr('添加与工具','Add and tools')} icon={<Plus size={16}/>} placement="top" disabled={!active||pending} items={[
          {label:tr('选择工作目录','Choose working directory'),onClick:()=>{setCommandsOpen(false);setBrowsing(true);}},
          {label:tr('命令与技能','Commands and skills'),disabled:session.running,onClick:()=>{setCommandsOpen(true);input.current?.focus();}},
          {label:tr('会话详情','Session details'),onClick:()=>setDetails(true)},
        ]}/><Button variant="ghost" className="edge-agent-model" aria-label={tr('切换模型','Switch model')} disabled={!active||pending||session.running} onClick={()=>setModelPicker(true)} title={tr('模型来自 Codex 本机配置','Model from Codex local configuration')}><Terminal size={14}/><code>{session.model||tr('Codex 本机配置','Codex local configuration')}</code><ChevronRight size={12}/></Button>{session.running?<Button disabled={pending} aria-label={tr('停止','Stop')} title={tr('停止','Stop')} onClick={()=>void act('interrupt')}><Square size={14}/></Button>:<Button type="submit" variant="primary" aria-label={tr('发送','Send')} title={tr('发送','Send')} disabled={!active||pending||!text.trim()}><ArrowUp size={16}/></Button>}</div>
      </form>
<footer className="edge-agent-footnote"><span><ShieldCheck size={12}/>{active&&session.permissions_available?<ActionMenu icon={<span>{session.permission_mode==='workspace-write'?tr('工作区写入','Workspace write'):tr('只读','Read-only')}</span>} label={session.permission_mode==='workspace-write'?tr('工作区写入','Workspace write'):tr('只读','Read-only')} disabled={pending||session.running} placement="top" items={[
  {label:tr('只读','Read-only'),onClick:()=>void permissionAction('permissions',{permission_mode:'read-only'})},
  {label:tr('允许工作区写入','Allow workspace writes'),onClick:()=>void permissionAction('permissions',{permission_mode:'workspace-write'})},
]}/>:<span>{session.permission_mode==='workspace-write'?tr('工作区写入','Workspace write'):tr('只读','Read-only')}</span>}{session.history_persistent?tr('历史保存在服务端','History stored on the server'):tr('临时历史','Temporary history')}</span><span className="edge-agent-keyhint">{tr('Shift + Enter 换行','Shift + Enter for newline')}</span></footer>
    </section>}
    <Modal open={confirm} title={nextDirectory?tr('切换工作目录','Switch working directory'):tr('结束会话','End session')} onClose={()=>{if(!pending){setNextDirectory('');setConfirm(false);}}} footer={<><Button disabled={pending} onClick={()=>{setNextDirectory('');setConfirm(false);}}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={pending} loading={pending} onClick={()=>{if(nextDirectory)void act('start',nextDirectory);else void act('stop');}}>{nextDirectory?tr('新建会话','New conversation'):tr('结束会话','End session')}</Button></>}><DangerConfirm title={nextDirectory?tr('在新目录开始会话？','Start a conversation in the new folder?'):tr('关闭本次 Codex 实例？','Close this Codex instance?')} description={nextDirectory?(emptySession?tr('当前空会话将移除，其他会话不受影响。','The current empty conversation will be removed. Other conversations are unaffected.'):tr('当前会话已有内容，将予以保留。','The current conversation has content and will be retained.')):tr('不会关闭其他会话。已结束的会话可在列表中查看，不能继续发送消息。','Other sessions will not be closed. Ended conversations remain viewable in the list, but cannot receive messages.')}/>{nextDirectory&&<code className="edge-agent-switch-path">{nextDirectory}</code>}</Modal>
{browsing&&<DirectoryBrowser edge={Number(edge)} accessID={access.id} current={session?.project||workingDirectory.current} switching={active} onClose={()=>setBrowsing(false)} onSelect={path=>{setBrowsing(false);if(path===(session?.project||workingDirectory.current))return;if(active){setNextDirectory(path);setConfirm(true);}else void act('start',path);}}/>}
    <SessionDetails open={details} onClose={()=>setDetails(false)} session={session} connector={selected}/>
    {modelPicker&&session&&<ModelPicker access={access} session={session} onClose={()=>setModelPicker(false)} onChange={next=>apply(next,++sequence.current)}/>}
  </div>;
}
