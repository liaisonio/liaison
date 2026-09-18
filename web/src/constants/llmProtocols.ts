export const LLM_PROTOCOLS = [
  {value:'openai',label:'OpenAI',base:'/v1'},
  {value:'anthropic',label:'Anthropic',base:'/v1'},
  {value:'ark',label:'Ark',base:'/api/v3'},
  {value:'qwen',label:'QWen',base:'/api/v1'},
  {value:'gemini',label:'Gemini',base:'/v1beta'},
  {value:'ollama',label:'Ollama',base:'/api'},
  {value:'openai-compatible',label:'OpenAI',base:'/v1'},
] as const;
// Retain legacy wire-profile IDs, but expose a single OpenAI protocol family.
export const protocolFamily=(type?:string|null)=>type==='openai-compatible'?'openai':type||undefined;
export const LLM_PROTOCOL_OPTIONS=LLM_PROTOCOLS.filter(p=>p.value!=='openai-compatible');
export const uniqueProtocolFamilies=(types:string[])=>[...new Set(types.map(p=>protocolFamily(p)!))];
export const requestAPILabel=(type:string)=>type==='openai-compatible'?'OpenAI · Chat Completions':type==='openai'?'OpenAI · Responses':type==='ark'?'Ark · Responses':type==='anthropic'?'Anthropic · Messages':llmLabel(type);
export const isLLMApplicationType=(type?:string)=>type==='llm'||LLM_PROTOCOLS.some(p=>p.value===type);
export const llmLabel=(type?:string)=>LLM_PROTOCOLS.find(p=>p.value===type)?.label||type||'—';
export const llmBase=(type?:string)=>LLM_PROTOCOLS.find(p=>p.value===type)?.base||'/v1';
export const clientProtocols=(type:string):string[]=>['qwen','gemini'].includes(type)?[type]:['openai','ark'].includes(type)?[type,'openai-compatible']:['anthropic','ollama'].includes(type)?['openai-compatible',type]:['openai-compatible'];
