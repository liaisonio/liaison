import {useEffect, useRef, useState} from 'react';
import {Check, Minus} from 'lucide-react';
import {request} from '@/api/client';
import {Button, Notice} from '@/components/ui';
import {useI18n} from '@/i18n';
import {refreshPermissions} from '@/store/permissions';
import './Permissions.less';

type Policy = {catalog:Array<{code:string;name:string}>;enabled:string[]};
type PermissionNode = {label:[string,string];code?:string;children?:PermissionNode[];note?:[string,string]};
const modules:PermissionNode[] = [
 {label:['首页','Home'],children:[{label:['Agent','Agent'],code:'ai.home.use'}]},
 {label:['访问','Access'],children:[{label:['Agent','Agent'],code:'ai.access.use'}],note:['适用于各协议访问页面中的 Agent','Applies to Agent in all protocol workspaces']},
 {label:['日志与审计','Logs & Audit'],children:[{label:['管理日志','Management logs'],code:'audit.read'},{label:['审计日志','Audit logs'],code:'audit.read'}],note:['管理日志与审计日志共用权限，勾选时同步生效','Both log pages share one permission and are selected together']},
 {label:['设置','Settings'],children:[{label:['模型配置','Models'],children:[{label:['查看','View'],code:'settings.global.read'},{label:['修改','Edit'],code:'settings.global.update'}]}]},
];
const codesOf=(node:PermissionNode):string[]=>[...new Set(node.code?[node.code]:(node.children||[]).flatMap(codesOf))];
function PermissionCheck({label,checked,mixed,disabled,onChange}:{label:string;checked:boolean;mixed:boolean;disabled:boolean;onChange:(on:boolean)=>void}) {
 const ref=useRef<HTMLInputElement>(null);
 useEffect(()=>{if(ref.current)ref.current.indeterminate=mixed;},[mixed]);
 return <span className="permission-check"><input ref={ref} type="checkbox" aria-label={label} checked={checked} disabled={disabled} onChange={e=>onChange(e.target.checked)}/><span aria-hidden="true">{mixed?<Minus size={13} strokeWidth={3}/>:checked?<Check size={13} strokeWidth={3}/>:null}</span></span>;
}
export default function Permissions() {
 const {tr}=useI18n();
 const [policy,setPolicy]=useState<Policy>(); const [busy,setBusy]=useState(false); const [notice,setNotice]=useState('');
 useEffect(()=>{ void request<API.Response<Policy>>('/api/v1/iam/roles/user/permissions').then(r=>setPolicy(r.data)).catch(e=>setNotice(e.message)); },[]);
 const toggle=(codes:string[],on:boolean)=>{setNotice('');setPolicy(p=>{
  if (!p)return p; const next=new Set(p.enabled);
  codes.filter(code=>p.catalog.some(f=>f.code===code)).forEach(code=>on?next.add(code):next.delete(code));
  if(codes.includes('settings.global.update')&&on)next.add('settings.global.read');
  if(codes.includes('settings.global.read')&&!on)next.delete('settings.global.update');
  return {...p,enabled:[...next]};
 });};
 const save=async()=>{if(!policy)return;setBusy(true);setNotice('');try{
  const r=await request<API.Response<Policy>>('/api/v1/iam/roles/user/permissions',{method:'PUT',data:{enabled:policy.enabled}});
  setPolicy(r.data); await refreshPermissions(); setNotice(tr('权限已保存，适用于所有普通用户。','Permissions saved for all ordinary users.'));
 }catch(e){setNotice((e as Error).message);}finally{setBusy(false);}};
 const renderNode=(node:PermissionNode,parent=''):React.ReactNode=>{
  const label=tr(...node.label),path=parent?`${parent} / ${label}`:label;
  const codes=codesOf(node).filter(code=>policy?.catalog.some(f=>f.code===code));
  const count=codes.filter(code=>policy?.enabled.includes(code)).length;
  const checked=codes.length>0&&count===codes.length,mixed=count>0&&!checked;
  return <li key={label}><label className={`permission-tree-row${!parent?' is-module':''}`}><PermissionCheck label={path} checked={checked} mixed={mixed} disabled={busy||!codes.length} onChange={on=>toggle(codes,on)}/><span>{label}</span>{!parent&&<span className={`permission-state${count?' is-enabled':''}`}>{mixed?tr('部分开放','Partially enabled'):checked?tr('已开放','Enabled'):tr('未开放','Disabled')}</span>}</label>{node.children&&<ul>{node.children.map(child=>renderNode(child,path))}</ul>}{node.note&&<p className="permission-tree-note">{tr(...node.note)}</p>}</li>;
 };
 const allCodes=[...new Set(modules.flatMap(codesOf))].filter(code=>policy?.catalog.some(f=>f.code===code));
 const all=allCodes.length>0&&allCodes.every(code=>policy?.enabled.includes(code));
 const any=allCodes.some(code=>policy?.enabled.includes(code));
 return <section className="settings-section permission-settings"><header className="settings-section-heading"><h2>{tr('用户功能权限','User feature permissions')}</h2><p>{tr('基础功能始终开放。以下授权同时约束页面、API 和 AI 工具，管理员权限不受影响。','Basic features stay available. These grants control pages, APIs and AI tools, without changing administrator permissions.')}</p></header>
 {notice&&<Notice>{notice}</Notice>}
 {policy&&<div className="permission-tree"><label className="permission-tree-toolbar"><PermissionCheck label={tr('全部功能','All features')} checked={all} mixed={any&&!all} disabled={busy} onChange={on=>toggle(allCodes,on)}/><strong>{tr('全部功能','All features')}</strong><span>{tr('勾选模块可批量选择子项','Select a module to select its children')}</span></label><ul>{modules.map(node=>renderNode(node))}</ul></div>}
 <footer className="permission-settings-footer"><Button variant="primary" disabled={!policy||busy} onClick={()=>void save()}>{busy?tr('保存中…','Saving…'):tr('保存权限','Save permissions')}</Button></footer></section>;
}
