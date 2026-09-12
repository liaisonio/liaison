// Component fixture only. No connection, model request or command execution.
import React,{useState} from 'react';
import {createRoot} from 'react-dom/client';
import ShellAgent from '../src/components/TerminalAssistant/ShellAgent';
import CommandCompletion from '../src/components/TerminalAssistant/CommandCompletion';
import '../src/styles/index.css';
import '../src/components/TerminalAssistant/index.less';
import '../src/pages/WebSSH/index.less';

function Fixture(){
  const [risky,setRisky]=useState(true),[automatic,setAutomatic]=useState(true),[accepted,setAccepted]=useState('');
  const input=risky?'rm -rf ':'ls ';
  return <main style={{padding:24,maxWidth:1200,margin:'auto'}}>
    <p style={{marginBottom:20}}>本地组件预览 · 不连接服务器、不执行命令</p>
    <div className="webssh-shell" style={{display:'block',height:'auto',minHeight:0,overflow:'visible'}}>
      <div style={{position:'relative',height:320,background:'#101418',color:'#d7dee8',padding:16,fontFamily:'monospace'}}>
        demo@host:~$ {input}
        <CommandCompletion view={{input,candidates:[input+(risky?'/tmp/obsolete-cache':'-la')],selected:0,left:8,top:28,above:false}} onAccept={setAccepted}/>
      </div>
      <section className="terminal-assistant">
        <ShellAgent handleId="fixture" ensureSession={async()=>{throw Error('Fixture never connects');}} controls={<>
          <button aria-pressed={automatic} onClick={()=>setAutomatic(!automatic)}>{automatic?'关闭自动提示':'开启自动提示'}</button>
          <button>提示 · Ctrl+Space</button>
          <select aria-label="AI 上下文共享"><option>草稿与 Agent 记忆</option><option>共享目录与最近命令</option></select>
        </>}/>
        <div className="terminal-assistant-help">Tab 接受 · Esc 忽略 · 回车前请检查命令</div>
      </section>
    </div>
    <button style={{marginTop:20}} onClick={()=>setRisky(!risky)}>切换候选</button>
    <output aria-label="Accepted candidate">{accepted}</output>
  </main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
