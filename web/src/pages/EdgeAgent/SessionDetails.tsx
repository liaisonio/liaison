import {useEffect,useState} from 'react';
import {Copy} from 'lucide-react';
import {Button,Modal,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import type {AgentSnapshot,AgentConnector} from '@/services/edgeAgent';
import type {TurnTiming} from './turnTiming';

export function SessionDetails({open,onClose,session,connector,timing}:{open:boolean;onClose:()=>void;session?:AgentSnapshot;connector?:AgentConnector;timing?:TurnTiming}){
  const {tr,locale}=useI18n();const [copied,setCopied]=useState(''),[error,setError]=useState(false);
  useEffect(()=>{if(!copied)return;const timer=setTimeout(()=>setCopied(''),1800);return()=>clearTimeout(timer);},[copied]);
  useEffect(()=>{setCopied('');setError(false);},[open]);
  const rows=[['Liaison Session ID',session?.session_id],['Codex Thread ID',session?.thread_id],['Codex '+tr('版本','version'),session?.agent_version],[tr('模型','Model'),session?.model],[tr('项目目录','Project directory'),session?.project],[tr('连接器','Connector'),connector?.name],[tr('设备','Device'),connector?.device],[tr('开始时间','Started'),session?.started_at?new Date(session.started_at).toLocaleString(locale):undefined]];
  const duration=(value?:number)=>value===undefined?'—':`${(value/1000).toFixed(2)} s`;
  return <Modal open={open} width={560} title={tr('会话详情','Session details')} onClose={onClose} footer={<Button onClick={onClose}>{tr('关闭','Close')}</Button>}>
    <dl className="edge-agent-details">{rows.map(([label,value],index)=><div className={index<2||index===4||index===7?'is-wide':''} key={label}><dt>{label}</dt><dd>{index<5?<code>{value||'—'}</code>:<span>{value||'—'}</span>}{value&&label?.endsWith('ID')&&<Button aria-label={`${tr('复制','Copy')} ${label}`} onClick={async()=>{try{await navigator.clipboard.writeText(value);setCopied(label);setError(false);}catch{setError(true);}}}><Copy size={13}/>{copied===label?tr('已复制','Copied'):tr('复制','Copy')}</Button>}</dd></div>)}</dl>
    {timing&&<section className="edge-agent-timing" aria-label={tr('本次发送耗时','Send timing')}><strong>{tr('最近一次发送 · 浏览器观测','Last send · Browser observations')}</strong><dl className="edge-agent-details">
      <div><dt>{tr('发送请求往返','Send request round trip')}</dt><dd><code>{duration(timing.requestMs)}</code></dd></div>
      <div><dt>{tr('首次收到回复文字','First reply text received')}</dt><dd><code>{duration(timing.firstReplyMs)}</code></dd></div>
      <div><dt>{tr('观察到本轮结束','Turn end observed')}</dt><dd><code>{duration(timing.finishedMs)}</code></dd></div>
      <div><dt>{tr('状态','Status')}</dt><dd>{timing.state==='unconfirmed'?tr('发送结果未确认','Send outcome unconfirmed'):timing.state==='completed'?tr('本轮已结束','Turn ended'):tr('等待本轮结束','Awaiting turn end')}</dd></div>
    </dl><p>{tr('均从点击发送计时，包含传输与服务处理，结束耗时还可能包含工具执行及等待授权。尚未分别采集网络、Edge、App Server 和模型耗时。仅保留本次页面内的观测。','Measured from Send, including transport and service processing. Turn duration may include tools and approval waits. Network, Edge, App Server and model timings are not measured separately. Observations last only for this workspace view.')}</p>{timing.background&&<Notice tone="warning">{tr('本轮期间标签页曾在后台，观测可能延迟。','This tab was in the background during this turn; observations may be delayed.')}</Notice>}</section>}
    {session&&<Notice>{session.history_persistent?tr('历史保存在服务端，不自动过期。空闲时仅结束运行实例。','History is stored on the server without automatic expiration. Idle instances may stop.'):tr('当前会话使用临时历史，请升级服务端启用持久化。','This conversation uses temporary history. Upgrade the server to enable persistence.')}</Notice>}
    {error&&<Notice tone="warning">{tr('无法复制，请手动选择 ID。','Cannot copy. Select the ID manually.')}</Notice>}
  </Modal>;
}
