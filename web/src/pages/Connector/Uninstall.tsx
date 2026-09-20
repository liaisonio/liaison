import {request} from '@/api/client';
import {Button, DangerConfirm, Field, Input, Modal, Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import {useEffect, useRef, useState} from 'react';

type Installation = {can_uninstall:boolean; instance_id?:string; service?:string; reason:string};
type Task = {id:string; status:'accepted'|'running'|'completed'|'failed'|'unknown'; expires_at:string};

export default function Uninstall({edge,onClose,onComplete}:{edge:API.Edge;onClose:()=>void;onComplete:()=>void}) {
 const {tr}=useI18n();
 const [installation,setInstallation]=useState<Installation>();
 const [task,setTask]=useState<Task>();
 const [loading,setLoading]=useState(true),[saving,setSaving]=useState(false);
 const [name,setName]=useState(''),[error,setError]=useState('');
 const busy=useRef(false),completed=useRef(false);
 const callback=useRef(onComplete);callback.current=onComplete;
 useEffect(()=>{
  const controller=new AbortController();
  (async()=>{
   try {
    const active=await request<API.Response<Task|null>>(`/api/v1/edges/${edge.id}/uninstall/active`,{signal:controller.signal});
    if(active.code!==200)throw Error();
    if(active.data){setTask(active.data);return}
    const response=await request<API.Response<Installation>>(`/api/v1/edges/${edge.id}/installation`,{signal:controller.signal});
    if(response.code!==200||!response.data)throw Error();setInstallation(response.data);
   }catch{if(!controller.signal.aborted)setError(tr('无法检查安装状态，请稍后重试。','Could not check the installation. Try again later.'))}
   finally{if(!controller.signal.aborted)setLoading(false)}
  })();return()=>controller.abort();
 },[edge.id,tr]);
 useEffect(()=>{
  if(!task||task.status==='completed'||task.status==='failed'||(task.status==='unknown'&&Date.now()>=Date.parse(task.expires_at)))return;
  const controller=new AbortController();let timer:number|undefined;
  const poll=async()=>{
   try{
    const response=await request<API.Response<Task>>(`/api/v1/edges/${edge.id}/uninstall/${task.id}`,{signal:controller.signal});
    if(response.code!==200||!response.data)throw Error();
    if(controller.signal.aborted)return;
    setError('');setTask(response.data);
    if(response.data.status==='completed'&&!completed.current){completed.current=true;callback.current()}
    if(['completed','failed'].includes(response.data.status)||(response.data.status==='unknown'&&Date.now()>=Date.parse(response.data.expires_at)))return;
   }catch{
    if(controller.signal.aborted)return;
    setError(tr('暂时无法获取结果，不会重复发送卸载。','Result temporarily unavailable. Uninstall will not be sent again.'));
    if(Date.now()>=Date.parse(task.expires_at)){setTask({...task,status:'unknown'});return}
   }
   timer=window.setTimeout(poll,2000);
  };
  timer=window.setTimeout(poll,1000);
  return()=>{controller.abort();window.clearTimeout(timer)};
 },[edge.id,task?.id,task?.status,tr]);
 const submit=async()=>{
  if(busy.current||name!==edge.name||!installation?.can_uninstall||!installation.instance_id||task)return;
  busy.current=true;setSaving(true);setError('');
  try{
   const response=await request<API.Response<Task>>(`/api/v1/edges/${edge.id}/uninstall`,{method:'POST',data:{confirm_name:name,instance_id:installation.instance_id}});
   if(response.code!==202||!response.data)throw Error();setTask(response.data);
  }catch{
   // Lost HTTP acknowledgements are ambiguous. Reopen to query the durable task;
   // do not automatically enable a second destructive submission.
   setError(tr('提交结果未确认，请关闭后重新打开查看状态。','Submission unconfirmed. Close and reopen to check its status.'));
   setInstallation(undefined);
  }finally{setSaving(false)}
 };
 const reason=installation?.reason==='connector_offline'?tr('连接器离线，无法卸载。','Connector is offline and cannot be uninstalled.')
  :installation?.reason==='uninstall_executor_unavailable'?tr('此安装未启用远程卸载。','Remote uninstall is not enabled for this installation.')
  :tr('无法确认此安装可安全卸载，请在目标设备上处理。','This installation could not be verified for safe uninstall. Manage it on the target device.');
 const status=task?.status==='completed'?tr('卸载完成','Uninstalled'):task?.status==='failed'?tr('卸载未完成，请检查目标设备。','Uninstall did not complete. Check the target device.')
  :task?.status==='unknown'?tr('结果未确认，请检查目标设备。不会自动重试。','Result unconfirmed. Check the target device. No automatic retry will occur.')
  :task?.status==='running'?tr('正在卸载…','Uninstalling…'):tr('任务已提交，等待执行…','Task submitted. Waiting for execution…');
 return <Modal open title={tr('卸载连接器','Uninstall connector')} width={500} closeOnMask={false} onClose={()=>{if(!saving)onClose()}} footer={<>
  <Button onClick={onClose} disabled={saving}>{task?tr('关闭','Close'):tr('取消','Cancel')}</Button>
  {!task&&installation?.can_uninstall&&<Button variant="danger" loading={saving} disabled={saving||name!==edge.name} onClick={()=>void submit()}>{tr('卸载','Uninstall')}</Button>}
 </>}><div style={{display:'grid',gap:16,minWidth:0,overflowWrap:'anywhere'}}>
  <DangerConfirm title={edge.name} description={tr('停止并移除此连接器的本机程序、配置及服务，撤销接入凭据。应用、访问记录与日志保留。','Stop and remove this connector’s local program, configuration and service, and revoke its credentials. Applications, access records and logs are retained.')}/>
  {loading&&<p role="status">{tr('检查安装状态…','Checking installation…')}</p>}
  {error&&<Notice tone="danger">{error}</Notice>}
  {task&&<div role="status"><Notice tone={task.status==='completed'?'success':task.status==='failed'?'danger':'info'}>{status}</Notice></div>}
  {!task&&installation&&!installation.can_uninstall&&<Notice>{reason}</Notice>}
  {!task&&installation?.can_uninstall&&<Field label={tr('输入连接器名称以确认','Enter the connector name to confirm')}><Input autoComplete="off" value={name} onChange={e=>setName(e.target.value)}/></Field>}
 </div></Modal>;
}
