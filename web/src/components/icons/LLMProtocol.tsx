import {llmLabel} from '@/constants/llmProtocols';
import {ProviderIcon} from '@/components/AgentWorkspace/ProviderIcon';
import {useI18n} from '@/i18n';
import qwenLogo from '../../../../docs/assets/integrations/qwen.svg';
import arkLogo from '../../../../docs/assets/integrations/ark.svg';
import ollamaLogo from './ollama.svg';

export default function LLMProtocol({protocol}:{protocol?:string}) {
  const {tr}=useI18n();
  if(protocol==='ark'||protocol==='qwen')return <span className="liaison-inline-name"><img src={protocol==='ark'?arkLogo:qwenLogo} alt="" width={18} height={18}/><span>{llmLabel(protocol)}</span></span>;
  if(protocol==='ollama')return <span className="liaison-inline-name" title="Ollama native API"><span aria-hidden style={{display:'inline-block',width:18,height:18,flexShrink:0,background:'currentColor',mask:`url(${ollamaLogo}) center / contain no-repeat`,WebkitMask:`url(${ollamaLogo}) center / contain no-repeat`}}/><span>Ollama</span></span>;
  const provider=protocol==='openai-compatible'||protocol==='openai'?'openai':protocol==='anthropic'?'anthropic':protocol==='gemini'?'gemini':undefined;
  const label=protocol?llmLabel(protocol):tr('未配置','Not configured');
  return <span className="liaison-inline-name" title={provider==='openai'?'OpenAI-compatible':provider==='anthropic'?'Anthropic':undefined}>
    {provider&&<ProviderIcon provider={provider} size={18}/>}
    <span>{label}</span>
  </span>;
}
