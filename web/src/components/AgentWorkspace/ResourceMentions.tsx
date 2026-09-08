import { useEffect, useId, useRef, useState, type RefObject, type KeyboardEvent } from 'react';
import { Cable, HardDrive, Layers, AtSign, X } from 'lucide-react';
import { useI18n } from '@/i18n';
import { getApplicationList, getDeviceList, getEdgeList } from '@/services/api';
import type { AgentResourceReference } from '@/services/agent';
import './ResourceMentions.less';

const icons = {connector:Cable,device:HardDrive,application:Layers};
const key = (r:AgentResourceReference) => `${r.type}:${r.id}`;

export function ReferenceTags({references,onRemove}:{references:AgentResourceReference[];onRemove?:(ref:AgentResourceReference)=>void}) {
 const {tr}=useI18n();
 return references.length ? <section className="agent-reference-tags" aria-label={tr('引用资源','Referenced resources')}>
  {references.map(ref=>{const Icon=icons[ref.type]||Layers;return <span className="agent-reference-tag" key={key(ref)} title={`${ref.type} #${ref.id}`}><Icon size={13}/><span>{ref.name||`#${ref.id}`}</span><small>#{ref.id}</small>{onRemove&&<button type="button" aria-label={`${tr('移除引用','Remove reference')} ${ref.name||ref.id}`} onClick={()=>onRemove(ref)}><X size={12}/></button>}</span>;})}
 </section>:null;
}

export function useResourceMentions(input:RefObject<HTMLTextAreaElement>, value:string, setValue:(value:string)=>void, enabled=true) {
 const {tr}=useI18n();
 const id=useId();
 const [references,setReferences]=useState<AgentResourceReference[]>([]);
 const [trigger,setTrigger]=useState<{start:number;end:number;query:string}>();
 const [items,setItems]=useState<AgentResourceReference[]>([]);
 const [index,setIndex]=useState(0);
 const [loading,setLoading]=useState(false);
 const [failed,setFailed]=useState(false);
 const dismissed=useRef(false);
 useEffect(()=>{
  if(!trigger)return;
  const outside=(event:PointerEvent)=>{if(event.target instanceof Element&&!event.target.closest('.agent-composer,.management-agent-input'))setTrigger(undefined);};
  document.addEventListener('pointerdown',outside);
  return()=>document.removeEventListener('pointerdown',outside);
 },[!!trigger]);
 const detect=(text:string,cursor:number)=>{
  const match=/(?:^|\s)@([^@\s]{0,128})$/.exec(text.slice(0,cursor));
  setTrigger(enabled&&match?{start:cursor-match[1].length-1,end:cursor,query:match[1]}:undefined);
 };
 useEffect(()=>{
  if(!trigger||!enabled)return;
  let active=true;
  setLoading(true);setFailed(false);setItems([]);setIndex(0);
  const timer=setTimeout(()=>{
   const params={page:1,page_size:10,name:trigger.query};
   void Promise.allSettled([getEdgeList(params),getDeviceList(params),getApplicationList({...params,application_name:trigger.query})]).then(results=>{
    if(!active)return;
    const found:AgentResourceReference[]=[];
    results.forEach((result,i)=>{
     if(result.status!=='fulfilled')return;
     const data=result.value.data as (API.EdgeListResult&API.DeviceListResult&API.ApplicationListResult)|undefined;
     const type=(['connector','device','application'] as const)[i];
     const rows=i===0?data?.edges:i===1?data?.devices:data?.applications;
     rows?.forEach(row=>found.push({type,id:String(row.id),name:row.name}));
    });
    setFailed(results.some(r=>r.status==='rejected'));setItems(found);setLoading(false);
   });
  },250);
  return()=>{active=false;clearTimeout(timer);};
 },[trigger?.query,!!trigger,enabled]);
 const options=items.filter(item=>!references.some(ref=>key(ref)===key(item)));
 const choose=(item:AgentResourceReference)=>{
  if(!trigger||references.length>=8)return;
  setReferences(refs=>[...refs,item]);
  setValue(value.slice(0,trigger.start)+value.slice(trigger.end));
  const cursor=trigger.start;setTrigger(undefined);
  requestAnimationFrame(()=>{input.current?.focus();input.current?.setSelectionRange(cursor,cursor);});
 };
 const onKeyDown=(e:KeyboardEvent<HTMLTextAreaElement>)=>{
  if(!trigger||e.nativeEvent.isComposing)return false;
  if(e.key==='Escape'){e.preventDefault();dismissed.current=true;setTrigger(undefined);return true;}
  if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();setIndex(i=>options.length?(i+(e.key==='ArrowDown'?1:-1)+options.length)%options.length:0);return true;}
  if(e.key==='Enter'&&!e.shiftKey){e.preventDefault();if(options[index]&&!loading)choose(options[index]);return true;}
  return false;
 };
 useEffect(()=>{if(trigger)document.getElementById(`${id}-${index}`)?.scrollIntoView({block:'nearest'});},[index,id,!!trigger]);
 const labels={connector:tr('连接器','Connector'),device:tr('设备','Device'),application:tr('应用','Application')};
 return {
  references,setReferences,
  reset:()=>{setReferences([]);setTrigger(undefined);},
  onChange:(text:string,cursor:number)=>{dismissed.current=false;setValue(text);detect(text,cursor);},
  onSelect:()=>{const el=input.current;if(el&&trigger&&!dismissed.current)detect(el.value,el.selectionStart);},
  onKeyDown,
  inputProps:{'aria-controls':trigger?id:undefined,'aria-expanded':!!trigger,'aria-autocomplete':'list' as const,'aria-activedescendant':trigger&&options[index]?`${id}-${index}`:undefined},
  tags:<ReferenceTags references={references} onRemove={ref=>{setReferences(refs=>refs.filter(r=>key(r)!==key(ref)));input.current?.focus();}}/>,
  button:<button type="button" className="agent-mention-trigger" aria-label={tr('引用资源','Mention a resource')} title={tr('@ 引用连接器、设备或应用','@ Mention a connector, device or application')} disabled={!enabled||references.length>=8} onClick={()=>{const el=input.current;const cursor=el?.selectionStart??value.length;const prefix=cursor&& !/\s/.test(value[cursor-1])?' @':'@';const text=value.slice(0,cursor)+prefix+value.slice(cursor);setValue(text);detect(text,cursor+prefix.length);requestAnimationFrame(()=>{el?.focus();el?.setSelectionRange(cursor+prefix.length,cursor+prefix.length);});}}><AtSign size={17}/></button>,
  picker:trigger&&enabled?<section className="agent-mention-picker" onBlur={e=>{if(!e.currentTarget.contains(e.relatedTarget)&&e.relatedTarget!==input.current)setTrigger(undefined);}}>
   <header>{tr('引用资源','Mention a resource')}<small>{tr('↑ ↓ 选择 · Enter 确认 · Esc 关闭','↑ ↓ Select · Enter Confirm · Esc Close')}</small><button type="button" aria-label={tr('关闭资源选择','Close resource picker')} onClick={()=>setTrigger(undefined)}><X size={14}/></button></header>
   <div id={id} role="listbox" aria-label={tr('资源','Resources')}>
    {references.length>=8?<p>{tr('最多引用 8 个资源','Up to 8 references')}</p>:loading?<p role="status">{tr('搜索中…','Searching…')}</p>:options.length?options.map((item,i)=>{const Icon=icons[item.type];return <button type="button" role="option" aria-selected={i===index} id={`${id}-${i}`} key={key(item)} onMouseDown={e=>e.preventDefault()} onClick={()=>choose(item)}><Icon size={16}/><span>{item.name||`#${item.id}`}<small>{labels[item.type]} · #{item.id}</small></span></button>;}):<p role="status">{tr('没有匹配的可见资源','No matching visible resources')}</p>}
   </div>
   {failed&&<p role="status">{tr('部分资源暂时无法加载，请重试','Some resources could not be loaded. Try again.')}</p>}
  </section>:null,
 };
}
