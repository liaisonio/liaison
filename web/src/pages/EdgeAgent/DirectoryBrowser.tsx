import {useEffect,useRef,useState} from 'react';
import {ArrowUp,Folder,ChevronRight} from 'lucide-react';
import {Button,Field,Input,Modal,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import {edgeAgent,type AgentSnapshot} from '@/services/edgeAgent';

export function DirectoryBrowser({edge,accessID,current,onClose,onSelect,switching=false}:{edge:number;accessID?:string;current:string;onClose:()=>void;onSelect:(path:string)=>void;switching?:boolean}){
 const {tr}=useI18n();const [path,setPath]=useState(current),[data,setData]=useState<AgentSnapshot>(),[pending,setPending]=useState(false),[error,setError]=useState('');
 const abort=useRef<AbortController>();
 async function load(directory:string){
  abort.current?.abort();const controller=new AbortController();abort.current=controller;setPending(true);setError('');
  try{const next=await edgeAgent(edge,'directories',{directory,...(accessID?{access_id:accessID}:{})},controller.signal);
   if(controller.signal.aborted)return;
   if(next.status!=='ok'){setError(next.status==='upgrade_required'||next.status==='invalid_request'?tr('无法打开目录。请检查路径、目录范围和连接器版本。','Cannot open this folder. Check the path, allowed roots and connector version.'):tr('目录不可用，请检查连接器与访问权限。','Folder unavailable. Check the connector and permissions.'));return;}
   setData(next);setPath(next.directory||'');
  }catch(e){if((e as Error).name!=='AbortError')setError(tr('无法加载目录，请重试。','Could not load folders. Try again.'));}
  finally{if(!controller.signal.aborted)setPending(false);}
 }
 useEffect(()=>{void load(current);return()=>abort.current?.abort();},[edge,accessID]);
 return <Modal open width={600} title={tr('选择工作目录','Choose working directory')} onClose={onClose} footer={<><Button onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={pending||!!error||!data?.directory||path!==data.directory} onClick={()=>onSelect(data!.directory!)}>{tr('选择此目录','Select folder')}</Button></>}>
  <div className="edge-agent-directory-browser">
   <Field label={tr('远端目录','Remote directory')} hint={tr('仅浏览连接器运行用户的主目录及此访问保存的项目目录。','Browse only the connector user’s home and this access entry’s saved project.')}><div className="edge-agent-directory-path"><Input value={path} maxLength={4096} onChange={e=>setPath(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'){e.preventDefault();void load(path);}}}/><Button disabled={pending} onClick={()=>void load(path)}>{tr('打开','Open')}</Button></div></Field>
   <div className="edge-agent-directory-roots"><Button disabled={pending} onClick={()=>void load('')}>{tr('主目录','Home')}</Button>{data?.directory_roots?.slice(1).map(root=><Button key={root} title={root} disabled={pending} onClick={()=>void load(root)}>{tr('默认项目','Default project')}</Button>)}<Button disabled={pending||!data?.parent_directory} aria-label={tr('上一级','Parent folder')} onClick={()=>void load(data!.parent_directory!)}><ArrowUp size={14}/></Button></div>
   {error&&<Notice tone="danger">{error}</Notice>}
   <div className="edge-agent-directory-list" aria-busy={pending}>
    {pending?<p role="status">{tr('正在加载目录…','Loading folders…')}</p>:!error&&<>{!data?.directories?.length&&<p>{tr('此目录下没有可显示的子目录。','No visible subfolders in this directory.')}</p>}{data?.directories?.map(dir=><Button variant="ghost" key={dir.path} onClick={()=>void load(dir.path)}><Folder size={15}/><span>{dir.name}</span><ChevronRight size={14}/></Button>)}</>}
   </div>
   {data?.truncated&&<Notice>{tr('目录较多，仅显示部分。可输入完整路径打开。','Some folders are omitted. Enter the full path to open a folder.')}</Notice>}
   {switching&&<Notice>{tr('选择后需确认新建会话，不会修改此访问的默认目录。','You will confirm a new conversation. The access entry’s default directory stays unchanged.')}</Notice>}
  </div>
 </Modal>;
}
