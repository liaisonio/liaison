import {useEffect, useState} from 'react';
import {Button, Field, Notice, Select} from '@/components/ui';
import {getEdgeList} from '@/services/api';
import {useI18n} from '@/i18n';
import {accessTypeLabel} from '@/constants/accessTypes';

export default function LLMApplicationPicker({edge,application,applications,protocol,disabled,onEdge,onApplication}:{edge:string;application:string;applications:API.Application[];protocol:string;disabled:boolean;onEdge:(id:string)=>void;onApplication:(id:string)=>void}) {
  const {tr}=useI18n();
  const [edges,setEdges]=useState<API.Edge[]>([]),[loading,setLoading]=useState(true),[failed,setFailed]=useState(false),[retry,setRetry]=useState(0);
  const isNew=application==='new';
  useEffect(()=>{if(!isNew)return;let alive=true;setLoading(true);setFailed(false);getEdgeList({page_size:1000}).then(r=>{
    if(r.code!==200)throw Error('edges');if(alive)setEdges(r.data?.edges||[]);
  }).catch(()=>{if(alive)setFailed(true);}).finally(()=>{if(alive)setLoading(false);});return()=>{alive=false;};},[retry,isNew]);
  return <>
    <Field label={tr('应用来源','Application source')} required><Select aria-label={tr('应用来源','Application source')} value={isNew?'new':'existing'} disabled={disabled} onChange={e=>onApplication(e.target.value==='new'?'new':'')}>
      <option value="existing">{tr('已有应用','Existing application')}</option>
      <option value="new">{tr('新建应用','New application')}</option>
    </Select></Field>
    {isNew?<>
    <Field label={tr('连接器','Connector')} required><Select aria-label={tr('连接器','Connector')} value={edge} disabled={disabled||loading||failed} onChange={e=>onEdge(e.target.value)}>
      <option value="">{loading?tr('正在加载…','Loading…'):tr('选择连接器','Select connector')}</option>
      {edges.map(e=><option key={e.id} value={e.id}>{e.name} · {e.device?.name||tr('设备未上报','Device not reported')} ({e.online===1?tr('在线','Online'):tr('离线','Offline')})</option>)}
    </Select></Field>
    {failed&&<Notice tone="danger">{tr('连接器加载失败','Could not load connectors')} <Button onClick={()=>setRetry(n=>n+1)}>{tr('重试','Retry')}</Button></Notice>}
    {!loading&&!failed&&!edges.length&&<Notice><a href="/connector?create=1">{tr('创建连接器','Create connector')}</a></Notice>}
    </>:<>
      <Field label={tr('应用','Application')} required><Select aria-label={tr('应用','Application')} value={application} disabled={disabled} onChange={e=>onApplication(e.target.value)}>
        <option value="">{tr('选择应用','Select application')}</option>
        {applications.map(a=><option key={a.id} value={a.id}>{a.name} · {a.ip}:{a.port}</option>)}
      </Select></Field>
      {!applications.length&&<p className="liaison-access-route-note">{tr(`暂无 ${accessTypeLabel(protocol)} 应用，可选择新建应用。`,`No ${accessTypeLabel(protocol)} applications. Select New application to add one.`)}</p>}
    </>}
  </>;
}
