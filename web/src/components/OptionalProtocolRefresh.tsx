import {useEffect} from 'react';
import {request} from '@/api/client';
import {useSession} from '@/store/session';
import {useOptionalProtocols} from '@/store/optionalProtocols';

export function OptionalProtocolRefresh() {
  const token=useSession(state=>state.token);
  useEffect(()=>{
    useOptionalProtocols.setState({dameng:false});
    if(!token)return;
    const controller=new AbortController();
    request<API.Response<{dameng:boolean}>>('/api/v1/webdata/capabilities',{signal:controller.signal,skipErrorHandler:true})
      .then(result=>{if(!controller.signal.aborted)useOptionalProtocols.setState({dameng:result.data?.dameng===true});})
      .catch(()=>{/* Older servers and failures leave optional drivers hidden. */});
    return ()=>controller.abort();
  },[token]);
  return null;
}
