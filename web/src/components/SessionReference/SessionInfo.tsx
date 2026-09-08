import {useRef} from 'react';
import {createPortal} from 'react-dom';
import {Info,X} from 'lucide-react';
import {useI18n} from '@/i18n';
import SessionReference from './index';

export default function SessionInfo({connectionId,handle,agentId}:{connectionId?:string;handle?:string;agentId?:string}){
 const {tr}=useI18n();const dialog=useRef<HTMLDialogElement>(null);
 if(!connectionId&&!handle&&!agentId)return null;
 return <><button type="button" className="session-info-trigger" aria-label={tr('会话信息','Session info')} title={tr('会话信息','Session info')} onClick={()=>dialog.current?.showModal()}><Info size={15}/></button>
 {createPortal(<dialog ref={dialog} className="session-info-dialog" aria-label={tr('会话信息','Session info')} onClick={e=>{if(e.target===e.currentTarget){const r=e.currentTarget.getBoundingClientRect();if(e.clientX<r.left||e.clientX>r.right||e.clientY<r.top||e.clientY>r.bottom)dialog.current?.close();}}}>
 <header><strong>{tr('会话信息','Session info')}</strong><button type="button" aria-label={tr('关闭会话信息','Close session info')} onClick={()=>dialog.current?.close()}><X size={17}/></button></header>
 <SessionReference id={connectionId} handle={handle} label={tr('连接会话','Connection')}/>
 <SessionReference id={agentId} label={tr('Agent 会话','Agent session')}/>
 <p>{tr('排查问题时可复制这些 ID，不包含连接密码或令牌。','Copy these IDs for troubleshooting. They contain no passwords or connection tokens.')}</p>
 </dialog>,document.body)}</>;
}
