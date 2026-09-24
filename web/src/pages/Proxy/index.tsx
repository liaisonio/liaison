import {useOptionalProtocols, optionalProtocolEnabled} from '@/store/optionalProtocols';
import {resolveLegacyLLMTypes,applyResolvedLLMTypes} from '@/services/llmTypes';
import {isLLMApplicationType,protocolFamily} from '@/constants/llmProtocols';
import { Button, Column, DangerConfirm, DataTable, Drawer, Field, Input, Modal, Notice, Pager, Select, StatusPill, Timestamp } from '@/components/ui';
import { isLLMAccessType, ACCESS_TYPES, ACCESS_CREATION_TYPES, ACCESS_TYPES_CHANGED_EVENT, accessProtocolForType, accessTypeLabel, applicationTypeForAccess, getProxyAccessType, isAccessType, isProxyPublicPortExposed, isSupportedAccessType, isWebAccessType } from '@/constants/accessTypes';
import { useI18n } from '@/i18n';
import OverflowTabs from '@/components/ui/OverflowTabs';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { history, useSearchParams } from '@/lib/runtime';
import { createApplication, createProxy, deleteProxy, deleteProxyFirewall, getApplicationList, getClientIP, getProxyFirewall, getProxyList, updateProxy, upsertProxyFirewall } from '@/services/api';
import { Check, Copy, Globe2, Plus, Shield, Terminal, Trash2 } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import ProtocolIcon from '@/components/icons/ProtocolIcon';
import {Switch} from '@/components/ui/complex';
import {Link} from 'react-router-dom';
import ConnectionSummary from './ConnectionSummary';
import LLMSummary from './LLMSummary';
import LLMEmptyState from './LLMEmptyState';
import AccessEmptyState from './AccessEmptyState';
import LLMApplicationDraft, {type LLMDraftHandle} from './LLMApplicationDraft';
import LLMApplicationPicker from './LLMApplicationPicker';
import ApplicationDraft, {applicationTarget} from './ApplicationDraft';
import LLMConnection, {type LLMConnectionHandle} from './LLMConnection';
import { InitialConnectionFields, emptyConnection, saveInitialConnection, directAccessPath, loadAccessConnection, AccessConfigurationRequired, supportsInitialConnection, databaseAccessTypes } from './connection';
import './connection.less';
import {accessGroup,groupTypes,accessTabLabel} from '@/constants/accessGroups';
import './groups.less';
import {launchWebEntry} from '@/services/webEntry';
import WebEntryModeField, {useWebEntryMode} from '@/components/WebEntryModeField';

const pageSize = 10;
const supportsSavedPassword = (type?: string) => !!type && supportsInitialConnection(type) && !['webs3', 'webmemcached'].includes(type);
const defaultAccessName = () => {
  const bytes = new Uint8Array(4);
  window.crypto.getRandomValues(bytes);
  return `Access-${Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')}`;
};

const connectionCommand = (row: API.Proxy) => {
  const type = getProxyAccessType(row);
  const rawTarget = row.access_url || `${window.location.hostname}:${row.port}`;
  const separator = rawTarget.lastIndexOf(':');
  const host = separator > 0 ? rawTarget.slice(0, separator) : window.location.hostname;
  const port = separator > 0 ? rawTarget.slice(separator + 1) : String(row.port);
  switch (type) {
    case 'ssh': return `ssh <user>@${host} -p ${port}`;
    case 'rdp': return `xfreerdp /v:${host}:${port} /u:<user>`;
    case 'vnc': return `vncviewer ${host}:${port}`;
    case 'mysql': return `mysql -h ${host} -P ${port} -u <user> -p`;
    case 'postgresql': return `psql -h ${host} -p ${port} -U <user>`;
    case 'redis': return `redis-cli -h ${host} -p ${port}`;
    case 'mongodb': return `mongosh "mongodb://${host}:${port}"`;
    default: return `nc ${host} ${port}`;
  }
};

const ConnectionCommand: React.FC<{ row: API.Proxy; copiedLabel: string; copyHintLabel: string; commandLabel: string; exampleLabel: string }> = ({ row, copiedLabel, copyHintLabel, commandLabel, exampleLabel }) => {
  const trigger = useRef<HTMLButtonElement>(null);
  const [position, setPosition] = useState<{ top: number; left: number }>();
  const [copied, setCopied] = useState(false);
  const command = connectionCommand(row);
  const show = () => {
    const rect = trigger.current?.getBoundingClientRect();
    if (rect) setPosition({ top: rect.top - 8, left: Math.max(12, Math.min(rect.left, window.innerWidth - 390)) });
  };
  const hide = () => setPosition(undefined);
  const copy = async () => {
    await navigator.clipboard.writeText(command);
    setCopied(true);
    window.setTimeout(() => setCopied(false), 1600);
  };
  return <><button ref={trigger} className="liaison-connect-command" onMouseEnter={show} onMouseLeave={hide} onFocus={show} onBlur={hide} onClick={() => void copy()}><Terminal size={12} />{commandLabel}</button>{position ? createPortal(<div className="liaison-connection-tooltip" style={position} role="tooltip"><span>{exampleLabel}</span><code>{command}</code><small><Copy size={11} />{copied ? copiedLabel : copyHintLabel}</small></div>, document.body) : null}</>;
};

