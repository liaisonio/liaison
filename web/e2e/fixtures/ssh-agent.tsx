// Development-only visual fixture. No real SSH connection or model calls.
import React, { useEffect, useRef, useState } from 'react';
import { createRoot } from 'react-dom/client';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import '@xterm/xterm/css/xterm.css';
import TerminalAssistant from '../../src/components/TerminalAssistant';
import AgentWorkspace from '../../src/components/AgentWorkspace';
import {usePermissions} from '../../src/store/permissions';
import {useSession} from '../../src/store/session';
import '../../src/styles/index.css';
import '../../src/pages/WebSSH/index.less';
import { applyTheme, applyAccent } from '../../src/store/theme';
if (!import.meta.env.DEV) throw new Error('Development fixture only');
usePermissions.setState({owner:useSession.getState().token,loaded:true,grants:{'ai.access.use':true}});
applyTheme('dark'); applyAccent('brand-purple');
let revision = 0;
const detail = { session: { id:'preview', active_turn_id:'' }, attachments:[], turns:[], steps:[], approvals:[], messages:[
  {id:'1',value:{role:'user',content:'如何查看这个目录下占用空间最大的文件？'}},
  {id:'2',value:{role:'assistant',content:'可以先查看当前目录各项的大小，再按占用空间排序。\n\ndu -sh ./* | sort -h\n\n这是只读操作。你也可以在下方命令草稿栏补全命令，确认后填入终端。'}},
] };
const json = (data: unknown) => new Response(JSON.stringify({code:200, data}), {headers:{'Content-Type':'application/json'}});
const originalFetch = window.fetch.bind(window);
window.fetch = async (url, options) => {
  const path = String(url);
  if (!path.startsWith('/api/v1/')) return originalFetch(url, options);
  if (path.includes('/assistance/')) {
    const body = JSON.parse(String(options?.body || '{}'));
    if (options?.method === 'DELETE') return json({});
    revision = body.revision;
    return json({ revision, cursor:body.cursor, text:' -sh ./* | sort -h' });
  }
  if (path.endsWith('/status')) return json({enabled:true});
  if (path.endsWith('/turns') && options?.method === 'POST') {
    await new Promise(resolve => setTimeout(resolve, 3000));
    return json({text:'模拟回复'});
  }
  if (path.endsWith('/events')) return new Response(new ReadableStream({start(controller) {
    options?.signal?.addEventListener('abort', () => controller.close(), {once:true});
  }}),{headers:{'Content-Type':'text/event-stream'}});
  return json(detail);
};
function Preview() {
  const [open,setOpen] = useState(true);
  const terminalHost = useRef<HTMLDivElement>(null);
  const terminal = useRef<Terminal>();
  useEffect(() => {
    const term = new Terminal({fontSize:14,fontFamily:'Menlo, monospace',cursorBlink:true,theme:{background:'#0d1117',foreground:'#c9d1d9'}});
    const fit = new FitAddon(); term.loadAddon(fit); term.open(terminalHost.current!); fit.fit(); terminal.current = term;
    term.writeln('\x1b[38;2;140;109;240mLiaison · WebSSH\x1b[0m');
    term.writeln('UI preview — simulated connection, no commands are executed.\r\n');
    term.writeln('\x1b[32mdeveloper@demo\x1b[0m:~/workspace$ ls');
    term.writeln('app/    config/    logs/    README.md\r\n');
    term.write('\x1b[32mdeveloper@demo\x1b[0m:~/workspace$ ');
    const observer = new ResizeObserver(() => fit.fit()); observer.observe(terminalHost.current!);
    return () => {observer.disconnect();term.dispose();};
  },[]);
  return <main style={{padding:'24px', maxWidth:1600, margin:'auto'}}>
    <header style={{display:'flex',justifyContent:'space-between',alignItems:'center',marginBottom:20}}><strong style={{fontSize:20}}>Liaison</strong><span style={{color:'rgb(var(--muted))',fontSize:12}}>WebSSH · 第一版 UI 预览（模拟连接与模型）</span></header>
    <div className="webssh-shell" style={{minHeight:'calc(100vh - 100px)',maxHeight:'calc(100vh - 100px)'}}>
      <header className="webssh-toolbar" style={{gridTemplateColumns:'1fr auto'}}><div className="webssh-identity"><span className="webssh-terminal-mark">&gt;_</span><div><strong>Development server</strong><span>Web SSH · developer</span></div></div><button onClick={() => setOpen(!open)} style={{background:'transparent',color:'rgb(var(--ink))',padding:'6px 12px',border:'1px solid rgb(var(--line))',borderRadius:8}}>Agent</button></header>
      <div className="webssh-terminal"><div className="webssh-terminal-screen" ref={terminalHost} /></div>
      <TerminalAssistant handleId="preview-handle" onInsert={text => { terminal.current?.write(text); (window as any).__inserted = text; }} />
      <AgentWorkspace open={open} docked handleId="preview-handle" title="Development server" protocol="WebSSH" onClose={() => setOpen(false)} />
    </div>
  </main>;
}
createRoot(document.getElementById('root')!).render(<Preview/>);
