import {useEffect, useRef, useState} from 'react';
import {Plus, Server} from 'lucide-react';
import ProtocolIcon from '@/components/icons/ProtocolIcon';
import {Button, Field, Modal, Notice, Select, StatusPill} from '@/components/ui';
import {useI18n} from '@/i18n';
import {APPLICATION_TYPES} from '@/constants/applicationTypes';
import {llmBase, protocolFamily} from '@/constants/llmProtocols';
import {getApplicationList} from '@/services/api';
import {history} from '@/lib/runtime';
import LLMApplicationDraft, {type LLMDraftHandle} from '@/pages/Proxy/LLMApplicationDraft';
import './modelDiscovery.less';
import '@/pages/Proxy/connection.less';

export function DiscoveryRow({target, info, fallbackType, onAdd, onModel}: {
  target:string; info?:API.DiscoveredModelAPI; fallbackType:string;
  onAdd:()=>void; onModel:(info:API.DiscoveredModelAPI)=>void;
}) {
  const {tr}=useI18n();
  const [ip,port]=target.split(':');
  const verified=info?.state==='verified';
  return <div className="liaison-scan-row liaison-model-discovery-row">
    <span className="liaison-scan-target"><i>{info?<ProtocolIcon protocol={info.service==='ollama'?'ollama':'llm'}/>:<Server size={14}/>}</i><span>
      <strong className={info?.service==='ollama'?'liaison-model-service-name':undefined}>{info?.service==='ollama'?'Ollama':ip}</strong>
      <small>{info?.service==='ollama'?`${ip}:${port}`:`${tr('端口','Port')} ${port}`}</small>
    </span></span>
    <div className="liaison-model-discovery-evidence">
      {info?<>
        {verified?(info.protocols?.length?info.protocols:['llm']).map(p=><StatusPill key={p}>{p==='ollama'?'Ollama':p==='llm'?'LLM':tr('OpenAI 兼容','OpenAI compatible')}</StatusPill>):<StatusPill>{tr('待验证','Unverified')}</StatusPill>}
      </>:<StatusPill>{APPLICATION_TYPES.find(t=>t.value===fallbackType)?.label||'TCP'}</StatusPill>}
    </div>
    <button type="button" className="liaison-table-link" onClick={()=>info?onModel(info):onAdd()}><Plus size={13}/>{tr('添加','Add')}</button>
  </div>;
}

export function DiscoveryServices({applications=[],modelAPIs=[],status,statusLabel,onAdd,onModel,portTypes={}}: {
  applications?:string[];modelAPIs?:API.DiscoveredModelAPI[];status?:string;statusLabel:string;
  onAdd:(target:string)=>void;onModel:(info:API.DiscoveredModelAPI)=>void;portTypes?:Record<number,string>;
}) {
  const {tr}=useI18n();
  const models=Array.from(new Map(modelAPIs.map(info=>[`${info.ip}:${info.port}`,info])).values());
  const network=applications.filter(target=>{const [ip,port]=target.split(':');return !models.some(info=>info.ip===ip&&Number(info.port)===Number(port));});
  const tone=status==='completed'?'success':status==='failed'?'danger':'info';
  return <>{[{key:'llm',label:'LLM',count:models.length},{key:'network',label:tr('网络服务','Network services'),count:network.length}].map(group=><section key={group.key} aria-label={group.label}>
    <div className="liaison-scan-toolbar"><span>{group.label} <b>{group.count}</b></span><StatusPill tone={tone}>{statusLabel}</StatusPill></div>
    <div className="liaison-scan-list">
      <div className="liaison-scan-list-head"><span>{tr('目标服务','Target')}</span><span>{tr('协议','Protocol')}</span><span/></div>
      {group.key==='llm'?models.map(info=><DiscoveryRow key={`${info.ip}:${info.port}`} target={`${info.ip}:${info.port}`} info={info} fallbackType="http" onAdd={()=>{}} onModel={onModel}/>):network.map(target=><DiscoveryRow key={target} target={target} fallbackType={target.split(':')[2]||portTypes[Number(target.split(':')[1])]||'tcp'} onAdd={()=>onAdd(target)} onModel={onModel}/>)}
      {!group.count&&<div className="liaison-scan-empty" role="status">{status==='completed'?(group.key==='llm'?tr('未发现模型服务','No model services found'):tr('未发现网络服务','No network services found')):status==='failed'?tr('探测未完成，请重新探测。','Discovery failed. Try again.'):tr('正在探测…','Discovering…')}</div>}
    </div>
  </section>)}</>;
}

export function ModelDiscoverySetup({edge,info,onClose}: {edge:number;info:API.DiscoveredModelAPI;onClose:()=>void}) {
  const {tr}=useI18n();
  const [protocol,setProtocol]=useState(protocolFamily(info.protocols?.[0])||'openai');
  const [ready,setReady]=useState(false),[saving,setSaving]=useState(false),[error,setError]=useState('');
  const [applications,setApplications]=useState<API.Application[]>([]);
  const draft=useRef<LLMDraftHandle>(null);
  useEffect(()=>{
    let alive=true;
    getApplicationList({page:1,page_size:1000}).then(r=>{if(alive&&r.code===200)setApplications(r.data?.applications||[]);}).catch(()=>{/* Creation also detects duplicates on the server. */});
    return()=>{alive=false;};
  },[]);
  return <Modal open title={tr('创建模型应用和访问','Create model application and access')} width={520} onClose={()=>{if(!saving)onClose();}} closeOnMask={!saving}
    footer={<><Button disabled={saving} onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={!ready||saving} loading={saving} onClick={async()=>{
      if(!ready||saving||!draft.current)return;
      setSaving(true);setError('');
      try{await draft.current.create('','');onClose();history.push('/proxy?category=llm');}
      catch(e){setError(e instanceof Error?e.message:tr('创建失败，请重试。','Creation failed. Try again.'));}
      finally{setSaving(false);}
    }}>{tr('创建应用和访问','Create application and access')}</Button></>}>
    <div className="liaison-model-discovery-setup">
      {info.state==='auth_required'&&<Notice>{tr('端口和认证响应仅提示可能是模型服务，请选择协议、填写密钥并验证。','The port and authentication response only suggest a model service. Select the protocol, enter a key and verify.')}</Notice>}
      <Field label={tr('上游协议','Upstream protocol')}><Select value={protocol} disabled={saving} onChange={e=>{setProtocol(e.target.value);setReady(false);setError('');}}><option value="openai">{tr('OpenAI 兼容','OpenAI compatible')}</option><option value="ollama">Ollama</option></Select></Field>
      <LLMApplicationDraft key={protocol} ref={draft} edge={String(edge)} protocol={protocol} applications={applications} disabled={saving} onReady={setReady} autoProbe initialName={info.service==='ollama'?'Ollama':tr('模型服务','Model service')} authenticationRequired={info.state==='auth_required'}
        initialAddress={`http://${info.ip}:${info.port}${llmBase(protocol)}`} onUse={id=>{onClose();history.push(`/proxy?category=llm&new_application=${encodeURIComponent(id)}`);}}/>
      <small>{tr('同时创建同名的 LLM 访问，仅开放所选模型。','Creates an LLM access with the same name, exposing only selected models.')}</small>
      {error&&<Notice tone="danger">{error}</Notice>}
    </div>
  </Modal>;
}
