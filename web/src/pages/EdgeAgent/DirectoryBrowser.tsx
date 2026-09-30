import {useEffect,useRef,useState} from 'react';
import {ArrowUp,Folder,ChevronRight} from 'lucide-react';
import {Button,Field,Input,Modal,Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import {edgeAgent,type AgentSnapshot} from '@/services/edgeAgent';

export function DirectoryBrowser({edge,accessID,current,onClose,onSelect,switching=false,creating=false}:{edge:number;accessID?:string;current:string;onClose:()=>void;onSelect:(path:string)=>void;switching?:boolean;creating?:boolean}){
 const {tr}=useI18n();const [path,setPath]=useState(current),[data,setData]=useState<AgentSnapshot>(),[pending,setPending]=useState(false),[error,setError]=useState('');
 const abort=useRef<AbortController>(),search=useRef<HTMLDivElement>(null),list=useRef<HTMLDivElement>(null);
 const [query,setQuery]=useState(''),[selected,setSelected]=useState(0);
 const [recent,setRecent]=useState<string[]>([]);
 useEffect(()=>{
  setRecent([]);
  if(!creating||!accessID)return;
  const controller=new AbortController();
  void edgeAgent(edge,'sessions',{access_id:accessID},controller.signal).then(next=>{
   if(controller.signal.aborted||next.status!=='ok')return;
   const ordered=[...(next.sessions||[])].sort((a,b)=>(Date.parse(b.updated_at)||0)-(Date.parse(a.updated_at)||0));
   setRecent([...new Set(ordered.map(session=>session.project).filter(Boolean))].slice(0,4));
  }).catch(()=>{/* Folder browsing remains available when recent history cannot be loaded. */});
  return()=>controller.abort();
 },[edge,accessID,creating]);
 const folders=(data?.directories||[]).filter(dir=>dir.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()));
 useEffect(()=>{setSelected(0);},[query]);
 useEffect(()=>{list.current?.querySelector(`[data-index="${selected}"]`)?.scrollIntoView({block:'nearest'});},[selected]);
 async function load(directory:string){
  abort.current?.abort();const controller=new AbortController();abort.current=controller;setPending(true);setError('');
  try{const next=await edgeAgent(edge,'directories',{directory,...(accessID?{access_id:accessID}:{})},controller.signal);
   if(controller.signal.aborted)return;
   if(next.status!=='ok'){setError(next.status==='upgrade_required'||next.status==='invalid_request'?tr('无法打开目录。请检查路径、目录范围和连接器版本。','Cannot open this folder. Check the path, allowed roots and connector version.'):tr('目录不可用，请检查连接器与访问权限。','Folder unavailable. Check the connector and permissions.'));return;}
   setData(next);setPath(next.directory||'');setQuery('');setSelected(0);
  }catch(e){if((e as Error).name!=='AbortError')setError(tr('无法加载目录，请重试。','Could not load folders. Try again.'));}
  finally{if(!controller.signal.aborted)setPending(false);}
 }
 useEffect(()=>{void load(current);return()=>abort.current?.abort();},[edge,accessID]);
 return <Modal open width={600} title={creating?tr('新建会话 · 选择项目目录','New conversation · Choose project folder'):tr('选择工作目录','Choose working directory')} onClose={onClose} footer={<><Button onClick={onClose}>{tr('取消','Cancel')}</Button><Button variant="primary" disabled={pending||!!error||!data?.directory||path!==data.directory} onClick={()=>onSelect(data!.directory!)}>{creating?tr('在此目录新建','Create in this folder'):tr('选择此目录','Select folder')}</Button></>}>
  <div className="edge-agent-directory-browser">
   {recent.length>0&&<section className="edge-agent-recent-projects" aria-label={tr('最近项目','Recent projects')}>
    <span>{tr('最近项目','Recent projects')}</span>
    <div>{recent.map(directory=><Button variant="ghost" key={directory} title={directory} aria-label={`${tr('打开最近项目','Open recent project')} ${directory}`} disabled={pending} onClick={()=>void load(directory)}><Folder size={14}/><span>{directory.split(/[\\/]/).filter(Boolean).pop()||directory}</span></Button>)}</div>
   </section>}
   <Field label={tr('远端目录','Remote directory')}><div className="edge-agent-directory-path"><Input aria-label={tr('远端目录','Remote directory')} value={path} maxLength={4096} onChange={e=>setPath(e.target.value)} onKeyDown={e=>{if(e.key==='Enter'&&!e.nativeEvent.isComposing){e.preventDefault();void load(path);}}}/><Button disabled={pending} onClick={()=>void load(path)}>{tr('打开','Open')}</Button></div></Field>
   <div className="edge-agent-directory-roots"><Button disabled={pending} onClick={()=>void load('')}>{tr('主目录','Home')}</Button>{data?.directory_roots?.slice(1).map(root=><Button key={root} title={root} disabled={pending} onClick={()=>void load(root)}>{tr('默认项目','Default project')}</Button>)}<Button disabled={pending||!data?.parent_directory} aria-label={tr('上一级','Parent folder')} onClick={()=>void load(data!.parent_directory!)}><ArrowUp size={14}/></Button></div>
   {error&&<Notice tone="danger">{error}</Notice>}
   <div ref={search}><Input aria-label={tr('定位文件夹','Find folder')} placeholder={tr('输入名称定位文件夹…','Type a folder name…')} value={query} onChange={e=>setQuery(e.target.value)} onKeyDown={e=>{if(e.nativeEvent.isComposing||pending||error)return;if(e.key==='ArrowDown'||e.key==='ArrowUp'){e.preventDefault();setSelected(i=>Math.max(0,Math.min(folders.length-1,i+(e.key==='ArrowDown'?1:-1))));}else if(e.key==='Enter'&&folders[selected]){e.preventDefault();void load(folders[selected].path);}}}/></div>
   <div ref={list} className="edge-agent-directory-list" aria-busy={pending} onKeyDown={e=>{if(e.nativeEvent.isComposing||e.ctrlKey||e.metaKey||e.altKey||pending)return;if(e.key.length===1&&e.key!==' '){e.preventDefault();setQuery(q=>q+e.key);search.current?.querySelector('input')?.focus({preventScroll:true});}}}>
    {pending?<p role="status">{tr('正在加载目录…','Loading folders…')}</p>:!error&&<>{!folders.length&&<p role="status">{query?tr('没有匹配的文件夹。','No matching folders.'):tr('此目录下没有可显示的子目录。','No visible subfolders in this directory.')}</p>}{folders.map((dir,index)=><Button variant="ghost" className={query&&index===selected?'is-located':undefined} data-index={index} key={dir.path} title={dir.name} onClick={()=>void load(dir.path)}><Folder size={15}/><span>{dir.name}</span><ChevronRight size={14}/></Button>)}</>}
   </div>
   {data?.truncated&&<Notice>{tr('目录较多，仅显示部分。可输入完整路径打开。','Some folders are omitted. Enter the full path to open a folder.')}</Notice>}
   {switching&&<Notice>{tr('选择后需确认新建会话，不会修改此访问的默认目录。','You will confirm a new conversation. The access entry’s default directory stays unchanged.')}</Notice>}
  </div>
 </Modal>;
}
