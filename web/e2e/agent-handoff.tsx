import React, {useState} from 'react';
import {createRoot} from 'react-dom/client';
import {MemoryRouter, Routes, Route} from 'react-router-dom';
import AgentWorkspace from '../src/components/AgentWorkspace';
import {useSessionPath} from '../src/components/SessionReference/useSessionPath';
import {useSession} from '../src/store/session';
import {usePermissions} from '../src/store/permissions';
import {applyThemeOnBoot} from '../src/store/theme';
import {accessDraft, stageAccessDraft, discardAccessDraft} from '../src/components/AgentWorkspace/handoff';
import {accessResults} from '../src/components/AgentWorkspace/AccessResults';
import '../src/styles/index.css';
if(!import.meta.env.DEV)throw Error('Development fixture only');
applyThemeOnBoot();
useSession.getState().setToken('handoff-fixture');
usePermissions.setState({owner:'handoff-fixture',loaded:true,grants:{'ai.home.use':true,'ai.access.use':true}});
Object.assign(window,{handoffTest:{accessDraft,stageAccessDraft,discardAccessDraft,accessResults,setToken:useSession.getState().setToken}});
function Target(){const [open,setOpen]=useState(false);const path=useSessionPath('fixture-handle',open,setOpen);return <AgentWorkspace open={path.agentOpen} handleId="fixture-handle" accessId={12} connectionAvailable={path.matching} title="Analytics" protocol="WebMySQL" onClose={path.closeAgent} recoveredDraft={{prompt:'Existing draft',references:[]}}/>;}
createRoot(document.getElementById('root')!).render(<MemoryRouter initialEntries={['/agent/sessions/session_home']}><Routes><Route path="/agent/sessions/:id" element={<AgentWorkspace open managementSessionId="session_home" title="Home" protocol="Resources" onClose={()=>{}}/>}/><Route path="/webdata/*" element={<Target/>}/></Routes></MemoryRouter>);
