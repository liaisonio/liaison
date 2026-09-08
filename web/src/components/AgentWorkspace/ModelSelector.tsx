import {useEffect,useState} from 'react';
import {ChevronDown} from 'lucide-react';
import {useI18n} from '@/i18n';
import {getAgentStatus, type AgentModelChoice, type AgentModelSelection} from '@/services/agent';
import './ModelSelector.less';

export default function ModelSelector({value,onChange,disabled}:{value?:AgentModelSelection;onChange:(value:AgentModelSelection|undefined)=>void;disabled?:boolean}) {
 const {tr}=useI18n();
 const [choices,setChoices]=useState<AgentModelChoice[]>([]);
 const [failed,setFailed]=useState(false);
 const [loaded,setLoaded]=useState(false);
 useEffect(()=>{let active=true;void getAgentStatus().then(r=>{if(active){setChoices(r.data?.models||[]);setLoaded(true);}}).catch(()=>{if(active){setFailed(true);setLoaded(true);}});return()=>{active=false;};},[]);
 const key=(m:AgentModelSelection)=>JSON.stringify([m.provider_id,m.model]);
 const current=value?key(value):'';
 const unavailable=!!value&&!choices.some(m=>key(m)===current);
 const groups=[...new Set(choices.map(m=>m.provider_id))];
 const fallback=choices.find(m=>m.is_default);
 return <label className="agent-model-selector" title={tr('选择本次对话使用的模型','Choose a model for this conversation')}>
  <select aria-label={tr('对话模型','Conversation model')} value={current} disabled={disabled||!loaded||failed||!choices.length} onChange={e=>{const selected=choices.find(m=>key(m)===e.target.value);onChange(selected?{provider_id:selected.provider_id,model:selected.model}:undefined);}}>
   <option value="">{!loaded?tr('加载模型…','Loading models…'):failed?tr('模型加载失败','Models unavailable'):fallback?`${fallback.model} · ${tr('默认','Default')}`:tr('系统默认模型','System default')}</option>
   {unavailable&&<option value={current} disabled>{value.model} · {tr('不可用，请重新选择','Unavailable; select another')}</option>}
   {groups.map(id=><optgroup key={id} label={id}>{choices.filter(m=>m.provider_id===id).map(m=><option key={key(m)} value={key(m)}>{m.model}</option>)}</optgroup>)}
  </select><ChevronDown size={13} aria-hidden="true"/>
 </label>;
}
