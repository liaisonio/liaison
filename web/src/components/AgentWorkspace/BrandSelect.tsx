import {useEffect,useId,useRef,useState,type ReactNode} from 'react';
import {createPortal} from 'react-dom';
import {Check,ChevronDown} from 'lucide-react';
import './BrandSelect.less';

export type BrandOption={value:string;label:string;icon:ReactNode;disabled?:boolean};
export default function BrandSelect({label,value,options,onChange,disabled=false}:{label:string;value:string;options:BrandOption[];onChange:(value:string)=>void;disabled?:boolean}){
 const id=useId(),button=useRef<HTMLButtonElement>(null),menu=useRef<HTMLDivElement>(null);
 const [open,setOpen]=useState(false),[active,setActive]=useState(0),[position,setPosition]=useState({left:0,top:0,width:280,height:280});
 const selected=options.find(o=>o.value===value)||options[0];
 useEffect(()=>{if(!open)return;const close=(e:PointerEvent)=>{if(!button.current?.contains(e.target as Node)&&!menu.current?.contains(e.target as Node))setOpen(false);};const hide=()=>setOpen(false);document.addEventListener('pointerdown',close);window.addEventListener('resize',hide);return()=>{document.removeEventListener('pointerdown',close);window.removeEventListener('resize',hide);};},[open]);
 useEffect(()=>{if(disabled)setOpen(false);},[disabled]);
 useEffect(()=>{if(open)document.getElementById(`${id}-${active}`)?.scrollIntoView({block:'nearest'});},[active,open,id]);
 const show=()=>{const r=button.current!.getBoundingClientRect();const width=Math.min(Math.max(r.width,280),innerWidth-24),height=Math.min(280,Math.max(100,innerHeight-32));setPosition({left:Math.min(Math.max(12,r.left),innerWidth-width-12),top:r.bottom+height+8<innerHeight?r.bottom+6:Math.max(12,r.top-height-6),width,height});setActive(Math.max(0,options.findIndex(o=>o.value===value)));setOpen(true);};
 const choose=(index:number)=>{const o=options[index];if(o&&!o.disabled){onChange(o.value);setOpen(false);button.current?.focus();}};
 return <>
  <button ref={button} type="button" className="brand-select" role="combobox" aria-label={label} aria-expanded={open} aria-controls={open?id:undefined} aria-haspopup="listbox" aria-activedescendant={open?`${id}-${active}`:undefined} disabled={disabled} onClick={()=>open?setOpen(false):show()} onBlur={e=>{if(!menu.current?.contains(e.relatedTarget))setOpen(false);}} onKeyDown={e=>{
   if(e.key==='Escape'){setOpen(false);return;}
   if(['ArrowDown','ArrowUp','Home','End'].includes(e.key)){e.preventDefault();if(!open){show();return;}setActive(i=>e.key==='Home'?0:e.key==='End'?options.length-1:(i+(e.key==='ArrowDown'?1:-1)+options.length)%options.length);}
   if(open&&(e.key==='Enter'||e.key===' ')){e.preventDefault();choose(active);}
  }}><span className="brand-select-icon">{selected?.icon}</span><span className="brand-select-label">{selected?.label}</span><ChevronDown size={13}/></button>
  {open&&createPortal(<div ref={menu} id={id} className="brand-select-menu" role="listbox" aria-label={label} style={{left:position.left,top:position.top,width:position.width,maxHeight:position.height}}>{options.map((o,i)=><div key={o.value} id={`${id}-${i}`} role="option" aria-selected={o.value===value} aria-disabled={o.disabled||undefined} className={active===i?'is-active':''} onMouseDown={e=>e.preventDefault()} onMouseEnter={()=>setActive(i)} onClick={()=>choose(i)}><span className="brand-select-icon">{o.icon}</span><span>{o.label}</span>{o.value===value&&<Check size={14}/>}</div>)}</div>,document.body)}
 </>;
}
