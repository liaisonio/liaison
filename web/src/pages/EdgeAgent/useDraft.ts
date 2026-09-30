import {useCallback,useState} from 'react';
import {useSession} from '@/store/session';
import {agentDraftKey,readAgentDraft,writeAgentDraft} from '@/store/agentDraft';

export function useDraft(access:string,session?:string){
  const owner=useSession(s=>s.initialState.currentUser?.id);
  const key=agentDraftKey(owner,access,session);
  const [draft,setDraft]=useState(()=>({key,text:readAgentDraft(key),saved:true}));
  // Switch the displayed value with the scope, never save old text under a new key.
  if(draft.key!==key)setDraft({key,text:readAgentDraft(key),saved:true});
  const setText=useCallback((text:string)=>{
    setDraft({key,text,saved:writeAgentDraft(key,text)});
  },[key]);
  const clearSent=useCallback((sent:string)=>{
    // A delayed success from an unmounted workspace must not remove a newer draft.
    if(readAgentDraft(key)===sent)writeAgentDraft(key,'');
  },[key]);
  return {text:draft.key===key?draft.text:'',setText,clearSent,saved:draft.saved};
}
