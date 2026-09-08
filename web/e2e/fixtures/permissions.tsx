import React from 'react';
import {createRoot} from 'react-dom/client';
import Permissions from '../../src/pages/User/Permissions';
import '../../src/styles/index.css';
import '../../src/pages/Settings/index.less';
import {applyTheme,applyAccent} from '../../src/store/theme';
if(!import.meta.env.DEV)throw new Error('Development fixture only');
applyTheme('dark');applyAccent('brand-purple');
let enabled:string[]=[];
const codes=['audit.read','organizations.read','ai.home.use','ai.access.use','settings.global.read','settings.global.update'];
window.fetch=async(_url,options)=>{
 if(options?.method==='PUT') { enabled=JSON.parse(String(options.body)).enabled; (window as any).__saved=enabled; }
 return new Response(JSON.stringify({code:200,data:{catalog:codes.map(code=>({code,name:code})),enabled}}),{headers:{'Content-Type':'application/json'}});
};
createRoot(document.getElementById('root')!).render(<React.StrictMode><div className="settings-page" style={{maxWidth:980,margin:'40px auto',padding:32}}><Permissions /></div></React.StrictMode>);
