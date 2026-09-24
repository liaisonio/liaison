import {Check,ChevronRight,Clock,LoaderCircle,Terminal} from 'lucide-react';
import {useI18n} from '@/i18n';
import type {AgentActivity} from '@/services/edgeAgent';
import {DiffView} from './DiffView';
import {useState} from 'react';

export function Activity({items,activeTurn=false}:{items:AgentActivity[];activeTurn?:boolean}){
 const [expanded,setExpanded]=useState<boolean>();
 const {tr}=useI18n();if(!items.length)return null;
 const label=(kind:string)=>({commandExecution:tr('执行命令','Run command'),fileChange:tr('文件操作','File operation'),webSearch:tr('搜索网页','Search web'),mcpToolCall:tr('调用工具','Call tool'),dynamicToolCall:tr('调用工具','Call tool'),reasoning:tr('思考','Thinking'),contextCompaction:tr('整理上下文','Compact context'),userInput:tr('用户回答','User input'),turnDiff:tr('本轮累计变更','Turn changes')}[kind]||tr('处理任务','Process task'));
 const status=(s:string)=>({running:tr('进行中','In progress'),completed:tr('完成','Completed'),failed:tr('失败','Failed'),declined:tr('已拒绝','Declined'),ended:tr('已结束','Ended')}[s]||tr('已结束','Ended'));
 const running=items.some(i=>i.status==='running');
 return <details className="edge-agent-activity" open={expanded??(activeTurn||running)}><summary onClick={event=>{event.preventDefault();setExpanded(!(expanded??(activeTurn||running)));}}><ChevronRight size={13}/>{running?<LoaderCircle size={13} className="edge-agent-spinner"/>:<Terminal size={13}/>}<span>{tr('执行过程','Activity')}</span><small>{items.length}</small></summary><ol>{items.map(item=>{
  const detail=Boolean(item.command||item.output||item.changes?.length||item.exit_code!==undefined);
  const preview=item.command||item.changes?.map(change=>change.path).join(', ');
  const heading=<>{item.status==='running'?<LoaderCircle size={13} className="edge-agent-spinner"/>:item.status==='completed'?<Check size={13}/>:<Clock size={13}/>}<span>{label(item.kind)}</span>{preview&&<code className="edge-agent-step-preview" title={preview}>{preview}</code>}<small>{status(item.status)}{item.duration_ms>0?` · ${(item.duration_ms/1000).toFixed(1)}s`:''}</small></>;
  return <li key={item.id} className={detail?'has-detail':''}>{detail?<details><summary>{heading}<ChevronRight size={12}/></summary><div className="edge-agent-activity-detail">
   {item.directory&&<code className="edge-agent-command-directory">{item.directory}</code>}
   {item.command&&<pre aria-label={tr('命令','Command')}>{item.command}</pre>}
   {item.output&&(item.kind==='turnDiff'?<DiffView text={item.output}/>:<pre className="edge-agent-command-output" aria-label={tr('输出','Output')}>{item.output}</pre>)}
   {item.exit_code!==undefined&&<small>{tr('退出码','Exit code')} <code>{item.exit_code}</code></small>}
   {item.changes?.map((change,i)=><details key={i} className="edge-agent-file-change"><summary><ChevronRight size={12}/><code>{change.path}</code><small>{({add:tr('新增','Added'),delete:tr('删除','Deleted'),update:tr('修改','Modified')}[change.kind]||change.kind)}</small></summary>{change.move_path&&<code>→ {change.move_path}</code>}{change.diff?<DiffView text={change.diff}/>:<small>{tr('没有可显示的文本差异','No text diff available')}</small>}</details>)}
   {item.truncated&&<small>{tr('详情过长，已截断显示；任务仍会继续。','Details truncated; task execution continues.')}</small>}
  </div></details>:heading}</li>;
 })}</ol></details>;
}
