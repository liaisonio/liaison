import {Cable,FolderSync,FolderTree,Database,SquareTerminal,Globe,BrainCircuit,Boxes} from 'lucide-react';
const assets=import.meta.glob('../../../../docs/assets/integrations/*.{svg,png}',{eager:true,query:'?url',import:'default'}) as Record<string,string>;
export default function ProtocolIcon({protocol}:{protocol:string}){
 const key=protocol.replace(/^web/,'');
 if(key==='smb')return <FolderTree size={18} strokeWidth={1.75} aria-hidden="true"/>;
 if(['doris','starrocks','tidb','dameng'].includes(key))return <Database size={18} strokeWidth={1.75} aria-hidden="true"/>;
 // S3 is a protocol, not necessarily Amazon's hosted service (e.g. MinIO).
 if(key==='s3')return <Boxes size={18} strokeWidth={1.75} style={{flexShrink:0}} aria-hidden="true"/>;
 // Official Lucide SVGs, reused without custom path drawing (ISC license).
 if(key==='ssh'||key==='sftp'){
  const Icon=key==='ssh'?SquareTerminal:FolderSync;
  return <Icon size={18} strokeWidth={1.75} style={{flexShrink:0}} aria-hidden="true"/>;
 }
 const name=key==='rdp'?'windows':key==='openai-compatible'?'openai':key;
 const src=assets[`../../../../docs/assets/integrations/${name}.${name==='vnc'?'png':'svg'}`];
 if(src)return <img src={src} alt="" width={18} height={18} style={{objectFit:'contain',flexShrink:0}}/>;
 const Icon=['http','https'].includes(key)?Globe:['llm','aiapi'].includes(key)?BrainCircuit:Cable;
 return <Icon size={18} aria-hidden="true"/>;
}
