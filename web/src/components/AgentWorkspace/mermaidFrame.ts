// The renderer runs in an opaque-origin frame, never in the console DOM.
// Bundle locally: diagram text must not be sent to a CDN or rendering service.
import runtime from 'mermaid/dist/mermaid.min.js?raw';

export function mermaidFrameDocument() {
  const nonce = crypto.randomUUID().replace(/-/g, '');
  return `<!doctype html><html><head><meta charset="utf-8"><meta http-equiv="Content-Security-Policy" content="default-src 'none'; script-src 'nonce-${nonce}'; style-src 'unsafe-inline'; img-src 'none'; connect-src 'none'; font-src 'none'; base-uri 'none'; form-action 'none'"><style>body{margin:0;padding:12px;box-sizing:border-box;font-family:system-ui,sans-serif}svg{display:block;margin:auto;max-width:100%;height:auto}a{pointer-events:none}</style></head><body><div id="diagram"></div><script nonce="${nonce}">${runtime.replace(/<\/script/gi, '<\\/script')}</script><script nonce="${nonce}">
let rendered=false;
let zoom=null,enlarged=false;
const scale=()=>{
  const svg=document.querySelector('#diagram svg');if(!svg)return;
  const box=svg.viewBox.baseVal;if(!box.width||!box.height)return;
  const ratio=zoom===null?Math.min(1,Math.max(1,innerWidth-24)/box.width,enlarged?Math.max(1,innerHeight-24)/box.height:1):zoom/100;
  svg.style.maxWidth='none';svg.style.width=box.width*ratio+'px';svg.style.height=box.height*ratio+'px';svg.style.margin=zoom===null?'auto':'0';
  parent.postMessage({type:'liaison-mermaid-scale-result',percent:ratio*100},'*');
};
addEventListener('resize',scale);
addEventListener('keydown',event=>{if(event.key==='Escape')parent.postMessage({type:'liaison-mermaid-close'},'*');});
let drag=null;
document.addEventListener('pointerdown',event=>{if(!enlarged||event.button!==0)return;drag={x:event.clientX,y:event.clientY,left:scrollX,top:scrollY};document.body.setPointerCapture(event.pointerId);document.body.style.cursor='grabbing';event.preventDefault();});
document.addEventListener('pointermove',event=>{if(drag)scrollTo(drag.left+drag.x-event.clientX,drag.top+drag.y-event.clientY);});
const endDrag=()=>{drag=null;document.body.style.cursor=enlarged?'grab':'';};
document.addEventListener('pointerup',endDrag);document.addEventListener('pointercancel',endDrag);
window.addEventListener('message', async event => {
  if(event.source!==parent)return;
  if(event.data?.type==='liaison-mermaid-scale'){zoom=typeof event.data.zoom==='number'?Math.max(10,Math.min(400,event.data.zoom)):null;scale();if(zoom===null)scrollTo(0,0);return;}
  if(rendered || event.data?.type!=='liaison-mermaid-render')return;
  rendered=true;
  enlarged=event.data.enlarged===true;zoom=typeof event.data.zoom==='number'?Math.max(10,Math.min(400,event.data.zoom)):null;
  document.body.style.cursor=enlarged?'grab':'';
  const {text, dark, ink, panel, accent, line, canvas}=event.data;
  document.documentElement.style.backgroundColor=canvas;
  document.body.style.color=ink;
  try {
    if(typeof text!=='string'||text.length>30000||/%%\\s*\\{|^\\s*---/m.test(text))throw new Error('Unsupported configuration');
    mermaid.initialize({startOnLoad:false,securityLevel:'strict',suppressErrorRendering:true,maxTextSize:30000,maxEdges:300,htmlLabels:false,flowchart:{htmlLabels:false},theme:'base',themeVariables:{darkMode:dark,background:panel,primaryColor:panel,primaryTextColor:ink,primaryBorderColor:accent,lineColor:ink,secondaryColor:panel,tertiaryColor:panel,clusterBkg:panel,clusterBorder:line,fontFamily:'system-ui, sans-serif'}});
    const {svg}=await mermaid.render('liaison-diagram',text);
    document.getElementById('diagram').innerHTML=svg;
    // No diagram links, image loads, or event bindings are enabled.
    document.querySelectorAll('a').forEach(a=>{a.removeAttribute('href');a.removeAttribute('xlink:href');});
    scale();
    const report=()=>parent.postMessage({type:'liaison-mermaid-result',ok:true,height:Math.ceil(document.getElementById('diagram').getBoundingClientRect().height)+24},'*');
    report();
    new ResizeObserver(report).observe(document.getElementById('diagram'));
  } catch {
    document.getElementById('diagram').replaceChildren();
    parent.postMessage({type:'liaison-mermaid-result',ok:false},'*');
  }
});
parent.postMessage({type:'liaison-mermaid-ready'},'*');
</script></body></html>`;
}
