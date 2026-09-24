import {useEffect, useId, useState} from 'react';
import {ArrowRight, CircleHelp, Sparkles} from 'lucide-react';
import {EmptyAccessLayout} from './AccessEmptyState';
import {Button} from '@/components/ui';
import {isLLMApplicationType} from '@/constants/llmProtocols';
import {useI18n} from '@/i18n';
import {history} from '@/lib/runtime';
import {getApplicationList, getEdgeList} from '@/services/api';

export default function LLMEmptyState({onCreate}: {onCreate: () => void}) {
  const {tr} = useI18n();
  const [help,setHelp]=useState(false);const helpId=useId();
  const [step, setStep] = useState<'loading' | 'error' | 'connector' | 'application' | 'access'>('loading');
  const [attempt, setAttempt] = useState(0);
  useEffect(() => {
    let cancelled = false;
    setStep('loading');
    Promise.all([getEdgeList({page_size: 1}), getApplicationList({page_size: 1000})]).then(([edges, apps]) => {
      if (cancelled) return;
      if (edges.code !== 200 || apps.code !== 200) { setStep('error'); return; }
      setStep(apps.data?.applications?.some(app => isLLMApplicationType(app.application_type)) ? 'access' : edges.data?.edges?.length ? 'application' : 'connector');
    }).catch(() => { if (!cancelled) setStep('error'); });
    return () => { cancelled = true; };
  }, [attempt]);
  const label = step === 'connector' ? tr('创建连接器', 'Create connector') : step === 'application' ? tr('添加模型应用', 'Add model application') : tr('新建访问', 'Create access');
  return <EmptyAccessLayout className="liaison-llm-empty" icon={<Sparkles size={20}/>} title={<><h3>{tr('接入本地模型', 'Connect a local model')}</h3><span className="liaison-access-empty-help" onMouseEnter={()=>setHelp(true)} onMouseLeave={()=>setHelp(false)}><button type="button" aria-label={tr('与助理模型有什么区别？','How is this different from assistant models?')} aria-describedby={help?helpId:undefined} onFocus={()=>setHelp(true)} onBlur={()=>setHelp(false)} onClick={()=>setHelp(true)} onKeyDown={e=>{if(e.key==='Escape')setHelp(false);}}><CircleHelp size={14}/></button>{help&&<span id={helpId} role="tooltip">{tr('这里的模型 API 供客户端调用；设置中的模型供产品内助理使用。', 'These model APIs serve your clients. Models in Settings power the built-in assistants.')}</span>}</span></>} description={tr('通过连接器连接模型服务，创建可供客户端调用的 API。', 'Reach your model service through a connector and expose an API for your clients.')}>
    {step === 'loading' ? <span role="status">{tr('正在检查配置…', 'Checking setup…')}</span> : step === 'error' ? <><span role="alert">{tr('无法获取配置', 'Unable to load setup')}</span><Button onClick={() => setAttempt(value => value + 1)}>{tr('重试', 'Retry')}</Button></> : <Button variant="primary" onClick={() => step === 'connector' ? history.push('/connector?create=1') : onCreate()}>{label}<ArrowRight size={14}/></Button>}
  </EmptyAccessLayout>;
}
