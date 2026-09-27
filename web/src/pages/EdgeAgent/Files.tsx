import {createContext,useContext,useEffect,useRef,useState} from 'react';
import {ArrowUp,Download,File as FileIcon,Folder,Paperclip,RefreshCw,X} from 'lucide-react';
import {Button,Field,Input,Modal,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import {edgeAgent,type AgentSnapshot} from '@/services/edgeAgent';

export type AgentFile={name:string;path:string;size:number;directory?:boolean;media_type?:string;preview?:string};
type FileReply=AgentSnapshot&{files?:AgentFile[];file?:AgentFile;transfer_id?:string;file_data?:string;file_offset?:number;file_done?:boolean};
const limit=20*1024*1024,chunk=128*1024;
export const FileDownloadContext=createContext<((file:AgentFile)=>void)|undefined>(undefined);
export function FileAttachments({files}:{files:AgentFile[]}){
 const download=useContext(FileDownloadContext),{tr}=useI18n();
 return <div className="edge-agent-message-files">{files.map(f=><Button key={f.path} disabled={!download} title={f.path} onClick={()=>download?.(f)}><Paperclip size={14}/><span>{f.name}</span><Download size={13}/><span className="sr-only">{tr('下载','Download')}</span></Button>)}</div>;
}
function base64(bytes:Uint8Array){let text='';for(let i=0;i<bytes.length;i+=8192)text+=String.fromCharCode(...bytes.subarray(i,i+8192));return btoa(text);}
const imageFile=(file:File)=>/\.(png|jpe?g|gif)$/i.test(file.name);
export function useAgentFiles(edge:number,access:string,session?:string){
 const {tr}=useI18n();const [attachments,setAttachments]=useState<AgentFile[]>([]),[error,setError]=useState(''),[progress,setProgress]=useState<{name:string;percent:number;download:boolean}>();
 const controller=useRef<AbortController>(),previews=useRef(new Set<string>()),input=useRef<HTMLInputElement>(null),imageInput=useRef<HTMLInputElement>(null);
 const current=useRef(session);current.current=session;
 function clear(){for(const url of previews.current)URL.revokeObjectURL(url);previews.current.clear();setAttachments([]);}
 useEffect(()=>{clear();setProgress(undefined);setError('');return()=>{controller.current?.abort();controller.current=undefined;for(const url of previews.current)URL.revokeObjectURL(url);previews.current.clear();};},[session]);
 async function perform(action:string,file:Record<string,unknown>,signal?:AbortSignal):Promise<FileReply>{
  const r=await edgeAgent(edge,action,{access_id:access,session_id:session,file},signal) as FileReply;
  if(r.status!=='ok')throw new Error(r.status);return r;
 }
 const failure=()=>tr('传输未完成。请检查连接器、访问权限、文件大小及路径后重试。','Transfer failed. Check the connector, permissions, file size and path, then retry.');
 async function upload(files:File[]){
  if(!session||controller.current||!files.length)return;
  if(files.length+attachments.length>8){setError(tr('每条消息最多添加 8 个附件。','Add up to 8 attachments per message.'));return;}
  if(files.some(f=>f.size>limit||(imageFile(f)&&f.size>4*1024*1024))){setError(tr('单个文件最多 20 MB；PNG、JPEG、GIF 图片最多 4 MB。','Files: up to 20 MB each. PNG, JPEG and GIF images: up to 4 MB.'));return;}
  const abort=new AbortController(),id=session;controller.current=abort;setError('');
  let transfer='';
  try{
   for(const file of files){
    if(abort.signal.aborted)throw new DOMException('Cancelled','AbortError');
    setProgress({name:file.name,percent:0,download:false});
    const begun=await perform('file_begin',{name:file.name,size:file.size},abort.signal);transfer=begun.transfer_id||'';if(!transfer)throw new Error('invalid transfer');
    for(let offset=0;offset<file.size;offset+=chunk){
     const data=new Uint8Array(await file.slice(offset,offset+chunk).arrayBuffer());
     const next=await perform('file_write',{transfer_id:transfer,offset,data:base64(data)},abort.signal);
     if(next.file_offset!==offset+data.length)throw new Error('invalid offset');
     setProgress({name:file.name,percent:Math.round((offset+data.length)/Math.max(1,file.size)*100),download:false});
    }
    const done=await perform('file_commit',{transfer_id:transfer},abort.signal);transfer='';if(!done.file)throw new Error('missing file');
    if(current.current!==id||abort.signal.aborted)return;
    const preview=imageFile(file)?URL.createObjectURL(file):undefined;if(preview)previews.current.add(preview);
    setAttachments(previous=>[...previous,{...done.file!,preview}]);
   }
  }catch{if(!abort.signal.aborted&&current.current===id)setError(failure());}
  finally{
   if(transfer)await perform('file_cancel',{transfer_id:transfer}).catch(()=>{/* Edge expires abandoned partial transfers. */});
   if(controller.current===abort){controller.current=undefined;setProgress(undefined);}
  }
 }
 async function download(file:AgentFile){
  if(!session||controller.current)return;
  if(file.size>limit){setError(tr('暂支持下载 20 MB 以内的单个文件。','Downloads support files up to 20 MB.'));return;}
  const abort=new AbortController();controller.current=abort;setError('');setProgress({name:file.name,percent:0,download:true});let transfer='';
  try{
   const parts:Uint8Array<ArrayBuffer>[]=[];let offset=0,done=false;
   while(!done){
    const next=await perform('file_read',transfer?{transfer_id:transfer,offset}:{path:file.path},abort.signal);
    transfer=next.transfer_id||'';if(!transfer)throw new Error('invalid transfer');
    const bytes=Uint8Array.from(atob(next.file_data||''),c=>c.charCodeAt(0));
    if((next.file_offset??0)!==offset+bytes.length||offset+bytes.length>limit||(!bytes.length&&!next.file_done))throw new Error('invalid file');
    parts.push(bytes);offset+=bytes.length;done=!!next.file_done;
    setProgress({name:file.name,percent:Math.min(100,Math.round(offset/Math.max(1,file.size)*100)),download:true});
   }
   transfer='';if(abort.signal.aborted)return;
   const url=URL.createObjectURL(new Blob(parts,{type:'application/octet-stream'}));
   const link=document.createElement('a');link.href=url;link.download=file.name;document.body.append(link);link.click();link.remove();setTimeout(()=>URL.revokeObjectURL(url),1000);
  }catch{if(!abort.signal.aborted)setError(failure());}
  finally{if(transfer)await perform('file_cancel',{transfer_id:transfer}).catch(()=>{});if(controller.current===abort){controller.current=undefined;setProgress(undefined);}}
 }
 function remove(file:AgentFile){if(file.preview){URL.revokeObjectURL(file.preview);previews.current.delete(file.preview);}setAttachments(previous=>previous.filter(f=>f.path!==file.path));}
 return {attachments,upload,download,clear,input,imageInput,busy:!!progress,
  controls:<><input ref={input} type="file" multiple hidden aria-label={tr('上传文件','Upload files')} onChange={e=>{void upload(Array.from(e.target.files||[]));e.target.value='';}}/><input ref={imageInput} type="file" accept="image/png,image/jpeg,image/gif" multiple hidden aria-label={tr('上传图片','Upload images')} onChange={e=>{void upload(Array.from(e.target.files||[]));e.target.value='';}}/></>,
  feedback:<>{error&&<Notice tone="danger">{error}<Button variant="ghost" aria-label={tr('关闭提示','Dismiss')} onClick={()=>setError('')}><X size={13}/></Button></Notice>}{progress&&<div className="edge-agent-transfer" role="status"><span>{progress.download?tr('下载中','Downloading'):tr('上传中','Uploading')} · {progress.name}</span><progress max={100} value={progress.percent}/><span>{progress.percent}%</span><Button onClick={()=>controller.current?.abort()}>{tr('取消','Cancel')}</Button></div>}</>,
  chips:attachments.length>0&&<div className="edge-agent-attachments">{attachments.map(f=><div key={f.path}>{f.preview?<img src={f.preview} alt={f.name}/>:<FileIcon size={20}/>}<span title={f.name}>{f.name}</span><Button variant="ghost" aria-label={`${tr('移除附件','Remove attachment')} ${f.name}`} onClick={()=>remove(f)}><X size={13}/></Button></div>)}</div>
 };
}
export function ProjectFiles({edge,access,session,onClose,onDownload,busy}:{edge:number;access:string;session:string;onClose:()=>void;onDownload:(f:AgentFile)=>void;busy:boolean}){
 const {tr}=useI18n();const [path,setPath]=useState('.'),[data,setData]=useState<FileReply>(),[error,setError]=useState(''),[loading,setLoading]=useState(false);const controller=useRef<AbortController>();
 async function load(p:string){
  controller.current?.abort();const abort=new AbortController();controller.current=abort;setLoading(true);setError('');
  try{const next=await edgeAgent(edge,'file_list',{access_id:access,session_id:session,file:{path:p}},abort.signal) as FileReply;if(abort.signal.aborted)return;if(next.status!=='ok')throw new Error('unavailable');setData(next);setPath(next.directory||'.');}
  catch{if(!abort.signal.aborted)setError(tr('无法读取目录，请检查路径、连接器和权限。','Cannot read this folder. Check the path, connector and permissions.'));}
  finally{if(!abort.signal.aborted)setLoading(false);}
 }
 useEffect(()=>{void load('.');return()=>controller.current?.abort();},[session]);
 return <Modal open width={640} title={tr('项目文件','Project files')} onClose={onClose} footer={<Button onClick={onClose}>{tr('关闭','Close')}</Button>}><div className="edge-agent-directory-browser">
  <Field label={tr('项目内路径','Path within project')} hint={tr('仅当前会话的项目目录。单个文件最多 20 MB。','Only this conversation’s project directory. Up to 20 MB per file.')}><div className="edge-agent-directory-path"><Input value={path} onChange={e=>setPath(e.target.value)} onKeyDown={e=>{if(e.key==='Enter')void load(path);}}/><Button disabled={loading} onClick={()=>void load(path)}>{tr('打开','Open')}</Button></div></Field>
  <div className="edge-agent-directory-roots"><Button disabled={loading} onClick={()=>void load('.')}>{tr('项目根目录','Project root')}</Button><Button aria-label={tr('上一级','Parent folder')} disabled={loading||!data?.directory||data.directory==='.'} onClick={()=>void load(data!.directory!.split('/').slice(0,-1).join('/')||'.')}><ArrowUp size={14}/></Button><Button aria-label={tr('刷新文件','Refresh files')} disabled={loading} onClick={()=>void load(data?.directory||'.')}><RefreshCw size={14}/></Button></div>
  {error&&<Notice tone="danger">{error}</Notice>}
  <div className="edge-agent-directory-list" aria-busy={loading}>{loading?<p role="status">{tr('正在加载…','Loading…')}</p>:!error&&<>{!data?.files?.length&&<p>{tr('此目录为空','This folder is empty')}</p>}{data?.files?.map(f=><Button variant="ghost" key={f.path} disabled={busy&&!f.directory} title={f.path} onClick={()=>f.directory?void load(f.path):(onDownload(f),onClose())}>{f.directory?<Folder size={15}/>:<FileIcon size={15}/>}<span>{f.name}</span>{!f.directory&&<><small>{Math.ceil(f.size/1024)} KB</small><Download size={14}/></>}</Button>)}</>}</div>
  {data?.truncated&&<Notice>{tr('文件较多，仅显示前 500 项。','Showing the first 500 entries.')}</Notice>}
 </div></Modal>;
}
