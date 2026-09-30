import {useEffect,useState} from 'react';

export function useProjectListState(access:string,kind:string){
 const key=`liaison-agent-projects:${access}:${kind}`;
 const [value,setValue]=useState<Set<string>>(()=>{
  try{const data:unknown=JSON.parse(sessionStorage.getItem(key)||'[]');return new Set(Array.isArray(data)?data.filter((item):item is string=>typeof item==='string'):[]);}catch{return new Set();}
 });
 useEffect(()=>{try{sessionStorage.setItem(key,JSON.stringify([...value]));}catch{/* List controls still work when storage is disabled. */}},[key,value]);
 return [value,setValue] as const;
}
