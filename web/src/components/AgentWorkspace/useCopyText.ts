import {useEffect,useRef,useState} from 'react';

export function useCopyText(text:string){
 const [state,setState]=useState<'idle'|'copied'|'failed'>('idle');
 const timer=useRef<ReturnType<typeof setTimeout>>(),generation=useRef(0);
 useEffect(()=>{generation.current++;setState('idle');return()=>{generation.current++;clearTimeout(timer.current);};},[text]);
 async function copy(){
  const request=++generation.current;clearTimeout(timer.current);
  try{await navigator.clipboard.writeText(text);if(request!==generation.current)return;setState('copied');timer.current=setTimeout(()=>setState('idle'),2000);}
  catch{if(request===generation.current)setState('failed');}
 }
 return {state,copy};
}