const ProxyPage: React.FC = () => {
  useOptionalProtocols();
  const { tr } = useI18n();
  const [routeSearch,setRouteSearch] = useSearchParams();
  const group=accessGroup(routeSearch);
  const tabs=group?groupTypes(group):[];
  const requestedRouteType = routeSearch.get('access_type')==='aiapi'?'':protocolFamily(routeSearch.get('access_type')) || '';
  const routeType = isSupportedAccessType(requestedRouteType) && (!group || group.types.includes(requestedRouteType)) ? requestedRouteType : group?.types.length===1?group.types[0]:'';
  const [rows, setRows] = useState<API.Proxy[]>([]);
  const [applications, setApplications] = useState<API.Application[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [loaded, setLoaded] = useState(false);
  const filters = {name:routeSearch.get('name')||'',access_type:routeType,application_id:routeSearch.get('application_id')||'',status:routeSearch.get('status')||''};
  const setFilters=(next:typeof filters|((previous:typeof filters)=>typeof filters))=>{
    const value=typeof next==='function'?next(filters):next;
    const search=new URLSearchParams(routeSearch);
    for(const key of ['name','access_type','application_id','status'] as const){if(value[key])search.set(key,value[key]);else search.delete(key);}
    setRouteSearch(search,{replace:true});
  };
  const chooseTab=(type:string)=>{const search=new URLSearchParams(routeSearch);if(group)search.set('category',group.value);if(type)search.set('access_type',type);else search.delete('access_type');setRouteSearch(search);setPage(1);};
  const debouncedName = useDebouncedValue(filters.name);
  const [createOpen, setCreateOpen] = useState(false);
  const llmConnection=useRef<LLMConnectionHandle>(null);
  const llmDraft=useRef<LLMDraftHandle>(null);
  const [applicationSource,setApplicationSource]=useState<'existing'|'new'>();
  const [llmEdge,setLLMEdge]=useState('');
  const [applicationDraft,setApplicationDraft]=useState({name:'',address:''});
  const [createdApplication,setCreatedApplication]=useState<API.Application>();
  const [draftReady,setDraftReady]=useState(false),[connectionReady,setConnectionReady]=useState(false);
  const [editRow, setEditRow] = useState<API.Proxy>();
  const [openAfterSave, setOpenAfterSave] = useState(false);
  const webEntry = useWebEntryMode();
  const {mode:entryMode,setMode:setEntryMode} = webEntry;
  const [deleteRow, setDeleteRow] = useState<API.Proxy>();
  const [form, setForm] = useState({ name: '', application_id: '', access_type: routeType, port: '', description: '' });
  const [suggestedAccessName, setSuggestedAccessName] = useState(defaultAccessName);
  const [saving, setSaving] = useState(false);
  const [initialConnection, setInitialConnection] = useState(emptyConnection);
  const [createdAccess, setCreatedAccess] = useState<API.Proxy>();
  const [createError, setCreateError] = useState('');
  const [openingId, setOpeningId] = useState<number>();
  const configuredRoute = useRef<string>();
  const [togglingIds, setTogglingIds] = useState<number[]>([]);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  useEffect(()=>{
    if(notice?.tone!=='success')return;
    const timer=window.setTimeout(()=>setNotice(current=>current===notice?undefined:current),3000);
    return()=>window.clearTimeout(timer);
  },[notice]);
  const [firewallRow, setFirewallRow] = useState<API.Proxy>();
  const [cidrs, setCidrs] = useState<string[]>([]);
  const [cidrDraft, setCidrDraft] = useState('');
  const [clientIP, setClientIP] = useState('');
  const [firewallUpdatedAt, setFirewallUpdatedAt] = useState('');
  const [firewallDirty, setFirewallDirty] = useState(false);

  useEffect(() => { setPage(1); }, [routeType,group?.value]);
  const loadApplications = useCallback(async () => { try { const response = await getApplicationList({ page_size: 1000 }); if (response.code === 200) setApplications(response.data?.applications || []); } catch { setApplications([]); } }, []);
  const load = useCallback(async () => {
    setLoading(true);
    setLoaded(false);
    try {
      const response = await getProxyList({ page: 1, page_size: 1000 });
      if (response.code !== 200) throw new Error(response.message);
      const resolved=await resolveLegacyLLMTypes(response.data?.proxies||[]);setRows(resolved);setApplications(old=>applyResolvedLLMTypes(old,resolved));
      window.dispatchEvent(new CustomEvent(ACCESS_TYPES_CHANGED_EVENT));
      setLoaded(true);
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('加载访问失败', 'Failed to load access') }); }
    finally { setLoading(false); }
  }, [tr]);
  useEffect(() => { void loadApplications(); }, [loadApplications]);
  useEffect(() => { void load(); }, [load]);

  const filteredRows = useMemo(() => {
    const name = debouncedName.trim().toLowerCase();
    return rows.filter((row) => {
      if (!isSupportedAccessType(getProxyAccessType(row))) return false;
      if (group && !group.types.includes(getProxyAccessType(row)||'')) return false;
      if (name && !row.name.toLowerCase().includes(name)) return false;
      if (filters.access_type && (filters.access_type==='aiapi'?!isLLMAccessType(getProxyAccessType(row)):getProxyAccessType(row) !== filters.access_type)) return false;
      if (filters.application_id && String(row.application?.id || '') !== filters.application_id) return false;
      if (filters.status && row.status !== filters.status) return false;
      return true;
    });
  }, [debouncedName, filters.access_type, filters.application_id, filters.status, rows,group]);
  const visibleRows = useMemo(() => filteredRows.slice((page - 1) * pageSize, page * pageSize), [filteredRows, page]);
  const categoryHasRows=rows.some(row=>isSupportedAccessType(getProxyAccessType(row))&&(!group||group.types.includes(getProxyAccessType(row)||'')));
  const initialEmpty=loaded&&!loading&&!categoryHasRows&&!filters.name&&!filters.application_id&&!filters.status;
  const filteredEmpty=loaded&&!loading&&!initialEmpty&&filteredRows.length===0;
  const resetEmptyFilters=()=>{setFilters({name:'',access_type:group?.types.length===1?group.types[0]:'',application_id:'',status:''});setPage(1);};

  const selectedAccessType = routeType || form.access_type;
  const closeCreate=()=>{if(saving)return;setCreateOpen(false);setInitialConnection(emptyConnection());setCreatedAccess(undefined);setCreateError('');};
  const availableApplications = useMemo(() => {
    if (!selectedAccessType) return [];
    // The current data plane treats every non-HTTP public listener as an
    // opaque L4 stream. Therefore TCP access can target SSH/RDP/database
    // applications as well as applications explicitly labelled TCP.
    if (selectedAccessType === 'tcp') {
      return applications;
    }
    if (!isAccessType(selectedAccessType)) return [];
    const applicationType = applicationTypeForAccess(selectedAccessType);
    return applyResolvedLLMTypes(applications,rows).filter((item) => protocolFamily(item.application_type) === protocolFamily(applicationType));
  }, [applications, rows, selectedAccessType]);
  const selectedApplication = useMemo(() => applications.find((item) => String(item.id) === form.application_id), [applications, form.application_id]);
  const llmSource=applicationSource||'existing';
  const openCreate = () => {
    setApplicationDraft({name:'',address:''});setCreatedApplication(undefined);
    setApplicationSource(undefined);
    setLLMEdge('');
    setEntryMode('path');
    setInitialConnection(emptyConnection()); setCreatedAccess(undefined); setCreateError('');
    setSuggestedAccessName(defaultAccessName());
    setForm({ name: '', application_id: '', access_type: routeType || tabs[0]?.value || '', port: '', description: '' });
    setCreateOpen(true);
  };
  useEffect(()=>{
    const id=routeSearch.get('new_application');
    if(!id||!applications.some(a=>String(a.id)===id&&isLLMApplicationType(a.application_type)))return;
    openCreate();setLLMEdge(String(applications.find(a=>String(a.id)===id)?.edge_id||''));setApplicationSource('existing');setForm({name:routeSearch.get('new_name')||'',application_id:id,access_type:protocolFamily(applyResolvedLLMTypes(applications,rows).find(a=>String(a.id)===id)?.application_type)||'',port:'',description:''});
    const search=new URLSearchParams(routeSearch);search.delete('new_application');search.delete('new_name');setRouteSearch(search,{replace:true});
  },[applications,routeSearch]);
  const create = async (event: FormEvent) => {
    event.preventDefault();
    if(saving)return;
    if(isLLMAccessType(selectedAccessType)&&llmSource==='new'){
      if(!llmDraft.current)return;
      setSaving(true);setCreateError('');
      try{
        await llmDraft.current.create(form.name.trim()||suggestedAccessName,form.description);
        setCreateOpen(false);setNotice({tone:'success',text:tr('应用和访问已创建','Application and access created')});
        window.dispatchEvent(new Event('liaison-llm-config-updated'));
        await Promise.all([loadApplications(),load()]);
      }catch(error){setCreateError((error as Error).message);void loadApplications();}
      finally{setSaving(false);}
      return;
    }
    if (!selectedAccessType || (llmSource==='existing'?!selectedApplication:!llmEdge||!applicationTarget(applicationDraft.address))) { setCreateError(tr('请选择应用，或填写新应用的连接信息。','Select an application, or complete the new application connection.')); return; }
    if (!isAccessType(selectedAccessType)) return;
    const webOnly = isWebAccessType(selectedAccessType) || selectedAccessType==='http' && entryMode!=='port';
    setSaving(true);
    const accessProtocol = accessProtocolForType(selectedAccessType);
    setCreateError('');
    let access = createdAccess;
    let application = createdApplication || selectedApplication;
    try {
      if(llmSource==='new'&&!createdApplication){
        const target=applicationTarget(applicationDraft.address)!;
        const response=await createApplication({name:applicationDraft.name.trim()||`App-${applicationDraft.address.trim()}`,edge_id:Number(llmEdge),application_type:applicationTypeForAccess(selectedAccessType),ip:target.host,port:target.port});
        if(response.code!==200||!response.data)throw Error('application');
        application=response.data;setCreatedApplication(application);void loadApplications();
      }
      if(isLLMAccessType(selectedAccessType)){
        if(!llmConnection.current)throw Error('configuration');
        await llmConnection.current.saveUpstream();
      }
      if (!access) {
        const response = await createProxy({ name: form.name.trim() || suggestedAccessName, description: form.description, application_id: application!.id, access_protocol: accessProtocol, http_entry_mode:selectedAccessType==='http'?entryMode:undefined, expose_public_port: !webOnly, port: !webOnly && form.port ? Number(form.port) : undefined });
        if (response.code !== 200 || !response.data) throw new Error('create');
        access=response.data; setCreatedAccess(access);
      }
      await saveInitialConnection(access.id,selectedAccessType,access.name,initialConnection);
      if(isLLMAccessType(selectedAccessType))await llmConnection.current!.saveAccess(access.id);
      window.dispatchEvent(new Event('liaison-llm-config-updated'));
      setCreateOpen(false);setInitialConnection(emptyConnection());setCreatedAccess(undefined);
      setNotice({tone:'success',text:tr('访问已创建','Access created')});await load();
    } catch {
      setCreateError(access ? tr('访问已创建，但连接配置未保存。请修正后重试，不会重复创建访问。','Access was created, but its connection was not saved. Correct the configuration and retry; no duplicate access will be created.') : llmSource==='new'&&application ? tr('应用已创建，但访问未创建。重试会复用此应用；关闭后可在应用列表找到它。','The application was created, but access was not. Retry reuses this application; it remains in Applications if you close this dialog.') : tr('创建失败，请检查配置后重试。','Creation failed. Check the configuration and retry.'));
      if(access)void load();
    } finally {setSaving(false);}
  };
  const openEdit = async (row:API.Proxy, continueAccess = false) => {
    setOpenAfterSave(continueAccess);
    setEntryMode(row.http_entry_mode || 'port');
    setOpeningId(row.id);
    try {
      const connection=await loadAccessConnection(row.id,getProxyAccessType(row)||'');
      setInitialConnection(connection);
      setForm({name:row.name,application_id:String(row.application?.id||''),access_type:getProxyAccessType(row)||'',port:row.port?String(row.port):'',description:row.description||''});
      setCreateError('');setEditRow(row);
    }catch{setNotice({tone:'danger',text:tr('无法加载访问配置，请检查权限后重试。','Unable to load access configuration. Check permissions and retry.')});}
    finally{setOpeningId(undefined);}
  };
  useEffect(()=>{
    const id=routeSearch.get('configure');if(!id){configuredRoute.current=undefined;return;}if(configuredRoute.current===id)return;
    const row=rows.find(r=>String(r.id)===id);if(!row)return;
    configuredRoute.current=id;void openEdit(row);
    const search=new URLSearchParams(routeSearch);search.delete('configure');setRouteSearch(search,{replace:true});
  },[rows,routeSearch]);
  const update = async (event: FormEvent) => {
    event.preventDefault(); if (!editRow) return; setSaving(true);
    setCreateError('');
    try {
      if(isLLMAccessType(getProxyAccessType(editRow))){
        if(!llmConnection.current)throw Error('configuration');
        await llmConnection.current.saveUpstream();
        await llmConnection.current.saveAccess(editRow.id);
        window.dispatchEvent(new Event('liaison-llm-config-updated'));
      }
      await saveInitialConnection(editRow.id,getProxyAccessType(editRow)||'',form.name.trim(),initialConnection);
      setInitialConnection(await loadAccessConnection(editRow.id,getProxyAccessType(editRow)||''));
    }catch{setCreateError(tr('连接配置保存失败，请检查后重试。','Unable to save connection configuration. Check it and retry.'));setSaving(false);return;}
    try { const isHTTP=getProxyAccessType(editRow)==='http'; const response = await updateProxy(editRow.id, { name: form.name.trim(), description: form.description, http_entry_mode:isHTTP?entryMode:undefined, port: (!isHTTP||entryMode==='port')&&form.port ? Number(form.port) : undefined, expose_public_port: isHTTP?entryMode==='port':isProxyPublicPortExposed(editRow) }); if (response.code !== 200) throw new Error(response.message);
      if(openAfterSave){
        try {
          const path=await directAccessPath(editRow.id,getProxyAccessType(editRow)||'');
          const search=routeSearch.toString(),from=`/proxy${search?`?${search}`:''}`;
          setEditRow(undefined);setOpenAfterSave(false);
          history.push(`${path}${path.includes('?')?'&':'?'}from=${encodeURIComponent(from)}`);
        }catch{setCreateError(tr('配置已保存，但无法打开访问，请重试。','Configuration saved, but access could not be opened. Please retry.'));}
      }else{setEditRow(undefined);setNotice({ tone: 'success', text: tr('访问已更新', 'Access updated') });await load();}
    }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); } finally { setSaving(false); }
  };
  const remove = async () => { if (!deleteRow) return; try { const response = await deleteProxy(deleteRow.id); if (response.code !== 200) throw new Error(response.message); setDeleteRow(undefined); setNotice({ tone: 'success', text: tr('访问已删除', 'Access deleted') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('删除失败', 'Delete failed') }); } };
  const toggle = async (row: API.Proxy) => {
    if (togglingIds.includes(row.id)) return;
    const nextStatus = row.status === 'running' ? 'stopped' : 'running';
    setTogglingIds((ids) => [...ids, row.id]);
    setRows((items) => items.map((item) => item.id === row.id ? { ...item, status: nextStatus, effective_status: nextStatus === 'stopped' ? 'stopped' : item.effective_status } : item));
    try {
      const response = await updateProxy(row.id, { status: nextStatus });
      if (response.code !== 200) throw new Error(response.message);
      if (response.data) setRows((items) => items.map((item) => item.id === row.id ? { ...item, ...response.data } : item));
    } catch (error: any) {
      setRows((items) => items.map((item) => item.id === row.id ? row : item));
      setNotice({ tone: 'danger', text: error?.message || tr('状态更新失败', 'Status update failed') });
    } finally {
      setTogglingIds((ids) => ids.filter((id) => id !== row.id));
    }
  };

  const openFirewall = async (row: API.Proxy) => {
    setFirewallRow(row); setCidrs([]); setCidrDraft(''); setClientIP(''); setFirewallUpdatedAt(''); setFirewallDirty(false);
    try {
      const [rule, client] = await Promise.all([getProxyFirewall(row.id), getClientIP()]);
      if (rule.code === 200) {
        const updatedAt = rule.data?.updated_at || '';
        const values = rule.data?.allowed_cidrs || [];
        setFirewallUpdatedAt(updatedAt);
        setCidrs(!updatedAt && values.length === 1 && values[0] === '0.0.0.0/0' ? [] : values);
      }
      if (client.code === 200) setClientIP(client.data?.ip || '');
    } catch { /* default allow-all */ }
  };
  const addCIDR = () => { const value = cidrDraft.trim(); if (!value || cidrs.includes(value)) return; if (!/^([\da-f:.]+)(\/\d{1,3})?$/i.test(value)) { setNotice({ tone: 'danger', text: tr('请输入有效 IP 或 CIDR', 'Enter a valid IP or CIDR') }); return; } setCidrs((items) => [...items, value]); setCidrDraft(''); setFirewallDirty(true); };
  const saveFirewall = async () => { if (!firewallRow || !firewallDirty) return; setSaving(true); try { const response = await upsertProxyFirewall(firewallRow.id, { allowed_cidrs: cidrs }); if (response.code !== 200) throw new Error(response.message); setFirewallRow(undefined); setNotice({ tone: 'success', text: tr('防火墙规则已更新', 'Firewall rules updated') }); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('保存失败', 'Save failed') }); } finally { setSaving(false); } };
  const clearFirewall = async () => { if (!firewallRow) return; setSaving(true); try { const response = await deleteProxyFirewall(firewallRow.id); if (response.code !== 200) throw new Error(response.message); setFirewallRow(undefined); setNotice({ tone: 'success', text: tr('已恢复默认放行', 'Default access restored') }); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('恢复失败', 'Restore failed') }); } finally { setSaving(false); } };

  const openAccess = async (row: API.Proxy) => {
    const type = getProxyAccessType(row);
    if(type==='http' && row.http_entry_mode && row.http_entry_mode!=='port') {
      const tab=window.open('about:blank','_blank'); if(tab)tab.opener=null;
      setOpeningId(row.id);
      try {const response=await launchWebEntry(row.id);if(response.code!==200||!response.data?.url)throw Error('launch');if(tab)tab.location.replace(response.data.url);else window.location.assign(response.data.url);}
      catch {tab?.close();setNotice({tone:'danger',text:tr('无法打开访问，请检查权限和连接状态。','Unable to open access. Check permissions and connection status.')});}
      finally {setOpeningId(undefined);} return;
    }
    const search = routeSearch.toString();
    const returnTo = `/proxy${search ? `?${search}` : ''}`;
    const internalPath = (path: string) => `${path}${path.includes('?')?'&':'?'}from=${encodeURIComponent(returnTo)}`;
    if(isWebAccessType(type)) {
      if(openingId!==undefined)return;
      setOpeningId(row.id);
      try {history.push(internalPath(await directAccessPath(row.id,type)));}
      catch (error) {if(error instanceof AccessConfigurationRequired)await openEdit(row,true);else setNotice({tone:'danger',text:tr('无法打开访问，请检查权限和连接状态后重试。','Unable to open access. Check permissions and connection status, then retry.')});}
      finally {setOpeningId(undefined);}
      return;
    }
    if (row.access_url) window.open(row.access_url, '_blank', 'noopener,noreferrer');
  };

  const endpointColumn: Column<API.Proxy>[] = isWebAccessType(routeType) ? [] : [{
    key: 'public',
    title: routeType === 'http' ? tr('访问地址', 'Access URL') : tr('访问端口', 'Access port'),
    width: routeType === 'http' ? 180 : 95,
    render: (row) => routeType === 'http' ? <code>{row.access_url || '-'}</code> : isProxyPublicPortExposed(row) ? row.port || tr('自动', 'Auto') : '-',
  }];
  const columns: Column<API.Proxy>[] = [
    { key: 'name', title: tr('访问名称', 'Access'), width: 175, render: (row) => row.name },
    ...(!routeType ? [{ key: 'type', title: tr('协议类型', 'Protocol'), width: 125, render: (row: API.Proxy) => <StatusPill tone="info">{accessTypeLabel(getProxyAccessType(row))}</StatusPill> } as Column<API.Proxy>] : []),
    { key: 'application', title: tr('应用', 'Application'), width: 175, render: (row) => row.application?.id?<Link className="liaison-access-app-link" title={row.application.name} to={`/resource/app?application_id=${encodeURIComponent(row.application.id)}`}>{row.application.name||'-'}</Link>:'-' },
    ...(supportsInitialConnection(routeType)?[
      ...(!['webvnc','webmemcached'].includes(routeType)?[{key:'username',title:routeType==='webs3'?'Access Key':tr('用户名','Username'),width:120,render:(row:API.Proxy)=><ConnectionSummary field="username" id={row.id} type={routeType} revision={row.updated_at}/>}]:[]),
      ...((databaseAccessTypes.includes(routeType)&&routeType!=='webmemcached')||['webs3','websmb'].includes(routeType)?[{key:'database',title:routeType==='websmb'?tr('共享名','Share'):routeType==='webs3'?tr('存储桶','Bucket'):routeType==='weboracle'?tr('服务名 / SID','Service / SID'):tr('数据库','Database'),width:150,render:(row:API.Proxy)=><ConnectionSummary field="database" id={row.id} type={routeType} revision={row.updated_at}/>}]:[]),
      {key:'advanced',title:tr('连接选项','Connection options'),width:190,render:(row:API.Proxy)=><ConnectionSummary field="advanced" id={row.id} type={routeType} revision={row.updated_at}/>},
    ]:[]),
    ...(supportsSavedPassword(routeType)||filteredRows.some(row=>supportsSavedPassword(getProxyAccessType(row)))?[
      {key:'password_saved',title:tr('我的密码','My password'),width:120,render:(row:API.Proxy)=>supportsSavedPassword(getProxyAccessType(row))?<ConnectionSummary field="password" id={row.id} type={getProxyAccessType(row)||''} revision={row.updated_at}/>:<span className="liaison-connection-muted">—</span>},
    ]:[]),
    ...((isLLMAccessType(routeType)||group?.value==='llm')?[
      {key:'external_protocol',title:tr('调用协议','Client protocol'),width:160,render:(row:API.Proxy)=><LLMSummary id={row.id} revision={row.updated_at} field="external"/>},
      {key:'models',title:tr('可调用模型','Available models'),width:180,render:(row:API.Proxy)=><LLMSummary id={row.id} revision={row.updated_at} field="models"/>},
    ]:[]),
    { key: 'application_protocol', title: (isLLMAccessType(routeType)||group?.value==='llm')?tr('上游协议','Upstream protocol'):tr('应用协议', 'Application protocol'), width: 130, render: (row) => isLLMAccessType(getProxyAccessType(row))?<LLMSummary id={row.id} revision={row.updated_at} field="protocol"/>:<span className="liaison-inline-name"><ProtocolIcon protocol={row.application?.application_type||''}/>{accessTypeLabel(row.application?.application_type)}</span> },
    ...((isLLMAccessType(routeType)||group?.value==='llm')?[
      {key:'auth',title:tr('认证方式','Authentication'),width:120,render:()=>tr('API 密钥','API key')},
    ]:[]),
    ...endpointColumn,
    { key: 'enabled', title: tr('启用', 'Enabled'), width: 82, render: (row) => <Switch checked={row.status==='running'} disabled={togglingIds.includes(row.id)} aria-busy={togglingIds.includes(row.id)} aria-label={tr(`启用访问：${row.name}`,`Enable access: ${row.name}`)} onChange={()=>void toggle(row)}/> },
    { key: 'created', title: tr('创建时间', 'Created'), width: 150, render: (row) => <Timestamp value={row.created_at} /> },
    { key: 'description', title: tr('描述', 'Description'), width: 180, render: (row) => row.description || '-' },
    { key:'actions', title:tr('操作','Actions'), width:'1%', fixed:'right', render:row=><span className="liaison-table-actions liaison-access-actions">
      {isLLMAccessType(getProxyAccessType(row))?<Link className="liaison-table-link" to={`/ai/${row.id}?from=${encodeURIComponent(`/proxy${routeSearch.toString()?`?${routeSearch.toString()}`:''}`)}`}>{tr('查看','View')}</Link>:<>
      {isProxyPublicPortExposed(row)&&getProxyAccessType(row)!=='http'?<ConnectionCommand row={row} commandLabel={tr('连接命令','Command')} exampleLabel={tr('连接示例','Connection example')} copyHintLabel={tr('点击复制','Click to copy')} copiedLabel={tr('已复制','Copied')}/>:<button className="liaison-table-link" disabled={openingId!==undefined||row.status!=='running'} onClick={()=>void openAccess(row)}>{openingId===row.id?tr('打开中…','Opening…'):tr('去访问','Open')}</button>}
      </>}
      <button className="liaison-table-link" disabled={openingId!==undefined} onClick={()=>void openEdit(row)}>{tr('编辑','Edit')}</button>
      {isProxyPublicPortExposed(row)&&<button className="liaison-table-link" onClick={()=>void openFirewall(row)}>{tr('防火墙','Firewall')}</button>}
      <button className="liaison-table-link is-danger" onClick={()=>setDeleteRow(row)}>{tr('删除','Delete')}</button></span> },
  ];

  const accessForm = (id: string, submit: (event: FormEvent) => void, editing = false) => {
    if(!editing&&isLLMAccessType(selectedAccessType)) {
      const useApplication=(value:string)=>{
        setApplicationSource(value==='new'?'new':'existing');setCreateError('');
        const app=applications.find(a=>String(a.id)===value);
        setConnectionReady(false);setForm(old=>({...old,application_id:app?value:''}));
      };
      return <form id={id} className="liaison-llm-create-form" onSubmit={submit}>
        <p className="liaison-access-route-note">{tr('访问通过连接器连接到应用或模型服务。','Access reaches your application or model service through the connector.')}</p>
        <fieldset className="liaison-llm-draft" disabled={saving||!!createdAccess}>
          <div className="liaison-llm-create-identity">
          <Field label={tr('访问名称','Access name')}><Input disabled={saving} value={form.name} onChange={e=>setForm(old=>({...old,name:e.target.value}))} placeholder={suggestedAccessName}/></Field>
          <Field label={tr('访问协议','Protocol')} required><Select aria-label={tr('访问协议','Protocol')} value={selectedAccessType} disabled={!!routeType} onChange={e=>{setApplicationSource(undefined);setConnectionReady(false);setDraftReady(false);setForm(old=>({...old,access_type:e.target.value,application_id:''}));}}>{ACCESS_CREATION_TYPES.filter(p=>isLLMAccessType(p.value)).map(p=><option key={p.value} value={p.value}>{p.label}</option>)}</Select></Field>
          </div>
          <LLMApplicationPicker edge={llmEdge} application={llmSource==='new'?'new':form.application_id} protocol={selectedAccessType} applications={availableApplications} disabled={saving||!!createdAccess}
            onEdge={value=>{setLLMEdge(value);setForm(old=>({...old,application_id:''}));setCreateError('');}} onApplication={useApplication}/>
        </fieldset>
        {createOpen&&llmEdge&&<div hidden={llmSource!=='new'}><LLMApplicationDraft key={`${llmEdge}:${selectedAccessType}`} edge={llmEdge} ref={llmDraft} protocol={selectedAccessType} applications={applications} disabled={saving||llmSource!=='new'} onUse={useApplication} onReady={setDraftReady}/></div>}
        {form.application_id&&<LLMConnection key={form.application_id} ref={llmConnection} applicationId={form.application_id} disabled={saving} reuseUpstream onReady={setConnectionReady}/>}
      </form>;
    }
    const isHTTP=(editing?getProxyAccessType(editRow):selectedAccessType)==='http';
    const exposesPublicPort = isHTTP?entryMode==='port':editing ? isProxyPublicPortExposed(editRow) : Boolean(selectedAccessType && !isWebAccessType(selectedAccessType));
    return <form id={id} className={`liaison-access-form${editing ? ' is-editing' : ''}${routeType ? ' is-protocol-fixed' : ''}`} onSubmit={submit}>
      {!editing&&<p className="is-full liaison-access-route-note">{tr('访问通过连接器连接到应用或模型服务。','Access reaches your application or model service through the connector.')}</p>}
      <fieldset className="liaison-access-identity" disabled={saving||(!editing&&!!createdAccess)}>
      <Field label={tr('访问名称', 'Access name')}><Input value={form.name} onChange={(event) => setForm((value) => ({ ...value, name: event.target.value }))} placeholder={editing ? undefined : suggestedAccessName} /></Field>
      {!editing ? <Field label={tr('访问协议', 'Protocol')} required><Select value={selectedAccessType} disabled={Boolean(routeType)||!!createdApplication} onChange={(event) => { setApplicationSource(undefined);setApplicationDraft({name:'',address:''});setInitialConnection(emptyConnection()); setForm((value) => ({ ...value, access_type: event.target.value, application_id: '', port: '' })); }}><option value="">{tr('选择协议', 'Select protocol')}</option>{ACCESS_CREATION_TYPES.filter(item=>optionalProtocolEnabled(item.value)&&(!group||group.types.includes(item.value))).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</Select></Field> : null}
      {!editing&&<div className="is-full liaison-llm-draft"><LLMApplicationPicker edge={llmEdge} application={llmSource==='new'?'new':form.application_id} protocol={selectedAccessType} applications={availableApplications} disabled={saving||!selectedAccessType||!!createdApplication||!!createdAccess}
        onEdge={value=>{setLLMEdge(value);setInitialConnection(emptyConnection());setForm(old=>({...old,application_id:''}));setCreateError('');}}
        onApplication={value=>{setApplicationSource(value==='new'?'new':'existing');setInitialConnection(emptyConnection());setForm(old=>({...old,application_id:value==='new'?'':value}));setCreateError('');}}/>
        {llmSource==='new'&&llmEdge&&<ApplicationDraft edge={llmEdge} value={applicationDraft} onChange={setApplicationDraft} disabled={saving||!!createdApplication||!!createdAccess}/>}
      </div>}
      {isHTTP&&<div className="is-full"><WebEntryModeField {...webEntry}/></div>}
      {!(isLLMAccessType(selectedAccessType)&&!editing&&llmSource==='new')&&<div className={editing || !exposesPublicPort ? 'is-full' : 'liaison-access-description'}><Field label={tr('描述', 'Description')}><Input value={form.description} onChange={(event) => setForm((value) => ({ ...value, description: event.target.value }))} placeholder={tr('选填', 'Optional')} /></Field></div>}
      {exposesPublicPort ? <div className="liaison-access-port"><Field label={tr('访问端口', 'Access port')} hint={tr('留空自动分配', 'Leave empty for automatic assignment')}><Input type="number" min={1} max={65535} value={form.port} onChange={(event) => setForm((value) => ({ ...value, port: event.target.value }))} placeholder={tr('自动分配', 'Auto')} /></Field></div> : null}
      </fieldset>
      <InitialConnectionFields type={editing?getProxyAccessType(editRow)||'':selectedAccessType} value={initialConnection} onChange={setInitialConnection} disabled={saving}/>
      {isLLMAccessType(editing?getProxyAccessType(editRow):selectedAccessType)&&form.application_id&&<div className="is-full" hidden={!editing&&llmSource!=='existing'}><LLMConnection key={`${editing?'edit':'create'}:${form.application_id}`} ref={llmConnection} applicationId={form.application_id} accessId={editing?editRow?.id:undefined} disabled={saving||(!editing&&llmSource!=='existing')}/></div>}
    </form>;
  };

  return <div className="liaison-page-stack liaison-access-list">
    {group && group.value!=='tcp' && <OverflowTabs label={tr('访问类型','Access types')} value={routeType} onChange={chooseTab} items={[...(tabs.length>1?[{value:'',label:tr('全部','All')}]:[]),...tabs.map(tab=>({value:tab.value,label:accessTabLabel(tab.label)}))]}/>}
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar">
      <label className="liaison-compound"><span>{tr('访问名称', 'Access')}</span><input value={filters.name} onChange={(event) => { setFilters((value) => ({ ...value, name: event.target.value })); setPage(1); }} placeholder={tr('输入访问名称', 'Access name')} /></label>{!group && !routeType ? <label className="liaison-compound"><span>{tr('协议', 'Protocol')}</span><select value={filters.access_type} onChange={(event) => { setFilters((value) => ({ ...value, access_type: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option>{ACCESS_TYPES.filter(item=>optionalProtocolEnabled(item.value)).map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label> : null}<label className="liaison-compound"><span>{tr('应用', 'Application')}</span><select value={filters.application_id} onChange={(event) => { setFilters((value) => ({ ...value, application_id: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option>{applications.map((item) => <option key={item.id} value={item.id}>{item.name}</option>)}</select></label><label className="liaison-compound"><span>{tr('启用状态', 'Enabled')}</span><select value={filters.status} onChange={(event) => { setFilters((value) => ({ ...value, status: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option><option value="running">{tr('启用', 'Enabled')}</option><option value="stopped">{tr('停用', 'Disabled')}</option></select></label><div className="liaison-filter-actions"><Button onClick={() => { setFilters({ name: '', access_type: routeType, application_id: '', status: '' }); setPage(1); }}>{tr('重置', 'Reset')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('访问列表', 'Access')}</h2>{!initialEmpty && loaded && <Button variant="primary" onClick={openCreate}><Plus size={14} />{tr('新建访问', 'Create access')}</Button>}</header>{initialEmpty ? group?.value==='llm'?<LLMEmptyState onCreate={openCreate}/>:<AccessEmptyState category={group?.value} onCreate={openCreate}/> : filteredEmpty?<AccessEmptyState onReset={resetEmptyFilters}/>:<><DataTable columns={columns} rows={visibleRows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无访问', 'No access')} /><Pager page={page} pageSize={pageSize} total={filteredRows.length} onPageChange={setPage} /></>}</section>
    <Modal open={createOpen} title={tr('新建访问', 'Create access')} onClose={closeCreate} closeOnMask={!saving} width={520} footer={<><Button onClick={closeCreate} disabled={saving}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-proxy" disabled={saving||(isLLMAccessType(selectedAccessType)&&(llmSource==='new'?!llmEdge||!draftReady:!form.application_id||!connectionReady))}>{saving?tr('保存中…','Saving…'):createdAccess?tr('保存连接','Save connection'):isLLMAccessType(selectedAccessType)?tr('创建访问','Create access'):tr('确定','Create')}</Button></>}>{accessForm('create-proxy', create)}{createError&&<Notice tone="danger">{createError}</Notice>}</Modal>
    <Modal open={!!editRow} title={openAfterSave?tr('配置访问','Configure access'):tr('编辑访问', 'Edit access')} onClose={() => {if(!saving){setEditRow(undefined);setOpenAfterSave(false);setInitialConnection(emptyConnection());}}} closeOnMask={!saving} width={520} footer={<><Button disabled={saving} onClick={() => {setEditRow(undefined);setOpenAfterSave(false);setInitialConnection(emptyConnection());}}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-proxy" disabled={saving}>{openAfterSave?tr('保存并访问','Save & open'):tr('确定', 'Save')}</Button></>}>{accessForm('edit-proxy', update, true)}{createError&&<Notice tone="danger">{createError}</Notice>}</Modal>
    <Modal open={!!deleteRow} title={tr('删除访问', 'Delete access')} onClose={() => setDeleteRow(undefined)} width={430} footer={<><Button onClick={() => setDeleteRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={() => void remove()}>{tr('删除', 'Delete')}</Button></>}><DangerConfirm title={tr(`删除“${deleteRow?.name || ''}”？`, `Delete “${deleteRow?.name || ''}”?`)} description={tr('该访问入口和防火墙规则将立即停止，此操作无法撤销。', 'This endpoint and its firewall rules will stop immediately. This cannot be undone.')} /></Modal>
    <Drawer open={!!firewallRow} title={<span className="liaison-firewall-title">{tr('防火墙', 'Firewall')}<small>{firewallRow?.name}</small></span>} onClose={() => setFirewallRow(undefined)}>
      <div className="liaison-firewall-overview">
        <div className="liaison-firewall-overview-icon"><Shield size={17} /></div>
        <div><strong>{firewallDirty ? tr('规则变更待保存', 'Rule changes are pending') : firewallUpdatedAt ? tr('来源白名单已启用', 'Source allowlist enabled') : tr('当前为默认放行', 'Default access is active')}</strong><p>{tr('规则应用于该访问的公网入口，不影响连接器所在网络。', 'Rules apply to this public endpoint only and do not affect the connector network.')}</p></div>
        <span className={firewallDirty ? 'is-pending' : firewallUpdatedAt ? 'is-restricted' : 'is-open'}>{firewallDirty ? tr('待保存', 'Pending') : firewallUpdatedAt ? tr('已限制', 'Restricted') : tr('未限制', 'Open')}</span>
      </div>
      <dl className="liaison-firewall-summary">
        <div><dt>{tr('公网入口', 'Public endpoint')}</dt><dd><Globe2 size={13} /><code>{firewallRow?.access_url || `${window.location.hostname}:${firewallRow?.port || '-'}`}</code></dd></div>
        <div><dt>{tr('协议', 'Protocol')}</dt><dd>{firewallRow?.application?.application_type === 'http' ? 'HTTP' : 'TCP'}</dd></div>
        <div><dt>{tr('规则', 'Rules')}</dt><dd>{firewallDirty || firewallUpdatedAt ? tr(`${cidrs.length} 条`, `${cidrs.length}`) : tr('默认', 'Default')}</dd></div>
      </dl>
      {clientIP ? <div className="liaison-firewall-current"><div><span>{tr('当前访问 IP', 'Current IP')}</span><code>{clientIP}</code></div>{cidrs.includes(`${clientIP}/32`) ? <span className="is-added"><Check size={13} />{tr('已加入', 'Added')}</span> : <button onClick={() => { setCidrs((items) => [...items, `${clientIP}/32`]); setFirewallDirty(true); }}><Plus size={13} />{tr('加入规则', 'Add rule')}</button>}</div> : null}
      <section className="liaison-firewall-rules">
        <header><div><h3>{tr('来源规则', 'Source rules')}</h3><span>{tr('只允许下列 IP 或网段访问', 'Only the following IPs or networks are allowed')}</span></div><b>{cidrs.length}</b></header>
        <div className="liaison-firewall-add"><Input aria-label={tr('IP 或 CIDR', 'IP or CIDR')} value={cidrDraft} onChange={(event) => setCidrDraft(event.target.value)} placeholder={tr('输入 IP 或 CIDR，例如 203.0.113.8/32', 'IP or CIDR, e.g. 203.0.113.8/32')} onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addCIDR(); } }} /><Button onClick={addCIDR}><Plus size={13} />{tr('添加', 'Add')}</Button></div>
        <div className="liaison-firewall-rule-table">
          <div className="liaison-firewall-rule-head"><span>{tr('来源 CIDR', 'Source CIDR')}</span><span>{tr('协议', 'Protocol')}</span><span>{tr('端口', 'Port')}</span><span>{tr('策略', 'Policy')}</span><span /></div>
          {cidrs.map((cidr) => <div className="liaison-firewall-rule-row" key={cidr}><code>{cidr}</code><span>{firewallRow?.application?.application_type === 'http' ? 'HTTP' : 'TCP'}</span><span>{firewallRow?.port || '-'}</span><span className="is-allow">{tr('允许', 'Allow')}</span><button aria-label={tr(`删除 ${cidr}`, `Delete ${cidr}`)} onClick={() => { setCidrs((items) => items.filter((item) => item !== cidr)); setFirewallDirty(true); }}><Trash2 size={14} /></button></div>)}
          {cidrs.length === 0 ? <div className={`liaison-firewall-empty${firewallDirty || firewallUpdatedAt ? ' is-deny' : ''}`}><Shield size={18} /><strong>{firewallDirty || firewallUpdatedAt ? tr('保存后将拒绝全部来源', 'Saving will deny all sources') : tr('尚未启用来源限制', 'Source restrictions are not enabled')}</strong><span>{firewallDirty || firewallUpdatedAt ? tr('添加至少一条来源规则，或恢复默认放行。', 'Add at least one source rule or restore default access.') : tr('添加第一条规则后，将仅允许白名单内的来源访问。', 'After adding the first rule, only allowlisted sources can connect.')}</span></div> : null}
        </div>
      </section>
      <div className="liaison-drawer-actions"><Button onClick={() => void clearFirewall()} disabled={saving || (!firewallUpdatedAt && !firewallDirty)}>{tr('恢复默认放行', 'Restore default')}</Button><Button variant="primary" onClick={() => void saveFirewall()} disabled={saving || !firewallDirty}>{tr('保存规则', 'Save rules')}</Button></div>
    </Drawer>
  </div>;
};

export default ProxyPage;
