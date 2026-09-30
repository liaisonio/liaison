import {useEffect,useRef,useState,type CSSProperties} from 'react';
import {useI18n} from '@/i18n';

const key='liaison-agent-sidebar-width',minimum=200,maximum=420,initial=230;
const clamp=(value:number,max=maximum)=>Math.max(minimum,Math.min(max,value));

export function useSessionResize(){
 const {tr}=useI18n();
 const layout=useRef<HTMLDivElement>(null),drag=useRef<{pointer:number;x:number;width:number}>();
 const [preferred,setPreferred]=useState(()=>{try{const value=Number(localStorage.getItem(key));return Number.isFinite(value)&&value>0?clamp(value):initial;}catch{return initial;}});
 const [limit,setLimit]=useState(maximum),[dragging,setDragging]=useState(false);
 const width=Math.min(preferred,limit);
 useEffect(()=>{
  const el=layout.current;if(!el)return;
  const observer=new ResizeObserver(()=>setLimit(clamp(el.clientWidth-480)));
  observer.observe(el);return()=>observer.disconnect();
 },[]);
 function change(value:number){const next=clamp(value,limit);setPreferred(next);try{localStorage.setItem(key,String(next));}catch{/* Resizing still works when browser storage is unavailable. */}}
 function end(){drag.current=undefined;setDragging(false);}
 const separator=<div className="agent-workspace-divider edge-agent-session-resize" role="separator" tabIndex={0} aria-orientation="vertical" aria-controls="agent-session-list" aria-label={tr('调整会话列表宽度','Resize session list')} aria-valuemin={minimum} aria-valuemax={limit} aria-valuenow={width} title={tr('拖动调整宽度，双击恢复默认','Drag to resize; double-click to reset')}
  onPointerDown={event=>{if(event.button!==0)return;event.preventDefault();event.currentTarget.blur();event.currentTarget.setPointerCapture(event.pointerId);drag.current={pointer:event.pointerId,x:event.clientX,width};setDragging(true);}}
  onPointerMove={event=>{const start=drag.current;if(start?.pointer===event.pointerId)change(start.width+event.clientX-start.x);}}
  onPointerUp={end} onPointerCancel={end} onLostPointerCapture={end} onDoubleClick={()=>change(initial)}
  onKeyDown={event=>{const step=event.shiftKey?40:10;const value=event.key==='ArrowLeft'?width-step:event.key==='ArrowRight'?width+step:event.key==='Home'?minimum:event.key==='End'?limit:event.key==='Enter'?initial:undefined;if(value!==undefined){event.preventDefault();change(value);}}}/>
 return {layout,separator,dragging,style:{'--session-list-width':`${width}px`} as CSSProperties};
}
