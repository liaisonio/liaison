import {useEffect,useRef,useState} from 'react';
import {Button,Field,Input,Modal,Notice,Select} from '@/components/ui';
import {useI18n} from '@/i18n';
import {DirectoryBrowser} from './DirectoryBrowser';
import {edgeAgent,saveAgentAccess,type AgentAccess,type AgentConnector,type Installation} from '@/services/edgeAgent';

export default function AccessForm({entry,connectors,onClose,onSaved}:{entry?:AgentAccess;connectors:AgentConnector[];onClose:()=>void;onSaved:()=>void}){
  const {tr}=useI18n();const [name,setName]=useState(entry?.name||`Access-${crypto.randomUUID().slice(0,8)}`),[edge,setEdge]=useState(String(entry?.edge_id||connectors.find(c=>c.online)?.id||'')),[project,setProject]=useState(entry?.project||''),[installation,setInstallation]=useState(entry?.installation_id||'');
  const [items,setItems]=useState<Installation[]>([]),[pending,setPending]=useState(false),[error,setError]=useState(''),[discovered,setDiscovered]=useState(false);
  const [browsing,setBrowsing]=useState(false);
  const alive=useRef(true);useEffect(()=>{alive.current=true;return()=>{alive.current=false;};},[]);
  async function discover(){setPending(true);setError('');try{
    const result=await edgeAgent(Number(edge),'discover');if(!alive.current)return;
    if(result.status!=='ok'){setError(result.status==='unsupported_user'?tr('请使用普通用户运行的 macOS 或 Linux 连接器。','Use a macOS or Linux connector running as a non-root user.'):tr('无法发现 Agent，请检查连接器及其版本。','Cannot discover Agents. Check the connector and its version.'));return;}
    setDiscovered(true);setItems(result.installations||[]);setInstallation(result.installations?.find(i=>i.id===installation)?.id||result.installations?.[0]?.id||'');if(!project)setProject(result.default_project||'');
  }catch{if(alive.current)setError(tr('发现失败，请重试。','Discovery failed. Try again.'));}finally{if(alive.current)setPending(false);}}
  async function save(){setPending(true);setError('');try{await saveAgentAccess({name:name.trim(),kind:'codex',edge_id:Number(edge),installation_id:installation,project:project.trim()},entry?.id);if(alive.current)onSaved();}catch{if(alive.current)setError(tr('保存失败，请检查配置和连接器权限。','Could not save. Check the configuration and connector permissions.'));}finally{if(alive.current)setPending(false);}}
  return <><Modal open={!browsing} title={entry?tr('编辑访问','Edit access'):tr('新建访问','Create access')} onClose={()=>{if(!pending)onClose();}} footer={<><Button disabled={pending} onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" loading={pending} disabled={pending||!name.trim()||!edge||!installation||!project.trim()} onClick={()=>void save()}>{entry?tr('保存','Save'):tr('创建访问','Create access')}</Button></>}>
    <div className="edge-agent-form">
      <div className="edge-agent-fields"><Field label={tr('访问名称','Access name')} required><Input value={name} maxLength={120} disabled={pending} onChange={e=>setName(e.target.value)}/></Field><Field label={tr('访问协议','Access protocol')}><Select value="codex" disabled><option value="codex">Codex</option></Select></Field></div>
      <Field label={tr('连接器','Connector')} required><Select value={edge} disabled={pending} onChange={e=>{setEdge(e.target.value);setInstallation('');setItems([]);setProject('');setDiscovered(false);setError('');}}><option value="">{tr('选择连接器','Choose connector')}</option>{connectors.map(c=><option key={c.id} value={c.id}>{c.name}{c.device?` · ${c.device}`:''}{c.online?'':tr(' · 离线',' · Offline')}</option>)}</Select></Field>
      <div className="edge-agent-discover"><Button loading={pending} disabled={pending||!connectors.find(c=>String(c.id)===edge)?.online} onClick={()=>void discover()}>{tr('发现已安装的 Agent','Discover installed Agents')}</Button><span>{tr('不会安装软件或更改 Codex 配置。','Does not install software or change Codex configuration.')}</span></div>
      {error&&<Notice tone="danger">{error}</Notice>}
      {discovered&&!items.length&&<Notice>{tr('没有找到此运行用户下的 Codex。','No Codex installation found for this OS user.')}</Notice>}
      {(items.length>0||installation)&&<Field label={tr('Codex 安装','Codex installation')} required><Select value={installation} disabled={pending||!items.length} onChange={e=>setInstallation(e.target.value)}>{!items.length&&<option value={installation}>{tr('已保存的安装','Saved installation')}</option>}{items.map(i=><option value={i.id} key={i.id}>{i.path}</option>)}</Select></Field>}
      <Field label={tr('项目目录','Project directory')} required hint={tr('目标设备上的绝对路径。使用该设备的 Codex 账号和模型配置。','Absolute path on the target device. Uses that device’s Codex account and model configuration.')}><div className="edge-agent-directory-path"><Input value={project} maxLength={4096} disabled={pending} placeholder="/path/to/project" onChange={e=>setProject(e.target.value)}/><Button disabled={pending||!edge} onClick={()=>setBrowsing(true)}>{tr('浏览','Browse')}</Button></div></Field>
      <Notice>{tr('当前访问为只读模式；退出页面会结束本次会话。','Access is currently read-only. Leaving the page ends the session.')}</Notice>
    </div>
  </Modal>{browsing&&<DirectoryBrowser edge={Number(edge)} accessID={entry?.edge_id===Number(edge)?entry.id:undefined} current={project} onClose={()=>setBrowsing(false)} onSelect={path=>{setProject(path);setBrowsing(false);}}/>}</>;
}
