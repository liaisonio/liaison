import {useState} from 'react';
import {Button,Field,Input} from '@/components/ui';
import {useI18n} from '@/i18n';
import type {AgentInputRequest} from '@/services/edgeAgent';

export function InputRequest({request,disabled,onAnswer}:{request:AgentInputRequest;disabled:boolean;onAnswer:(answers:{question_id:string;answers:string[]}[])=>void}){
 const {tr}=useI18n();
 const [selected,setSelected]=useState<Record<string,string>>({});
 const [custom,setCustom]=useState<Record<string,string>>({});
 const value=(q:AgentInputRequest['questions'][number])=>selected[q.id]==='custom'||!q.options?.length?custom[q.id]||'':q.options[Number(selected[q.id])]?.label||'';
 const complete=request.questions.every(q=>value(q).trim());
 return <form className="edge-agent-input-request" aria-label={tr('回答问题','Answer questions')} onSubmit={e=>{e.preventDefault();if(!disabled&&complete)onAnswer(request.questions.map(q=>({question_id:q.id,answers:[value(q)]})));}}>
  <strong>{request.blocking?tr('需要你的选择','Your input is needed'):tr('补充信息','Additional input')}</strong>
  {request.questions.map(q=><fieldset key={q.id} disabled={disabled}>
   <legend>{q.header&&<span>{q.header}</span>}{q.question}</legend>
   {q.options?.map((option,index)=><label key={index} className={`edge-agent-input-option ${selected[q.id]===String(index)?'is-selected':''}`}>
    <input type="radio" name={`${request.id}-${q.id}`} checked={selected[q.id]===String(index)} onChange={()=>setSelected(v=>({...v,[q.id]:String(index)}))}/>
    <span><strong>{option.label}</strong>{option.description&&<small>{option.description}</small>}</span>
   </label>)}
   {q.is_other&&Boolean(q.options?.length)&&<label className={`edge-agent-input-option ${selected[q.id]==='custom'?'is-selected':''}`}><input type="radio" name={`${request.id}-${q.id}`} checked={selected[q.id]==='custom'} onChange={()=>setSelected(v=>({...v,[q.id]:'custom'}))}/><span>{tr('其他回答','Other answer')}</span></label>}
   {(!q.options?.length||selected[q.id]==='custom')&&<Field label={tr('你的回答','Your answer')} hint={q.is_secret?tr('回答会传给 Codex，不会写入 Liaison 历史。','Sent to Codex, not stored in Liaison history.'):undefined}><Input type={q.is_secret?'password':'text'} autoComplete="off" maxLength={4096} value={custom[q.id]||''} onChange={e=>setCustom(v=>({...v,[q.id]:e.target.value}))}/></Field>}
  </fieldset>)}
  <div className="edge-agent-input-actions"><Button type="submit" variant="primary" disabled={disabled||!complete} loading={disabled}>{tr('提交回答','Submit answers')}</Button></div>
 </form>;
}
