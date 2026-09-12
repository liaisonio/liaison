import {useEffect,useState} from 'react';
import BrandSelect from './BrandSelect';
import {ModelIcon} from './ProviderIcon';
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
 const options=[{value:'',label:!loaded?tr('加载模型…','Loading models…'):failed?tr('模型加载失败','Models unavailable'):fallback?`${fallback.model} · ${tr('默认','Default')}`:tr('系统默认模型','System default'),icon:<ModelIcon model={fallback?.model||''} provider={fallback?.provider_type}/>},
 ...(unavailable?[{value:current,label:`${value.model} · ${tr('不可用','Unavailable')}`,disabled:true,icon:<ModelIcon model={value.model} provider={value.provider_id}/>}]:[]),
 ...groups.flatMap(id=>choices.filter(m=>m.provider_id===id).map(m=>({value:key(m),label:`${m.model} · ${id}`,icon:<ModelIcon model={m.model} provider={m.provider_type}/>})))];
 return <span className="agent-model-selector"><BrandSelect label={tr('对话模型','Conversation model')} value={current} options={options} disabled={disabled||!loaded||failed||!choices.length} onChange={v=>{const selected=choices.find(m=>key(m)===v);onChange(selected?{provider_id:selected.provider_id,model:selected.model}:undefined);}}/></span>;
}
