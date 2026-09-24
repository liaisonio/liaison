import React from 'react';
import {createRoot} from 'react-dom/client';
import {MemoryRouter,Routes,Route,useLocation} from 'react-router-dom';
import Home from '../src/pages/ManagementAgent';
import Connector from '../src/pages/Connector';
import Proxy from '../src/pages/Proxy';
import App from '../src/pages/App';
import {RuntimeBridge} from '../src/lib/runtime';
import {applyThemeOnBoot} from '../src/store/theme';
import '../src/styles/index.css';
import {usePermissions} from '../src/store/permissions';
import {useSession} from '../src/store/session';
if(!import.meta.env.DEV)throw Error('Development fixture only');
applyThemeOnBoot();
usePermissions.setState({owner:useSession.getState().token,loaded:true,grants:{'settings.global.read':true,'settings.global.update':true}});
const initial=new URLSearchParams(location.search).get('entry')||(new URLSearchParams(location.search).has('app')?'/resource/app':new URLSearchParams(location.search).has('llm')?'/proxy?category=llm':new URLSearchParams(location.search).has('ssh')?'/proxy?access_type=webssh':'/');
function Destination(){const location=useLocation();return <output data-testid="destination">{location.pathname+location.search}</output>;}
createRoot(document.getElementById('root')!).render(<MemoryRouter initialEntries={[initial]}><RuntimeBridge/><main style={{padding:24}}><Routes><Route path="/" element={<Home/>}/><Route path="/connector" element={<Connector/>}/><Route path="/proxy" element={<Proxy/>}/><Route path="/resource/app" element={<App/>}/><Route path="*" element={<Destination/>}/></Routes></main></MemoryRouter>);
