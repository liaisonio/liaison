import {Check,ChevronRight,Clock,XCircle,LoaderCircle,Terminal} from 'lucide-react';
import {useI18n} from '@/i18n';
import type {AgentActivity} from '@/services/edgeAgent';
import {DiffView} from './DiffView';
import {useState,type ReactNode} from 'react';
import {MessageContent} from '@/components/AgentWorkspace/MessageContent';

// Polling never overrides an explicit user expansion/collapse choice.
function LiveDetail({active,heading,children}:{active:boolean;heading:ReactNode;children:ReactNode}){
 const [expanded,setExpanded]=useState<boolean>();
 return <details open={expanded??active}><summary onClick={e=>{e.preventDefault();setExpanded(!(expanded??active));}}><ChevronRight size={12} className="edge-agent-disclosure"/>{heading}</summary>{children}</details>;
}

export function Activity({items,activeTurn=false,awaitingCommands=[]}:{items:AgentActivity[];activeTurn?:boolean;awaitingCommands?:string[]}){
 const [expanded,setExpanded]=useState<boolean>();
 const {tr}=useI18n();if(!items.length)return null;
 const label=(kind:string)=>({commandExecution:tr('执行命令','Run command'),fileChange:tr('文件操作','File operation'),webSearch:tr('搜索网页','Search web'),mcpToolCall:tr('调用工具','Call tool'),dynamicToolCall:tr('调用工具','Call tool'),reasoning:tr('思考','Thinking'),contextCompaction:tr('整理上下文','Compact context'),userInput:tr('用户回答','User input'),turnDiff:tr('本轮累计变更','Turn changes'),plan:tr('方案','Proposed plan'),imageView:tr('查看图片','View image'),enteredReviewMode:tr('开始审查','Start review'),exitedReviewMode:tr('结束审查','Finish review')}[kind]||tr('处理任务','Process task'));
const status=(s:string)=>({running:tr('进行中','In progress'),completed:tr('完成','Completed'),failed:tr('失败','Failed'),declined:tr('已拒绝','Declined'),cancelled:tr('已取消','Cancelled'),ended:tr('已结束','Ended')}[s]||tr('已结束','Ended'));
 const steps=items.filter(i=>i.kind!=='turnPlan'),plans=items.filter(i=>i.kind==='turnPlan');
 const running=activeTurn&&steps.some(i=>i.status==='running');
 return <>{plans.map(item=><section className="edge-agent-plan" key={item.id} aria-label={tr('任务计划','Task plan')}><header><strong>{tr('任务计划','Task plan')}</strong><small>{item.plan?.filter(s=>s.status==='completed').length||0} / {item.plan?.length||0}</small></header>{item.output&&<p>{item.output}</p>}<ol>{item.plan?.map((step,index)=><li key={index} className={step.status==='completed'?'is-completed':''}>{step.status==='completed'?<Check size={14}/>:step.status==='inProgress'&&activeTurn?<LoaderCircle size={14} className="edge-agent-spinner"/>:<Clock size={14}/>}<span>{step.step}</span><small>{step.status==='completed'?tr('已完成','Completed'):step.status==='inProgress'&&activeTurn?tr('进行中','In progress'):tr('未完成','Not completed')}</small></li>)}</ol>{item.truncated&&<small>{tr('部分计划内容已省略。','Some plan content is omitted.')}</small>}</section>)}{steps.length>0&&<details className="edge-agent-activity" open={expanded??(activeTurn||running)}><summary onClick={event=>{event.preventDefault();setExpanded(!(expanded??(activeTurn||running)));}}><ChevronRight size={13} className="edge-agent-disclosure"/>{running?<LoaderCircle size={13} className="edge-agent-spinner"/>:<Terminal size={13}/>}<span>{tr('执行过程','Activity')}</span><small>{steps.length}</small></summary><ol>{steps.map(item=>{
  const waiting=activeTurn&&item.status==='running'&&Boolean(item.command&&awaitingCommands.includes(item.command));
  const detail=Boolean(item.command||item.output||item.changes?.length||item.exit_code!==undefined);
  const preview=item.label||item.command||item.changes?.map(change=>change.path).join(', ');
const heading=<>{item.status==='running'&&activeTurn&&!waiting?<LoaderCircle size={13} className="edge-agent-spinner"/>:item.status==='completed'?<Check size={13}/>:item.status==='failed'||item.status==='declined'?<XCircle size={13}/>:<Clock size={13}/>}<span>{label(item.kind)}</span>{preview&&<code className="edge-agent-step-preview" title={preview}>{preview}</code>}<small>{waiting?tr('等待确认','Awaiting approval'):status(item.status)}{item.duration_ms>0?` · ${(item.duration_ms/1000).toFixed(1)}s`:''}</small></>;
return <li key={item.id} className={detail?'has-detail':''}>{detail?<LiveDetail active={activeTurn&&item.status==='running'&&!waiting} heading={heading}><div className="edge-agent-activity-detail">
   {item.directory&&<code className="edge-agent-command-directory">{item.directory}</code>}
   {item.command&&<pre aria-label={tr('命令','Command')}>{item.command}</pre>}
   {item.output&&(item.kind==='turnDiff'?<DiffView text={item.output}/>:item.kind==='plan'?<MessageContent text={item.output}/>:<pre className="edge-agent-command-output" aria-label={tr('输出','Output')}>{item.output}</pre>)}
   {item.exit_code!==undefined&&<small>{tr('退出码','Exit code')} <code>{item.exit_code}</code></small>}
   {item.changes?.map((change,i)=><details key={i} className="edge-agent-file-change"><summary><ChevronRight size={12} className="edge-agent-disclosure"/><code>{change.path}</code><small>{({add:tr('新增','Added'),delete:tr('删除','Deleted'),update:tr('修改','Modified')}[change.kind]||change.kind)}</small></summary>{change.move_path&&<code>→ {change.move_path}</code>}{change.diff?<DiffView text={change.diff}/>:<small>{tr('没有可显示的文本差异','No text diff available')}</small>}</details>)}
   {item.truncated&&<small>{tr('详情过长，已截断显示；任务仍会继续。','Details truncated; task execution continues.')}</small>}
</div></LiveDetail>:heading}</li>;
 })}</ol></details>}</>;
}
