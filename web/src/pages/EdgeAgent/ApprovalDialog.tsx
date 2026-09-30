import {useId} from 'react';
import {ShieldCheck} from 'lucide-react';
import {Button,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import type {AgentSnapshot} from '@/services/edgeAgent';
import {DiffView} from './DiffView';

export function ApprovalDialog({approval,count,pending,error,onDecision}:{approval:NonNullable<AgentSnapshot['approvals']>[number];count:number;pending:boolean;error:string;onDecision:(decision:'accept'|'decline')=>void}){
 const {tr}=useI18n(),titleId=useId();
 const cancel=()=>{if(!pending)onDecision('decline');};
 return <section className="edge-agent-approval" aria-labelledby={titleId} aria-busy={pending}>
  <div className="edge-agent-approval-title" role="status"><ShieldCheck size={16}/><strong id={titleId}>{approval.kind==='fileChange'?tr('确认文件修改','Confirm file changes'):tr('确认执行命令','Confirm command execution')}</strong>{count>1&&<span>{tr('待确认','Pending')} · {count}</span>}</div>
  <div className="edge-agent-approval-body">
   {error&&<div role="alert"><Notice tone="danger">{error}</Notice></div>}
   <p className="edge-agent-approval-reason">{approval.reason||tr('Agent 请求你的授权。','Agent is requesting your authorization.')}</p>
   {approval.command&&<pre>{approval.command}</pre>}
   <details><summary>{tr('查看操作详情','View operation details')}</summary>
    <code>{approval.directory}</code>
    {approval.changes?.map((change,index)=><div key={index} className="edge-agent-approval-change"><code>{change.path}{change.move_path?` → ${change.move_path}`:''}</code><DiffView text={change.diff}/></div>)}
   </details>
  </div>
  <div className="edge-agent-approval-actions"><span>{tr('仅授权本次操作','Authorize this operation once')}</span><Button disabled={pending} onClick={cancel}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={pending} loading={pending} onClick={()=>onDecision('accept')}>{tr('确认','Confirm')}</Button></div>
 </section>;
}
