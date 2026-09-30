// Text drafts are tab-local. Never store attachments, permissions or credentials.
export const agentDraftPrefix='liaison:agent-draft:v1:';
export function agentDraftKey(owner:unknown,access:string,session?:string){
  return owner!=null&&String(owner)&&session?agentDraftPrefix+[String(owner),access,session].map(encodeURIComponent).join(':'):'';
}
export function readAgentDraft(key:string):string{
  if(!key)return '';
  try{const value=sessionStorage.getItem(key);return value&&value.length<=16000?value:'';}catch{return '';}
}
export function writeAgentDraft(key:string,text:string):boolean{
  if(!key)return false;
  try{if(text)sessionStorage.setItem(key,text.slice(0,16000));else sessionStorage.removeItem(key);return true;}catch{return false;}
}
export function clearAgentDrafts(){
  try{for(const key of Object.keys(sessionStorage))if(key.startsWith(agentDraftPrefix))sessionStorage.removeItem(key);}catch{/* Storage may be disabled. */}
}
