import React,{useState} from 'react';
import {createRoot} from 'react-dom/client';
import LLMConnection from '../src/pages/Proxy/LLMConnection';
import RequestExample from '../src/pages/AIGateway/RequestExample';
import {LLM_PROTOCOLS,clientProtocols} from '../src/constants/llmProtocols';
import {Field,Select,Modal} from '../src/components/ui';
import {applyThemeOnBoot} from '../src/store/theme';
import '../src/styles/index.css';
import '../src/pages/Proxy/connection.less';
import '../src/pages/AIGateway/index.less';
if(!import.meta.env.DEV)throw Error('Development fixture only');
applyThemeOnBoot();
function Fixture(){const [protocol,setProtocol]=useState('openai');return <Modal open width={520} title="LLM" onClose={()=>{}}>
 <Field label="Application type"><Select value={protocol} onChange={e=>setProtocol(e.target.value)}>{LLM_PROTOCOLS.map(p=><option key={p.value} value={p.value}>{p.label}</option>)}</Select></Field>
 <LLMConnection applicationId={protocol} disabled={false}/>
 <div className="ai-api-page"><RequestExample base="/api/v1/ai/accesses/1" model="public-model" protocols={clientProtocols(protocol)}/></div>
 </Modal>}
createRoot(document.getElementById('root')!).render(<Fixture/>);
