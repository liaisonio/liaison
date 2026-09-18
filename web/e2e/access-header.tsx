import React from 'react';
import {createRoot} from 'react-dom/client';
import {MemoryRouter,Routes,Route,useLocation} from 'react-router-dom';
import LLMProtocol from '../src/components/icons/LLMProtocol';
import AccessContext from '../src/components/AccessContext';
import {useAccessBack} from '../src/hooks/useAccessBack';
import {applyThemeOnBoot} from '../src/store/theme';
import '../src/styles/index.css';
import '../src/pages/WebSSH/index.less';
import '../src/pages/WebDesktop/index.less';
import '../src/pages/WebData/index.less';
if(!import.meta.env.DEV)throw Error('Development fixture only');
applyThemeOnBoot();
function HeaderTest(){const back=useAccessBack('/proxy?access_type=webssh');return <main style={{padding:24}}><button onClick={back}>Back</button>{['webssh','webdesktop','webdata','websftp','llm'].map((family,i)=><section key={family} style={{marginBlock:24,padding:16,border:'1px solid rgb(var(--line))'}}><div className={`${family}-identity`}><AccessContext name="Demo access" protocol={family==='llm'?<LLMProtocol protocol="qwen"/>:['SSH','RDP','Oracle','SFTP'][i]} target="server.example:2222"/></div></section>)}</main>}
function Destination(){return <output>{useLocation().pathname}{useLocation().search}</output>}
createRoot(document.getElementById('root')!).render(<MemoryRouter initialEntries={['/workspace'+location.search]}><Routes><Route path="/workspace" element={<HeaderTest/>}/><Route path="*" element={<Destination/>}/></Routes></MemoryRouter>);
