import {useEffect,useId,useRef,useState,type RefObject} from 'react';
import {useI18n} from '@/i18n';
import type {AgentSkill} from '@/services/edgeAgent';

export function SlashMenu({text,setText,input,skills,available,disabled,onSkill,onStatus,onNew,forcedOpen=false,onClose}:{text:string;setText:(s:string)=>void;input:RefObject<HTMLTextAreaElement>;skills:AgentSkill[];available:boolean;disabled:boolean;onSkill:(s:AgentSkill)=>void;onStatus:()=>void;onNew:()=>void;forcedOpen?:boolean;onClose?:()=>void}){
  const {tr}=useI18n();const [index,setIndex]=useState(0),[dismissed,setDismissed]=useState(false);
  const id=useId(),list=useRef<HTMLDivElement>(null);
  const query=!forcedOpen&&text.startsWith('/')?text.slice(1).toLowerCase():'';
  const open=(forcedOpen||/^\/[^\s/]*$/.test(text))&&!disabled&&!dismissed;
  const commands=[{id:'status',name:'/status',description:tr('查看会话信息','View session details'),run:onStatus},{id:'new',name:'/new',description:tr('在当前项目新建会话','Start a new conversation in this project'),run:onNew}];
  const options=[...commands,...skills.map(s=>({id:s.id,name:s.name,description:s.description,run:()=>onSkill(s)}))].filter(o=>`${o.name} ${o.description}`.toLowerCase().includes(query));
  function choose(i:number){const selected=options[i];if(!selected)return;selected.run();if(!forcedOpen)setText('');onClose?.();input.current?.focus();}
  useEffect(()=>{setIndex(0);setDismissed(false);},[text,forcedOpen]);
  useEffect(()=>{if(open)list.current?.querySelector('[aria-selected="true"]')?.scrollIntoView({block:'nearest'});},[index,open]);
  useEffect(()=>{
    const el=input.current;if(!el)return;
    if(open){el.setAttribute('aria-controls',id);el.setAttribute('aria-activedescendant',`${id}-${index}`);el.setAttribute('aria-haspopup','listbox');}
    return()=>{el.removeAttribute('aria-controls');el.removeAttribute('aria-activedescendant');el.removeAttribute('aria-haspopup');};
  },[open,index,id]);
  useEffect(()=>{
    const el=input.current;if(!open||!el)return;
    const handle=(e:KeyboardEvent)=>{
      if(e.isComposing||!['ArrowDown','ArrowUp','Enter','Escape','Tab'].includes(e.key))return;
      if(e.key==='Enter'&&e.shiftKey)return;
      e.preventDefault();e.stopPropagation();e.stopImmediatePropagation();
      if(e.key==='Escape'){setDismissed(true);onClose?.();return;}
      if(e.key==='Enter'||e.key==='Tab'){choose(index);return;}
      setIndex(n=>options.length?(n+(e.key==='ArrowDown'?1:-1)+options.length)%options.length:0);
    };
    el.addEventListener('keydown',handle,true);return()=>el.removeEventListener('keydown',handle,true);
  },[open,index,text,skills,forcedOpen]);
  if(!open)return null;
  return <section className="edge-agent-slash" aria-label={tr('命令与技能','Commands and skills')}>
    <div className="edge-agent-slash-title">{tr('命令与技能','Commands and skills')}</div>
    <div id={id} ref={list} role="listbox" aria-label={tr('选择命令或技能','Choose a command or skill')}>{options.map((o,i)=><button id={`${id}-${i}`} tabIndex={-1} type="button" role="option" aria-selected={i===index} className={i===index?'is-active':''} key={o.id} onMouseDown={e=>e.preventDefault()} onClick={()=>choose(i)}><strong>{o.name}</strong><span title={o.description}>{o.description}</span></button>)}</div>
    {!options.length&&<p>{tr('没有匹配的命令或技能。','No matching commands or skills.')}</p>}
    {!available&&<p>{tr('当前 Codex 未提供技能列表。','Skills are unavailable from this Codex instance.')}</p>}
    {available&&!skills.length&&<p>{tr('当前项目没有可用技能。','No skills available for this project.')}</p>}
  </section>;
}
