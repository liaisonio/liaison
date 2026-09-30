import {useCallback, useEffect, useRef, useState} from 'react';
import {createPortal} from 'react-dom';
import {Check, Copy, Maximize} from 'lucide-react';
import {Button, Modal, Select} from '@/components/ui';
import {useI18n} from '@/i18n';

function themeSignature(){
  const html=document.documentElement,styles=getComputedStyle(html);
  return JSON.stringify([html.classList.contains('dark'),...['--ink','--canvas','--panel','--accent','--line'].map(name=>styles.getPropertyValue(name).trim())]);
}

export function MermaidBlock({text,enlarged=false,onClose}:{text:string;enlarged?:boolean;onClose?:()=>void}) {
  const {tr}=useI18n();
  const root=useRef<HTMLDivElement>(null), frame=useRef<HTMLIFrameElement>(null);
  const [visible,setVisible]=useState(false), [source,setSource]=useState(false), [copied,setCopied]=useState(false);
  const [document,setDocument]=useState(''), [status,setStatus]=useState<'loading'|'ready'|'error'>('loading'), [height,setHeight]=useState(220);
  const [theme,setTheme]=useState(themeSignature),[expanded,setExpanded]=useState(false),[zoom,setZoom]=useState<number|null>(null),[percent,setPercent]=useState(100);
  const options=useRef({zoom,onClose});options.current={zoom,onClose};
  const trigger=useRef<HTMLButtonElement>(null),dialog=useRef<HTMLDivElement>(null);
  const close=useCallback(()=>{setExpanded(false);requestAnimationFrame(()=>trigger.current?.focus());},[]);
  useEffect(()=>{
    if(!expanded)return;
    const host=dialog.current,first=host?.querySelector<HTMLButtonElement>('.liaison-modal > header button');
    first?.focus();
    const containFocus=(event:FocusEvent)=>{if(host&&!host.contains(event.target as Node))first?.focus();};
    window.document.addEventListener('focusin',containFocus);
    return ()=>window.document.removeEventListener('focusin',containFocus);
  },[expanded]);
  useEffect(()=>{frame.current?.contentWindow?.postMessage({type:'liaison-mermaid-scale',zoom,enlarged},'*');},[zoom,enlarged]);
  useEffect(()=>{
    const observer=new IntersectionObserver(entries=>{if(entries.some(e=>e.isIntersecting)){setVisible(true);observer.disconnect();}},{rootMargin:'200px'});
    if(root.current)observer.observe(root.current);
    return ()=>observer.disconnect();
  },[]);
  useEffect(()=>{
    const observer=new MutationObserver(()=>setTheme(themeSignature()));
    observer.observe(window.document.documentElement,{attributes:true,attributeFilter:['class','style']});
    return ()=>observer.disconnect();
  },[]);
  useEffect(()=>{
    if(!visible)return;
    let cancelled=false;
    setStatus('loading');setDocument('');
    const timer=window.setTimeout(()=>{
      if(text.length>30000){setStatus('error');return;}
      void import('./mermaidFrame').then(module=>{if(!cancelled)setDocument(module.mermaidFrameDocument());}).catch(()=>{if(!cancelled)setStatus('error');});
    },350);
    const deadline=window.setTimeout(()=>{if(!cancelled){setStatus('error');setDocument('');}},15000);
    const receive=(event:MessageEvent)=>{
      if(!frame.current||event.source!==frame.current.contentWindow)return;
      if(event.data?.type==='liaison-mermaid-ready'){
        const html=window.document.documentElement, styles=getComputedStyle(html);
        const color=(name:string)=>`rgb(${styles.getPropertyValue(name).trim()})`;
        frame.current.contentWindow?.postMessage({type:'liaison-mermaid-render',text,zoom:options.current.zoom,enlarged,dark:html.classList.contains('dark'),ink:color('--ink'),canvas:color('--canvas'),panel:color('--panel'),accent:color('--accent'),line:color('--line')},'*');
      }else if(event.data?.type==='liaison-mermaid-result'){
        clearTimeout(deadline);
        if(event.data.ok===true&&Number.isFinite(event.data.height)){
          setHeight(Math.min(600,Math.max(100,event.data.height)));setStatus('ready');
        }else{setStatus('error');setDocument('');}
      }else if(event.data?.type==='liaison-mermaid-scale-result'&&Number.isFinite(event.data.percent)){
        setPercent(Math.max(1,Math.min(400,Math.round(event.data.percent))));
      }else if(event.data?.type==='liaison-mermaid-close'){options.current.onClose?.();}
    };
    window.addEventListener('message',receive);
    return ()=>{cancelled=true;clearTimeout(timer);clearTimeout(deadline);window.removeEventListener('message',receive);};
  },[text,visible,theme,enlarged]);
  const tabs=<>
      <Button variant="ghost" aria-pressed={!source} onClick={()=>setSource(false)}>{tr('图形','Diagram')}</Button>
      <Button variant="ghost" aria-pressed={source} onClick={()=>setSource(true)}>{tr('源码','Source')}</Button>
    </>;
  return <div ref={root} className="agent-code agent-mermaid">
    <div className="agent-mermaid-toolbar">{enlarged?<div>{tabs}</div>:<span>Mermaid</span>}<div>
      {!enlarged&&tabs}
      {enlarged&&<span className="agent-mermaid-zoom" style={{visibility:source?'hidden':undefined}}>
        <Select aria-label={tr('缩放比例','Zoom level')} value={zoom===null?'fit':String(zoom)} onChange={event=>setZoom(event.target.value==='fit'?null:Number(event.target.value))}>
          <option value="fit">{percent}%</option>
          {[...new Set([10,25,50,75,100,125,150,200,300,400,...(zoom===null?[]:[zoom])])].sort((a,b)=>a-b).map(n=><option key={n} value={n}>{n}%</option>)}
        </Select>
        <Button variant="ghost" aria-label={tr('适应窗口','Fit window')} title={tr('适应窗口','Fit window')} onClick={()=>{setZoom(null);frame.current?.contentWindow?.postMessage({type:'liaison-mermaid-scale',zoom:null,enlarged},'*');}}>{tr('适应窗口','Fit window')}</Button>
      </span>}
      {!enlarged&&<button ref={trigger} className="liaison-button is-ghost" type="button" aria-label={tr('放大图表','Expand diagram')} title={tr('放大图表','Expand diagram')} onClick={()=>setExpanded(true)}><Maximize size={13}/></button>}
      <Button variant="ghost" aria-label={tr('复制源码','Copy source')} title={tr('复制源码','Copy source')} onClick={()=>{void navigator.clipboard.writeText(text).then(()=>setCopied(true)).catch(()=>setCopied(false));}}>{copied?<Check size={13}/>:<Copy size={13}/>}</Button>
    </div></div>
    {source&&<pre className={enlarged?'agent-mermaid-expanded-source':undefined}><code>{text}</code></pre>}
    <div className={enlarged?'agent-mermaid-expanded-canvas':undefined} style={{display:source?'none':undefined}}>
      {status==='loading'&&<div className="agent-mermaid-notice" role="status">{tr('正在绘制图表…','Rendering diagram…')}</div>}
      {status==='error'&&<div className="agent-mermaid-notice" role="status">{tr('暂时无法绘制，可查看源码。','Unable to render. You can view the source.')}</div>}
      {document&&<iframe ref={frame} title={tr('Mermaid 图表','Mermaid diagram')} sandbox="allow-scripts" referrerPolicy="no-referrer" srcDoc={document} style={{height:enlarged?'100%':height,visibility:status==='ready'?'visible':'hidden'}}/>}
    </div>
    {expanded&&createPortal(<div ref={dialog}><Modal open title={tr('Mermaid 图表','Mermaid diagram')} width={1200} className="is-mermaid-viewer" onClose={close}><MermaidBlock text={text} enlarged onClose={close}/></Modal></div>,window.document.body)}
  </div>;
}
