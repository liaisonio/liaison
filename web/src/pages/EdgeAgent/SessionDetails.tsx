import {useEffect,useState} from 'react';
import {Copy} from 'lucide-react';
import {Button,Modal,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import type {AgentSnapshot,AgentConnector} from '@/services/edgeAgent';

export function SessionDetails({open,onClose,session,connector}:{open:boolean;onClose:()=>void;session?:AgentSnapshot;connector?:AgentConnector}){
  const {tr,locale}=useI18n();const [copied,setCopied]=useState(''),[error,setError]=useState(false);
  useEffect(()=>{if(!copied)return;const timer=setTimeout(()=>setCopied(''),1800);return()=>clearTimeout(timer);},[copied]);
  useEffect(()=>{setCopied('');setError(false);},[open]);
  const rows=[['Liaison Session ID',session?.session_id],['Codex Thread ID',session?.thread_id],['Codex '+tr('版本','version'),session?.agent_version],[tr('模型','Model'),session?.model],[tr('项目目录','Project directory'),session?.project],[tr('连接器','Connector'),connector?.name],[tr('设备','Device'),connector?.device],[tr('开始时间','Started'),session?.started_at?new Date(session.started_at).toLocaleString(locale):undefined]];
  return <Modal open={open} width={560} title={tr('会话详情','Session details')} onClose={onClose} footer={<Button onClick={onClose}>{tr('关闭','Close')}</Button>}>
    <dl className="edge-agent-details">{rows.map(([label,value],index)=><div className={index<2||index===4||index===7?'is-wide':''} key={label}><dt>{label}</dt><dd>{index<5?<code>{value||'—'}</code>:<span>{value||'—'}</span>}{value&&label?.endsWith('ID')&&<Button aria-label={`${tr('复制','Copy')} ${label}`} onClick={async()=>{try{await navigator.clipboard.writeText(value);setCopied(label);setError(false);}catch{setError(true);}}}><Copy size={13}/>{copied===label?tr('已复制','Copied'):tr('复制','Copy')}</Button>}</dd></div>)}</dl>
    {error&&<Notice tone="warning">{tr('无法复制，请手动选择 ID。','Cannot copy. Select the ID manually.')}</Notice>}
  </Modal>;
}
