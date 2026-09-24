import {useEffect,useState} from 'react';
import {Button,Field,Modal,Notice,Select} from '@/components/ui';
import {useI18n} from '@/i18n';
import {edgeAgent,type AgentAccess,type AgentModel,type AgentSnapshot} from '@/services/edgeAgent';

export function ModelPicker({access,session,onClose,onChange}:{access:AgentAccess;session:AgentSnapshot;onClose:()=>void;onChange:(s:AgentSnapshot)=>void}){
 const {tr}=useI18n();
 const [models,setModels]=useState<AgentModel[]>([]),[selected,setSelected]=useState(session.model||''),[loading,setLoading]=useState(true),[saving,setSaving]=useState(false),[error,setError]=useState(''),[retry,setRetry]=useState(0);
 useEffect(()=>{
  const abort=new AbortController();setLoading(true);setError('');
  void edgeAgent(access.edge_id,'models',{access_id:access.id,session_id:session.session_id!},abort.signal).then(data=>{
   if(data.status!=='ok'||!data.models_available)throw new Error('unavailable');
   setModels(data.models||[]);
  }).catch(()=>{if(!abort.signal.aborted)setError(tr('无法获取 Codex 模型列表，请检查连接器版本或稍后重试。','Cannot load Codex models. Check the connector version or retry.'));}).finally(()=>{if(!abort.signal.aborted)setLoading(false);});
  return()=>abort.abort();
 },[retry]);
 async function save(){
  if(saving)return;setSaving(true);setError('');
  try{
   const next=await edgeAgent(access.edge_id,'model',{access_id:access.id,session_id:session.session_id!,model:selected});
   if(next.status!=='ok'||!next.session_id||next.model!==selected)throw new Error('failed');
   onChange(next);onClose();
  }catch{setError(tr('模型未切换。请等待当前回复结束后重试。','Model was not changed. Wait for the current response to finish and retry.'));}
  finally{setSaving(false);}
 }
 return <Modal open title={tr('切换模型','Switch model')} onClose={()=>{if(!saving)onClose();}} footer={<><Button disabled={saving} onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" loading={saving} disabled={loading||saving||!models.some(m=>m.id===selected)} onClick={()=>void save()}>{tr('切换','Switch')}</Button></>}>
  {error&&<Notice tone="danger">{error}<Button disabled={saving} onClick={()=>setRetry(n=>n+1)}>{tr('重试','Retry')}</Button></Notice>}
  {loading?<Notice>{tr('正在读取 Codex 模型…','Loading Codex models…')}</Notice>:<Field label={tr('模型','Model')}><Select value={selected} onChange={e=>setSelected(e.target.value)} disabled={saving}>
   {!models.some(m=>m.id===selected)&&<option value={selected}>{selected||tr('请选择模型','Select model')}</option>}
   {models.map(m=><option key={m.id} value={m.id}>{m.name} · {m.id}</option>)}
  </Select></Field>}
  {!loading&&!error&&!models.length&&<Notice>{tr('Codex 没有返回可切换的模型。','Codex did not return any selectable models.')}</Notice>}
  <p className="edge-agent-model-note">{tr('从下一条消息生效，仅影响本次会话，不修改 Codex 本机默认配置。','Applies from the next message to this conversation only. Codex local defaults are unchanged.')}</p>
 </Modal>;
}
