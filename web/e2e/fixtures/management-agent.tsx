// Development-only UI fixture. No real resources or model calls.
import React from 'react';
import { MemoryRouter, Routes, Route, useLocation } from 'react-router-dom';
import { createRoot } from 'react-dom/client';
import ManagementAgent from '../../src/pages/ManagementAgent';
import { usePermissions } from '../../src/store/permissions';
import { useSession } from '../../src/store/session';
import '../../src/styles/index.css';
import { applyTheme, applyAccent } from '../../src/store/theme';
if (!import.meta.env.DEV) throw new Error('Development fixture only');
usePermissions.setState({owner:useSession.getState().token,loaded:true,grants:{'ai.home.use':true}});
applyTheme('dark'); applyAccent('brand-purple');
const session = { id: 'management-preview', kind:'management', title:'查看我的连接器', status:0, version:1 };
let messages: unknown[] = [];
let submits = 0;
const response = (data:unknown) => new Response(JSON.stringify({code:200,data}),{headers:{'Content-Type':'application/json'}});
const originalFetch = window.fetch.bind(window);
window.fetch = async (url, options) => {
  const path=String(url);
  const resourceKey=path.startsWith('/api/v1/edges')?'edges':path.startsWith('/api/v1/devices')?'devices':path.startsWith('/api/v1/applications')?'applications':undefined;
  if(resourceKey){const query=new URL(path,location.origin).searchParams;const name=query.get('application_name')||query.get('name')||'';const items=[{id:7,name:'办公室'},{id:8,name:'测试服务器'}].filter(i=>i.name.includes(name));return response({[resourceKey]:items,total:items.length});}
  if (!path.startsWith('/api/v1/agent')) return originalFetch(url, options);
  if(path.endsWith('/status'))return response({enabled:true,models:[{provider_id:'deepseek',provider_type:'deepseek',model:'deepseek-chat',is_default:true},{provider_id:'deepseek',provider_type:'deepseek',model:'deepseek-reasoner',is_default:false},{provider_id:'anthropic',provider_type:'anthropic',model:'claude-sonnet',is_default:false}]});
  if(path.endsWith('/sessions'))return options?.method==='POST'?response({session,messages}):response({items:[session,{id:'ssh',kind:'access',title:'PRIVATE SSH',status:0}]});
  if(path.endsWith('/turns')) {
    submits++; (window as any).__managementSubmits=submits;
    (window as any).__selectedModel=JSON.parse(String(options?.body||'{}')).model_selection;
    const prompt=JSON.parse(String(options?.body||'{}')).prompt;
    const references=JSON.parse(String(options?.body||'{}')).references;
    (window as any).__references=references;
    messages=[...messages,{id:`u${submits}`,value:{role:'user',content:prompt,references:references?.map((r:any)=>({...r,name:r.id==='7'?'办公室':'测试服务器'}))}}, {id:`a${submits}`,value:{role:'assistant',content:'你有 2 个连接器：\n\n- **办公室**：在线\n- **测试服务器**：离线\n\n这里只展示你有权限查看的资源。'}}];
    return response({});
  }
  if(path.endsWith('/events')) return new Response(new ReadableStream({start(c){c.enqueue(new TextEncoder().encode('data: {"type":"turn.completed"}\n\n'));}}),{headers:{'Content-Type':'text/event-stream'}});
  return response({session, messages});
};
function Preview() {
  const location = useLocation();
  return <><output data-testid="path">{location.pathname}</output><ManagementAgent /></>;
}
createRoot(document.getElementById('root')!).render(<React.StrictMode><MemoryRouter><div style={{padding:'24px',maxWidth:1440,margin:'auto'}}><Routes><Route path="/" element={<Preview />} /><Route path="/agent/sessions/:agentSessionId" element={<Preview />} /></Routes></div></MemoryRouter></React.StrictMode>);
