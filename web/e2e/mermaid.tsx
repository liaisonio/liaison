import {useEffect,useState} from 'react';
import {createRoot} from 'react-dom/client';
import {MessageContent} from '../src/components/AgentWorkspace/MessageContent';
import '../src/styles/index.css';
import '../src/components/AgentWorkspace/index.less';
function Fixture(){
 const [tick,setTick]=useState(0),[tail,setTail]=useState('');
 useEffect(()=>{const timer=setInterval(()=>setTick(n=>n+1),200);return()=>clearInterval(timer);},[]);
 const [code,setCode]=useState('flowchart LR\n A["客户端 Client"] --> B["Liaison"]\n B --> C["连接器 Connector"]\n C --> D["Codex app-server"]');
 return <main style={{padding:20,maxWidth:900,margin:'auto'}}><span>Poll {tick}</span><button onClick={()=>setTail(s=>s+' More streaming text.')}>Append text</button><textarea aria-label="Diagram source" value={code} onChange={e=>setCode(e.target.value)} style={{width:'100%',height:100}}/><MessageContent text={'```mermaid\n'+code+'\n```\n\n'+tail}/></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
