import {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {Transcript} from '../src/pages/EdgeAgent/Transcript';
import type {AgentSnapshot} from '../src/services/edgeAgent';
import '../src/styles/index.css';
import '../src/components/AgentWorkspace/index.less';
import '../src/pages/EdgeAgent/index.less';
import '../src/pages/EdgeAgent/interaction.less';
function Fixture(){
 const [window,setWindow]=useState(45),[id,setID]=useState('s'.repeat(32));
 const session:AgentSnapshot={version:1,status:'ok',session_id:id,window,running:true,closed:false,messages:[{role:'user',text:`Round ${window}`},{role:'assistant',text:'Latest answer. '+('检查 `operations/map` 与文件的对应关系。 Verify multipart metadata. '.repeat(30))}],activities:[{id:'long-command',kind:'commandExecution',status:'running',message_index:1,duration_ms:0,command:'/bin/bash -lc '+('cat internal/edge/data/schema/multipart.go && '.repeat(100)),output:'x'.repeat(5000)}]};
 return <main style={{padding:24}}><button onClick={()=>setWindow(w=>w+1)}>Next round</button><button onClick={()=>setWindow(w=>w+25)}>Remote rounds</button><button onClick={()=>{setID('t'.repeat(32));setWindow(0);}}>Switch session</button><section className="edge-agent-workspace" style={{height:'calc(100vh - 100px)'}}><Transcript key={id} session={session} edge={6} accessID={'a'.repeat(32)}><p>Empty</p></Transcript></section></main>;
}
createRoot(document.getElementById('root')!).render(<Fixture/>);
