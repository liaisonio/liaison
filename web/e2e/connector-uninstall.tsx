import React from 'react';
import {createRoot} from 'react-dom/client';
import {MemoryRouter} from 'react-router-dom';
import Connector from '../src/pages/Connector';
import {RuntimeBridge} from '../src/lib/runtime';
import {usePermissions} from '../src/store/permissions';
import {useSession} from '../src/store/session';
import {applyThemeOnBoot} from '../src/store/theme';
import '../src/styles/index.css';
if(!import.meta.env.DEV)throw Error('Development fixture only');
const params=new URLSearchParams(location.search);
if(params.has('locale'))localStorage.setItem('liaison-locale',params.get('locale')!);
if(params.has('theme'))localStorage.setItem('liaison-theme-preference',params.get('theme')!);
applyThemeOnBoot();
usePermissions.setState({owner:useSession.getState().token,loaded:true,grants:{'connectors.uninstall':!params.has('denied')}});
// Read-only fixture with a simulated uninstall endpoint. Never contacts an Edge.
let task:any=null;
window.fetch=async(input,init)=>{
 const path=String(input).split('?')[0];let data:any={},code=200;
 if(path==='/api/v1/edges')data={edges:[{id:7,name:'Private inference connector',status:1,online:1,description:'Workspace network'},{id:8,name:'Other connector',status:1,online:1}]};
 else if(path.endsWith('/uninstall/active'))data=task;
 else if(path.endsWith('/installation'))data={can_uninstall:!params.has('unsupported'),instance_id:'a'.repeat(32),reason:'installation_unverified'};
 else if(path.endsWith('/uninstall')&&init?.method==='POST'){
  code=202;data=task={id:'test-task',status:'running',expires_at:new Date(Date.now()+300000).toISOString()};
 }else if(path.includes('/uninstall/'))data=task={...task,status:params.get('result')||'completed'};
 return new Response(JSON.stringify({code,data}),{status:code,headers:{'Content-Type':'application/json'}});
};
createRoot(document.getElementById('root')!).render(<MemoryRouter><RuntimeBridge/><main style={{padding:24}}><Connector/></main></MemoryRouter>);
