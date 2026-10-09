import {useId,useRef,useState,useEffect} from 'react';
import {createPortal} from 'react-dom';
import {Button,Modal} from '@/components/ui';
import {useI18n} from '@/i18n';
import {useCopyText} from '@/components/AgentWorkspace/useCopyText';

export function ProjectPath({path,name}:{path:string;name:string}){
 const {tr}=useI18n(),id=useId(),button=useRef<HTMLButtonElement>(null);
 const [position,setPosition]=useState<{left:number;top:number}>(),[open,setOpen]=useState(false);
 const {state,copy}=useCopyText(path);
 function show(){const rect=button.current?.getBoundingClientRect();if(rect)setPosition({left:Math.max(8,Math.min(rect.left,window.innerWidth-328)),top:rect.bottom+6});}
 useEffect(()=>{
  const update=()=>{if(!open&&document.activeElement===button.current)show();else setPosition(undefined);};
  window.addEventListener('scroll',update,true);window.addEventListener('resize',update);
  return()=>{window.removeEventListener('scroll',update,true);window.removeEventListener('resize',update);};
 },[open]);
 return <div className="edge-agent-project-path"><button ref={button} type="button" className="edge-agent-project-name" aria-label={`${name} · ${tr('查看项目路径','View project path')}`} aria-describedby={position?id:undefined} onMouseEnter={show} onMouseLeave={()=>{if(document.activeElement!==button.current)setPosition(undefined);}} onFocus={show} onBlur={()=>setPosition(undefined)} onKeyDown={e=>{if(e.key==='Escape'){setPosition(undefined);e.stopPropagation();}}} onClick={()=>{setPosition(undefined);setOpen(true);}}>{name}</button>
 {position&&!open&&createPortal(<div id={id} role="tooltip" className="edge-agent-path-tooltip" style={position}>{path}</div>,document.body)}
 <Modal open={open} title={tr('项目路径','Project path')} onClose={()=>{setOpen(false);button.current?.focus({preventScroll:true});}} footer={<Button onClick={()=>void copy()}>{state==='copied'?tr('已复制','Copied'):tr('复制路径','Copy path')}</Button>}>
  <code className="edge-agent-path-value">{path}</code>
  {state==='failed'&&<p role="status">{tr('复制失败，请选择路径手动复制。','Copy failed. Select the path and copy manually.')}</p>}
 </Modal></div>;
}
