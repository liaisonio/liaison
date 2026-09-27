import {useEffect,useRef} from 'react';
import {createPortal} from 'react-dom';
import {Button,Modal,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import type {AgentSnapshot} from '@/services/edgeAgent';
import {DiffView} from './DiffView';

export function ApprovalDialog({approval,count,pending,error,onDecision}:{approval:NonNullable<AgentSnapshot['approvals']>[number];count:number;pending:boolean;error:string;onDecision:(decision:'accept'|'decline')=>void}){
 const {tr}=useI18n(),body=useRef<HTMLDivElement>(null);
 useEffect(()=>{
  const previous=document.activeElement as HTMLElement|null;
  const dialog=body.current?.closest<HTMLElement>('[role="dialog"]');
  dialog?.setAttribute('aria-label',tr('授权确认','Confirm authorization'));
  dialog?.querySelector<HTMLButtonElement>('footer button')?.focus({preventScroll:true});
  const trap=(event:KeyboardEvent)=>{
   if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();dialog?.querySelector<HTMLButtonElement>('header button')?.click();return;}
   if(event.key!=='Tab'||!dialog)return;
   const nodes=Array.from(dialog.querySelectorAll<HTMLElement>('button:not(:disabled),summary,[tabindex="0"]')).filter(n=>n.getClientRects().length);
   const first=nodes[0],last=nodes[nodes.length-1];
   if(!first){event.preventDefault();return;}
   if(event.shiftKey&&(document.activeElement===first||!dialog.contains(document.activeElement))){event.preventDefault();last.focus({preventScroll:true});}
   else if(!event.shiftKey&&(document.activeElement===last||!dialog.contains(document.activeElement))){event.preventDefault();first.focus({preventScroll:true});}
  };
  document.addEventListener('keydown',trap,true);
  return()=>{document.removeEventListener('keydown',trap,true);if(previous?.isConnected)previous.focus({preventScroll:true});};
 },[]);
 const cancel=()=>{if(!pending)onDecision('decline');};
 return createPortal(<Modal open width={520} className="edge-agent-approval" closeOnMask={false} title={approval.kind==='fileChange'?tr('确认文件修改','Confirm file changes'):tr('确认执行命令','Confirm command execution')} onClose={cancel} footer={<><Button disabled={pending} onClick={cancel}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={pending} loading={pending} onClick={()=>onDecision('accept')}>{tr('确认','Confirm')}</Button></>}>
  <div ref={body}>
   {error&&<div role="alert"><Notice tone="danger">{error}</Notice></div>}
   <p className="edge-agent-approval-reason">{approval.reason||tr('Codex 请求你的授权。','Codex is requesting your authorization.')}</p>
   <p className="edge-agent-approval-note">{tr('确认仅授权本次操作；取消将拒绝执行。','Confirmation authorizes this operation once. Cancel denies it.')}{count>1&&` ${tr('剩余待确认：','Pending requests: ')}${count}`}</p>
   <details><summary>{tr('查看操作详情','View operation details')}</summary>
    <code>{approval.directory}</code>
    {approval.command&&<pre>{approval.command}</pre>}
    {approval.changes?.map((change,index)=><div key={index} className="edge-agent-approval-change"><code>{change.path}{change.move_path?` → ${change.move_path}`:''}</code><DiffView text={change.diff}/></div>)}
   </details>
  </div>
 </Modal>,document.body);
}
