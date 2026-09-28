import {useEffect,useRef,useState} from 'react';
import type {AgentSnapshot} from '@/services/edgeAgent';
import {TurnTimingTracker,type TurnTiming} from './turnTiming';

export function useTurnTiming(){
  const tracker=useRef<TurnTimingTracker>();
  const [timing,setTiming]=useState<TurnTiming>();
  useEffect(()=>{
    const changed=()=>{
      if(document.visibilityState==='hidden'&&tracker.current&&tracker.current.value.finishedMs===undefined){
        tracker.current.value.background=true;setTiming({...tracker.current.value});
      }
    };
    document.addEventListener('visibilitychange',changed);return()=>document.removeEventListener('visibilitychange',changed);
  },[]);
  return {
    timing,
    begin(session:AgentSnapshot){tracker.current=new TurnTimingTracker(session,performance.now());tracker.current.value.background=document.visibilityState!=='visible';setTiming({...tracker.current.value});},
    response(confirmed:boolean){tracker.current?.requestFinished(performance.now(),confirmed);if(tracker.current)setTiming({...tracker.current.value});},
    observe(session:AgentSnapshot){
      if(!tracker.current)return;
      tracker.current.observe(session,performance.now());
      const next={...tracker.current.value};
      setTiming(previous=>JSON.stringify(previous)===JSON.stringify(next)?previous:next);
    },
    reset(){tracker.current=undefined;setTiming(undefined);},
  };
}
