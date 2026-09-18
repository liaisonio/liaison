import {useEffect,useRef,useState} from 'react';
import {useParams} from 'react-router-dom';
import AccessContext from '@/components/AccessContext';
import {Button,Field,Input,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import {createWebDataSession,deleteWebDataSession,getWebDataTarget} from '@/services/api';
import Files from '@/pages/WebSSH/Files';
import {useSessionPath} from '@/components/SessionReference/useSessionPath';
import {useFeature} from '@/store/permissions';
import '@/pages/WebSFTP/index.less';

export default function WebSMB(){const {proxyId,credentialId}=useParams();return <Workspace key={`${proxyId}/${credentialId}`}/>;}
function Workspace(){
 const {proxyId,credentialId}=useParams(),{tr}=useI18n();
 const allowed=useFeature('webssh.files.read');
 const [target,setTarget]=useState<API.WebDataTarget>(),[credential,setCredential]=useState<API.WebDataCredential>();
 const [session,setSession]=useState<{token:string;expires_at:string}>(),[password,setPassword]=useState(''),[busy,setBusy]=useState(true),[error,setError]=useState(false),[retry,setRetry]=useState(0);
 const alive=useRef(false),generation=useRef(0),opened=useRef('');
 useSessionPath(session?.token||'',false,()=>{});
 useEffect(()=>{
  alive.current=true;generation.current++;let current=true;setBusy(true);setError(false);setSession(undefined);
  if(!allowed){setBusy(false);return;}
  (async()=>{
   const result=await getWebDataTarget(Number(proxyId));if(!current)return;
   const data=result.data,c=data?.credentials?.find(item=>item.id===Number(credentialId));
   if(result.code!==200||data?.protocol!=='smb'||!c)throw Error('target');
   setTarget(data);setCredential(c);if(c.saved)await connect(c,'');else setBusy(false);
  })().catch(()=>{if(current){setError(true);setBusy(false);}});
  return()=>{current=false;alive.current=false;generation.current++;if(opened.current){void deleteWebDataSession(opened.current).catch(()=>{});opened.current='';}};
 },[proxyId,credentialId,retry,allowed]);
 async function connect(c:API.WebDataCredential,secret:string){
  const run=generation.current;setBusy(true);setError(false);
  try{
   const result=await createWebDataSession(Number(proxyId),{...(c.saved?{credential_id:c.id}:{username:c.username,database:c.database,schema:c.schema,tls_mode:'disable'}),protocol:'smb',password:secret});
   if(result.code!==200||!result.data?.token)throw Error('connect');
   if(!alive.current||run!==generation.current){void deleteWebDataSession(result.data.token).catch(()=>{});return;}
   opened.current=result.data.token;setSession(result.data);setPassword('');
  }catch{if(alive.current&&run===generation.current)setError(true);}finally{if(alive.current&&run===generation.current)setBusy(false);}
 }
 return <div className="websftp-workspace">
  <AccessContext name={target?.proxy_name||'WebSMB'} protocol="WebSMB" target={target?`${target.target_host}:${target.target_port}`:undefined}/>
  {!allowed?<Notice>{tr('没有文件浏览权限。','File browsing is not permitted.')}</Notice>:<>
   <Notice>{tr('只读共享：支持目录浏览、文本预览和下载。','Read-only share: browse directories, preview text and download files.')}</Notice>
   {error&&<Notice tone="danger">{tr('无法连接 SMB，请检查用户名、域、共享名和权限。','Unable to connect to SMB. Check the username, domain, share name and permissions.')} <Button onClick={()=>setRetry(n=>n+1)}>{tr('重试','Retry')}</Button></Notice>}
   {session?<div className="websftp-browser"><Files key={session.token} proxyId={Number(proxyId)} username={credential?.username||''} saved={!!credential?.saved} standalone initialSession={{id:session.token,home:'/',can_upload:false,expires_at:session.expires_at}} apiBase={`/api/v1/webdata/sessions/${encodeURIComponent(session.token)}/smb`} closeSessionURL={`/api/v1/webdata/sessions/${encodeURIComponent(session.token)}`} onClose={()=>setRetry(n=>n+1)}/></div>:busy?<div role="status">{tr('正在连接 SMB…','Connecting to SMB…')}</div>:credential&&!credential.saved?<form className="native-modal-form" onSubmit={e=>{e.preventDefault();void connect(credential,password);}}>
    <Field label={tr('用户名','Username')}><Input readOnly value={credential.username}/></Field>
    <Field label={tr('密码','Password')} hint={tr('仅用于本次会话，不保存。','Used for this session only; not saved.')}><Input autoFocus type="password" autoComplete="off" value={password} onChange={e=>setPassword(e.target.value)}/></Field>
    <Button type="submit" variant="primary" disabled={!password||busy}>{tr('连接','Connect')}</Button>
   </form>:null}
  </>}
 </div>;
}
