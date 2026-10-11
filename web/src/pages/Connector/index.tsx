import {useOptionalProtocols, optionalProtocolEnabled} from '@/store/optionalProtocols';
import {isLLMAccessType} from '@/constants/accessTypes';
import { Button, Column, DangerConfirm, DataTable, Drawer, Field, Input, Modal, Notice, Pager, Select, StatusPill } from '@/components/ui';
import { accessProtocolForType, accessTypesForApplication, isWebAccessType, type AccessType } from '@/constants/accessTypes';
import { availableApplicationTypes, APPLICATION_TYPES } from '@/constants/applicationTypes';
import {Switch} from '@/components/ui/complex';
import { useI18n } from '@/i18n';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { history } from '@/lib/runtime';
import { createApplication, createEdge, createEdgeScanTask, createProxy, deleteEdge, getEdgeList, getEdgeScanTask, updateEdge } from '@/services/api';
import { Check, Copy, Plus, Radar, Server } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import WebEntryModeField, {useWebEntryMode} from '@/components/WebEntryModeField';
import {useSearchParams} from 'react-router-dom';
import {useFeature} from '@/store/permissions';
import ActionMenu from '@/components/ui/ActionMenu';
import Uninstall from './Uninstall';
import AgentProbe from './AgentProbe';
import {InitialConnectionFields,emptyConnection,saveInitialConnection,loadAccessConnection,directAccessPath,supportsInitialConnection} from '@/pages/Proxy/connection';
import '@/pages/Proxy/connection.less';

const pageSize = 10;
const scanPollInterval = 500;
const scanPollLimit = 120;
const portTypes: Record<number, string> = { 22: 'ssh', 80: 'http', 443: 'http', 3389: 'rdp', 5900: 'vnc', 3306: 'mysql', 5432: 'postgresql', 1433: 'sqlserver', 1521: 'oracle', 9000: 'clickhouse', 6379: 'redis', 27017: 'mongodb' };

const defaultConnectorName = () => {
  const bytes = new Uint8Array(4);
  window.crypto.getRandomValues(bytes);
  return `Connector-${Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')}`;
};

const defaultAccessName = () => {
  const bytes = new Uint8Array(4);
  window.crypto.getRandomValues(bytes);
  return `Access-${Array.from(bytes, (value) => value.toString(16).padStart(2, '0')).join('')}`;
};

const ConnectorPage: React.FC = () => {
  useOptionalProtocols();
  const { tr } = useI18n();
  const [rows, setRows] = useState<API.Edge[]>([]);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: '', device_name: '', online: '', status: '' });
  const debouncedName = useDebouncedValue(filters.name);
  const debouncedDeviceName = useDebouncedValue(filters.device_name);
  const [createOpen, setCreateOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  useEffect(() => {
    if (searchParams.get('create') !== '1') return;
    setCreateOpen(true);
    const next = new URLSearchParams(searchParams); next.delete('create');
    setSearchParams(next, {replace:true});
  }, [searchParams, setSearchParams]);
  const [createName, setCreateName] = useState('');
  const [suggestedConnectorName, setSuggestedConnectorName] = useState(defaultConnectorName);
  const [createDescription, setCreateDescription] = useState('');
  const [keys, setKeys] = useState<API.EdgeCreateResult>();
  const [createError, setCreateError] = useState('');
  const [installOS, setInstallOS] = useState<'other' | 'windows'>('other');
  const [installCopied, setInstallCopied] = useState(false);
  const [editRow, setEditRow] = useState<API.Edge>();
  const [deleteRow, setDeleteRow] = useState<API.Edge>();
  const [uninstallRow,setUninstallRow]=useState<API.Edge>();
  const canUninstall=useFeature('connectors.uninstall');
  const [saving, setSaving] = useState(false);
  const [togglingIds, setTogglingIds] = useState<number[]>([]);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  const [scanRow, setScanRow] = useState<API.Edge>();
  const [scanTask, setScanTask] = useState<API.EdgeScanApplicationTask>();
  const [scanning, setScanning] = useState(false);
  const [probeRevision,setProbeRevision]=useState(0);
  const scanBusyRef = useRef(false);
  const scanRequestRef = useRef(0);
  const [discovered, setDiscovered] = useState<string>();
  const [discoveredForm, setDiscoveredForm] = useState({ name: '', application_type: 'tcp' });
  const [scanAccessApplication, setScanAccessApplication] = useState<API.Application>();
  const [scanAccessName, setScanAccessName] = useState('');
  const [suggestedScanAccessName, setSuggestedScanAccessName] = useState(defaultAccessName);
  const [scanAccessType, setScanAccessType] = useState<AccessType>('tcp');
  const [scanPublicPort, setScanPublicPort] = useState('');
  const [scanConnection,setScanConnection]=useState(emptyConnection);
  const [createdScanAccess,setCreatedScanAccess]=useState<API.Proxy>();
  const [scanAccessError,setScanAccessError]=useState('');
  const webEntry = useWebEntryMode();

  const load = useCallback(async () => {
    setLoading(true);
    try { const response = await getEdgeList({ page: 1, page_size: 1000 }); if (response.code !== 200) throw new Error(response.message); setRows(response.data?.edges || []); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('加载连接器失败', 'Failed to load connectors') }); }
    finally { setLoading(false); }
  }, [tr]);
  useEffect(() => { void load(); }, [load]);

  const filteredRows = useMemo(() => {
    const name = debouncedName.trim().toLowerCase();
    const device = debouncedDeviceName.trim().toLowerCase();
    return rows.filter((row) => {
      if (name && !row.name.toLowerCase().includes(name)) return false;
      if (device && !(row.device?.name || '').toLowerCase().includes(device)) return false;
      if (filters.online && String(row.online) !== filters.online) return false;
      if (filters.status && String(row.status) !== filters.status) return false;
      return true;
    });
  }, [debouncedDeviceName, debouncedName, filters.online, filters.status, rows]);
  const visibleRows = useMemo(() => filteredRows.slice((page - 1) * pageSize, page * pageSize), [filteredRows, page]);

  const createConnector = async (name: string) => {
    setSaving(true); setCreateError('');
    try {
      const resolvedName = name.trim() || suggestedConnectorName;
      setCreateName(resolvedName);
      const response = await createEdge({ name: resolvedName, description: '' });
      if (response.code !== 200 || !response.data) throw new Error(response.message);
      setKeys(response.data); await load();
    }
    catch (error: any) { setCreateError(error?.message || tr('创建失败，请重试', 'Creation failed. Try again.')); } finally { setSaving(false); }
  };
  const openCreate = () => {
    setSuggestedConnectorName(defaultConnectorName()); setCreateName(''); setKeys(undefined); setInstallOS('other'); setInstallCopied(false); setCreateError(''); setCreateOpen(true);
  };
  const closeCreate = () => { if (saving) return; setCreateOpen(false); setCreateName(''); setCreateDescription(''); setKeys(undefined); setInstallOS('other'); setInstallCopied(false); setCreateError(''); };
  const update = async (event: FormEvent) => { event.preventDefault(); if (!editRow || !createName.trim()) return; setSaving(true); try { const response = await updateEdge(editRow.id, { name: createName.trim(), description: createDescription }); if (response.code !== 200) throw new Error(response.message); setEditRow(undefined); setNotice({ tone: 'success', text: tr('连接器已更新', 'Connector updated') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); } finally { setSaving(false); } };
  const toggle = async (row: API.Edge) => {
    if (togglingIds.includes(row.id)) return;
    const nextStatus = row.status === 1 ? 2 : 1;
    setTogglingIds((ids) => [...ids, row.id]);
    setRows((items) => items.map((item) => item.id === row.id ? { ...item, status: nextStatus } : item));
    try {
      const response = await updateEdge(row.id, { status: nextStatus });
      if (response.code !== 200) throw new Error(response.message);
      if (response.data) setRows((items) => items.map((item) => item.id === row.id ? { ...item, ...response.data } : item));
    } catch (error: any) {
      setRows((items) => items.map((item) => item.id === row.id ? row : item));
      setNotice({ tone: 'danger', text: error?.message || tr('操作失败', 'Operation failed') });
    } finally {
      setTogglingIds((ids) => ids.filter((id) => id !== row.id));
    }
  };
  const remove = async () => { if (!deleteRow) return; try { const response = await deleteEdge(deleteRow.id); if (response.code !== 200) throw new Error(response.message); setDeleteRow(undefined); setNotice({ tone: 'success', text: tr('连接器已删除', 'Connector deleted') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('删除失败', 'Delete failed') }); } };

  const waitForScan = async (edgeId: number, taskId: number, requestId: number) => {
    for (let attempt = 0; attempt < scanPollLimit; attempt += 1) {
      const response = await getEdgeScanTask(edgeId, taskId);
      if (scanRequestRef.current !== requestId) return;
      if (response.code !== 200) throw new Error(response.message);
      if (response.data) {
        setScanTask(response.data);
        if (response.data.task_status === 'completed' || response.data.task_status === 'failed') return;
      }
      await new Promise((resolve) => window.setTimeout(resolve, scanPollInterval));
    }
    throw new Error(tr('探测超时，请稍后重试', 'Discovery timed out. Try again later.'));
  };

  const refreshScan = async (edge: API.Edge, force = false) => {
    if (edge.status !== 1 || edge.online !== 1) { setNotice({ tone: 'danger', text: tr('连接器需处于启用且在线状态', 'Connector must be enabled and online') }); return; }
    if (scanBusyRef.current) return;
    scanBusyRef.current = true;
    const requestId = scanRequestRef.current + 1;
    scanRequestRef.current = requestId;
    if (scanRow?.id !== edge.id) setScanTask(undefined);
    setScanRow(edge); setScanning(true);setProbeRevision(value=>value+1);
    try {
      if (!force) {
        const existing = await getEdgeScanTask(edge.id);
        if (existing.code !== 200) throw new Error(existing.message);
        if (existing.data) {
          setScanTask(existing.data);
          if (existing.data.task_status === 'completed' || existing.data.task_status === 'failed') return;
          await waitForScan(edge.id, existing.data.id, requestId);
          return;
        }
      }
      const created = await createEdgeScanTask({ edge_id: edge.id, protocol: 'tcp' });
      if (created.code !== 200 || !created.data?.task_id) throw new Error(created.message || tr('创建探测任务失败', 'Failed to start discovery'));
      await waitForScan(edge.id, created.data.task_id, requestId);
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('探测失败', 'Discovery failed') }); }
    finally {
      if (scanRequestRef.current === requestId) {
        setScanning(false);
        scanBusyRef.current = false;
      }
    }
  };

  const closeScan = () => {
    scanRequestRef.current += 1;
    scanBusyRef.current = false;
    setScanning(false);
    setScanRow(undefined);
    setScanTask(undefined);
  };

  const parsedDiscovery = useMemo(() => {
    if (!discovered) return undefined; const [ip, portRaw] = discovered.split(':'); const port = Number(portRaw); return { ip, port };
  }, [discovered]);
  const scanStatusLabel = scanning
    ? tr('探测中', 'Discovering')
    : scanTask?.task_status === 'completed'
      ? tr('探测完成', 'Completed')
      : scanTask?.task_status === 'failed'
        ? tr('探测失败', 'Failed')
        : tr('尚未探测', 'Not discovered');
  const addDiscovered = async (createAccess: boolean) => {
    webEntry.setMode('path');
    if (!scanRow || !parsedDiscovery) return; setSaving(true);
    try { const response = await createApplication({ name: discoveredForm.name.trim(), application_type: discoveredForm.application_type, ip: parsedDiscovery.ip, port: parsedDiscovery.port, edge_id: scanRow.id }); if (response.code !== 200 || !response.data) throw new Error(response.message); setScanTask((task) => task ? { ...task, applications: task.applications.filter((item) => item !== discovered) } : task); setDiscovered(undefined); if (createAccess) { const options = accessTypesForApplication(discoveredForm.application_type); setScanAccessApplication(response.data); setScanAccessName(''); setSuggestedScanAccessName(defaultAccessName()); setScanAccessType(options[0].value); setScanPublicPort(''); } else setNotice({ tone: 'success', text: tr('应用已添加', 'Application added') }); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('添加应用失败', 'Failed to add application') }); } finally { setSaving(false); }
  };

  const createScannedAccess = async (event: FormEvent) => {
    event.preventDefault();
    if (!scanAccessApplication||saving) return;
    if(isLLMAccessType(scanAccessType)){
      history.push(`/proxy?category=llm&new_application=${encodeURIComponent(scanAccessApplication.id)}&new_name=${encodeURIComponent(scanAccessName.trim()||suggestedScanAccessName)}`);
      setScanAccessApplication(undefined);return;
    }
    const expose = !isWebAccessType(scanAccessType) && (scanAccessType !== 'http' || webEntry.mode === 'port');
    setSaving(true);setScanAccessError('');
    try {
      let access=createdScanAccess;
      if(!access){
        const response = await createProxy({ name: scanAccessName.trim() || suggestedScanAccessName, application_id: scanAccessApplication.id, access_protocol: accessProtocolForType(scanAccessType), http_entry_mode: scanAccessType === 'http' ? webEntry.mode : undefined, expose_public_port: expose, port: expose && scanPublicPort ? Number(scanPublicPort) : undefined });
        if (response.code !== 200||!response.data) throw new Error(response.message);
        access=response.data;setCreatedScanAccess(access);
      }
      if(scanAccessType!=='webssh')await saveInitialConnection(access.id,scanAccessType,access.name,scanConnection);
      if(supportsInitialConnection(scanAccessType))setScanConnection(await loadAccessConnection(access.id,scanAccessType));
      const destination=supportsInitialConnection(scanAccessType)?await directAccessPath(access.id,scanAccessType):`/proxy?access_type=${scanAccessType}`;
      setScanAccessApplication(undefined);
      setCreatedScanAccess(undefined);setScanConnection(emptyConnection());
      history.push(destination);
    } catch { setScanAccessError(tr('未能完成连接配置，请检查后重试。已创建的访问会复用，不会重复创建。','Could not complete connection setup. Check and retry; an already-created access will be reused.')); }
    finally { setSaving(false); }
  };

  const closeScannedAccess = () => {
    if(saving)return;
    setScanAccessApplication(undefined);
    setCreatedScanAccess(undefined);setScanConnection(emptyConnection());setScanAccessError('');
    setNotice({ tone: 'success', text: tr('应用已添加，可稍后创建访问', 'Application added. You can create access later.') });
  };

  const openDiscovered = (application: string) => {
    setCreatedScanAccess(undefined);setScanConnection(emptyConnection());setScanAccessError('');
    const [ip, portRaw, provided] = application.split(':');
    const port = Number(portRaw);
    setDiscoveredForm({ name: `App-${ip}:${port}`, application_type: provided || portTypes[port] || 'tcp' });
    setDiscovered(application);
  };

  const installCommand = useMemo(() => {
    if (!keys) return '';
    return (installOS === 'windows' ? keys.windows_command : keys.command) || '';
  }, [installOS, keys]);
  const copyInstallCommand = async () => {
    await navigator.clipboard.writeText(installCommand);
    setInstallCopied(true);
    window.setTimeout(() => setInstallCopied(false), 1800);
  };

  const columns: Column<API.Edge>[] = [
    { key: 'name', title: tr('连接器名称', 'Connector'), width: 170, render: (row) => row.name },
    { key: 'device', title: tr('所在设备', 'Device'), width: 160, render: (row) => row.device?.name || '-' },
    { key: 'online', title: tr('在线状态', 'Online'), width: 90, render: (row) => <StatusPill tone={row.online === 1 ? 'success' : 'neutral'}>{row.online === 1 ? tr('在线', 'Online') : tr('离线', 'Offline')}</StatusPill> },
    { key: 'runtime', title: tr('运行状态', 'Runtime'), width: 100, render: (row) => <Switch checked={row.status === 1} disabled={togglingIds.includes(row.id)} aria-busy={togglingIds.includes(row.id)} aria-label={tr(`运行连接器：${row.name}`,`Run connector: ${row.name}`)} onChange={() => void toggle(row)}/> },
    { key: 'created', title: tr('创建时间', 'Created'), width: 150, render: (row) => row.created_at },
    { key: 'description', title: tr('描述', 'Description'), width: 180, render: (row) => row.description || '-' },
    { key: 'actions', title: tr('操作', 'Actions'), width: 175, render: (row) => <span className="liaison-table-actions"><button className="liaison-table-link" disabled={row.status !== 1 || row.online !== 1} onClick={() => void refreshScan(row)}>{tr('探测应用', 'Discover')}</button><button className="liaison-table-link" onClick={() => { setEditRow(row); setCreateName(row.name); setCreateDescription(row.description || ''); }}>{tr('编辑', 'Edit')}</button><ActionMenu label={tr(`更多操作：${row.name}`,`More actions: ${row.name}`)} items={[...(canUninstall?[{label:tr('卸载','Uninstall'),danger:true,onClick:()=>setUninstallRow(row)}]:[]),{label:tr('删除','Delete'),danger:true,onClick:()=>setDeleteRow(row)}]}/></span> },
  ];

  return <div className="liaison-page-stack">
    {uninstallRow&&canUninstall&&<Uninstall key={uninstallRow.id} edge={uninstallRow} onClose={()=>setUninstallRow(undefined)} onComplete={()=>void load()}/>}
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('连接器名称', 'Connector')}</span><input value={filters.name} onChange={(event) => { setFilters((value) => ({ ...value, name: event.target.value })); setPage(1); }} placeholder={tr('输入连接器名称', 'Connector name')} /></label><label className="liaison-compound"><span>{tr('所在设备', 'Device')}</span><input value={filters.device_name} onChange={(event) => { setFilters((value) => ({ ...value, device_name: event.target.value })); setPage(1); }} placeholder={tr('输入设备名称', 'Device name')} /></label><label className="liaison-compound"><span>{tr('在线状态', 'Online')}</span><select value={filters.online} onChange={(event) => { setFilters((value) => ({ ...value, online: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option><option value="1">{tr('在线', 'Online')}</option><option value="0">{tr('离线', 'Offline')}</option></select></label><label className="liaison-compound"><span>{tr('运行状态', 'Runtime')}</span><select value={filters.status} onChange={(event) => { setFilters((value) => ({ ...value, status: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option><option value="1">{tr('运行', 'Running')}</option><option value="2">{tr('停止', 'Stopped')}</option></select></label><div className="liaison-filter-actions"><Button onClick={() => { setFilters({ name: '', device_name: '', online: '', status: '' }); setPage(1); }}>{tr('重置', 'Reset')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('连接器列表', 'Connectors')}</h2><Button variant="primary" onClick={openCreate} disabled={saving}><Plus size={14} />{tr('新建连接器', 'Create connector')}</Button></header><DataTable columns={columns} rows={visibleRows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无连接器', 'No connectors')} /><Pager page={page} pageSize={pageSize} total={filteredRows.length} onPageChange={setPage} /></section>
    <Modal open={createOpen} title={`${tr('新建连接器', 'Create connector')} · ${keys ? '2 / 2' : '1 / 2'}`} onClose={closeCreate} width={560} className="is-connector-wizard" closeOnMask={false} footer={keys ? <Button variant="primary" onClick={closeCreate}><Check size={14} />{tr('完成', 'Done')}</Button> : createError ? <><Button onClick={closeCreate}>{tr('关闭', 'Close')}</Button><Button variant="primary" onClick={() => void createConnector(createName)}>{tr('重试', 'Try again')}</Button></> : saving ? undefined : <><Button onClick={closeCreate}>{tr('取消', 'Cancel')}</Button><Button variant="primary" onClick={() => void createConnector(createName)}>{tr('下一步', 'Next')}</Button></>}>
      <div className="liaison-connector-wizard">
        <div className="liaison-connector-steps">
          <div className={!keys ? 'is-active' : 'is-complete'}><span><b>1</b><strong>{tr('创建连接器', 'Create connector')}</strong></span><small>{tr('填写名称并创建连接器', 'Choose a name and create the connector')}</small></div>
          <div className={keys ? 'is-active' : ''}><span><b>2</b><strong>{tr('安装连接器', 'Install connector')}</strong></span><small>{tr('复制命令并在目标设备执行', 'Copy the command and run it on the target device')}</small></div>
        </div>
        {saving ? <div className="liaison-install-loading"><span className="ui-spinner" aria-hidden /><strong>{tr('正在创建连接器', 'Creating connector')}</strong><small>{tr('正在准备一次性接入凭据，请稍候…', 'Preparing one-time enrollment credentials…')}</small></div> : createError ? <div className="liaison-install-error"><Notice tone="danger">{createError}</Notice><p>{tr('连接器创建失败，你可以直接重试。', 'The connector could not be created. Try again now.')}</p></div> : keys ? <div className="liaison-install"><p className="liaison-install-intro">{tr('在目标设备上执行下方命令即可完成安装，安装完成后连接器会自动上线。', 'Run the command below on your target device to finish setup. The connector will come online automatically after installation.')}</p><div className="liaison-install-toolbar"><div className="liaison-choice-row"><button type="button" className={installOS === 'other' ? 'is-active' : ''} onClick={() => setInstallOS('other')}>Linux / macOS</button><button type="button" className={installOS === 'windows' ? 'is-active' : ''} onClick={() => setInstallOS('windows')}>Windows</button></div></div><div className="liaison-command-block"><div><span>{tr('安装命令', 'Install command')}</span><button type="button" disabled={!installCommand} onClick={() => void copyInstallCommand()}>{installCopied ? <Check size={13} /> : <Copy size={13} />}{installCopied ? tr('已复制', 'Copied') : tr('复制命令', 'Copy command')}</button></div><pre><code>{installCommand || tr('服务端未提供此系统的安装命令，请升级服务端。', 'The server did not provide an install command for this OS. Please upgrade the server.')}</code></pre></div><p className="liaison-install-warning">{tr('命令包含一次性连接密钥，关闭后无法再次查看。', 'This command contains one-time credentials and cannot be viewed again after closing.')}</p></div> : <div className="liaison-connector-create-step"><p>{tr('先创建连接器，再在目标设备上执行安装命令完成接入。', 'Create the connector first, then run the install command on your target device.')}</p><form className="liaison-connector-create-form" onSubmit={(event) => { event.preventDefault(); void createConnector(createName); }}><Field label={tr('连接器名称', 'Connector name')} hint={tr('留空时系统会自动使用上方唯一名称。', 'The suggested unique name is used automatically if left empty.')}><Input value={createName} onChange={(event) => setCreateName(event.target.value)} placeholder={suggestedConnectorName} /></Field></form></div>}
      </div>
    </Modal>
    <Modal open={!!editRow} title={tr('编辑连接器', 'Edit connector')} onClose={() => setEditRow(undefined)} width={500} footer={<><Button onClick={() => setEditRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-edge" disabled={saving}>{tr('确定', 'Save')}</Button></>}><form id="edit-edge" className="native-modal-form" onSubmit={update}><Field label={tr('连接器名称', 'Connector name')} required><Input value={createName} onChange={(event) => setCreateName(event.target.value)} /></Field><Field label={tr('描述', 'Description')}><Input value={createDescription} onChange={(event) => setCreateDescription(event.target.value)} /></Field></form></Modal>
    <Modal open={!!deleteRow} title={tr('删除连接器', 'Delete connector')} onClose={() => setDeleteRow(undefined)} width={450} footer={<><Button onClick={() => setDeleteRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={() => void remove()}>{tr('删除', 'Delete')}</Button></>}><DangerConfirm title={tr(`删除“${deleteRow?.name || ''}”？`, `Delete “${deleteRow?.name || ''}”?`)} description={tr('承载的应用、访问和密钥关系将一并移除，历史记录仍会保留。', 'Applications, access entries and key relations will be removed. History is retained.')} /></Modal>
    <Drawer open={!!scanRow} title={tr('探测应用', 'Discover applications')} onClose={closeScan}><div className="liaison-scan-overview"><div className="liaison-scan-overview-icon"><Radar size={17} /></div><div><strong>{scanRow?.name || '-'}</strong><p>{tr('探测设备上已安装的 Agent 和可接入的网络服务。', 'Discover installed Agents and network services through this connector.')}</p></div><StatusPill tone={scanTask?.task_status === 'completed' ? 'success' : scanTask?.task_status === 'failed' ? 'danger' : 'info'}>{tr('网络服务：', 'Network services: ')}{scanStatusLabel}</StatusPill></div>{scanRow&&<AgentProbe key={scanRow.id} edge={Number(scanRow.id)} revision={probeRevision}/>}<div className="liaison-scan-toolbar"><span>{tr('网络服务', 'Network services')} <b>{scanTask?.applications?.length || 0}</b></span><Button onClick={() => scanRow && void refreshScan(scanRow, true)} disabled={scanning}><Radar size={14} />{scanning ? tr('探测中', 'Discovering') : scanTask ? tr('重新探测', 'Discover again') : tr('探测', 'Discover')}</Button></div>{scanTask?.error ? <Notice tone="danger">{scanTask.error}</Notice> : null}<div className="liaison-scan-list"><div className="liaison-scan-list-head"><span>{tr('目标服务', 'Target')}</span><span>{tr('协议', 'Protocol')}</span><span /></div>{scanTask?.applications?.map((app) => { const [ip, port, type] = app.split(':'); const protocol = APPLICATION_TYPES.find((item) => item.value === (type || portTypes[Number(port)] || 'tcp'))?.label || 'TCP'; return <div className="liaison-scan-row" key={app}><span className="liaison-scan-target"><i><Server size={14} /></i><span><strong>{ip}</strong><small>{tr('端口', 'Port')} {port}</small></span></span><StatusPill>{protocol}</StatusPill><button type="button" className="liaison-table-link" onClick={() => openDiscovered(app)}><Plus size={13} />{tr('添加', 'Add')}</button></div>; })}{scanTask?.task_status === 'completed' && !scanTask.applications?.length ? <div className="liaison-scan-empty"><Radar size={20} /><strong>{tr('未发现可用应用', 'No applications discovered')}</strong><span>{tr('确认连接器在线后重新探测。', 'Make sure the connector is online, then try discovery again.')}</span></div> : null}</div></Drawer>
    <Modal open={!!discovered} title={tr('添加探测到的应用', 'Add discovered application')} onClose={() => setDiscovered(undefined)} width={500} footer={<><Button onClick={() => void addDiscovered(false)} disabled={saving}>{tr('添加应用', 'Add application')}</Button><Button variant="primary" onClick={() => void addDiscovered(true)} disabled={saving}>{tr('添加并创建访问', 'Add and create access')}</Button></>}><form className="liaison-scan-application-form" onSubmit={(event) => event.preventDefault()}><div className="liaison-scan-target-summary"><span>{tr('探测目标', 'Discovered target')}</span><strong>{parsedDiscovery?.ip}:{parsedDiscovery?.port}</strong></div><Field label={tr('应用名称', 'Application name')}><Input value={discoveredForm.name} onChange={(event) => setDiscoveredForm((value) => ({ ...value, name: event.target.value }))} /></Field><Field label={tr('应用类型', 'Application type')} required><Select value={discoveredForm.application_type} onChange={(event) => setDiscoveredForm((value) => ({ ...value, application_type: event.target.value }))}>{availableApplicationTypes().map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</Select></Field></form></Modal>
    <Modal open={!!scanAccessApplication} title={tr('创建访问', 'Create access')} onClose={closeScannedAccess} width={500} footer={<><Button onClick={closeScannedAccess}>{tr('暂不创建', 'Not now')}</Button><Button variant="primary" type="submit" form="scan-create-access" disabled={saving}>{supportsInitialConnection(scanAccessType)?tr('创建并访问','Create and open'):tr('创建访问', 'Create access')}</Button></>}><form id="scan-create-access" className="liaison-scan-access-form" onSubmit={createScannedAccess}><div className="liaison-scan-target-summary"><span>{tr('应用', 'Application')}</span><strong>{scanAccessApplication?.name}</strong></div><Field label={tr('访问名称', 'Access name')}><Input disabled={saving||!!createdScanAccess} value={scanAccessName} onChange={(event) => setScanAccessName(event.target.value)} placeholder={suggestedScanAccessName} /></Field><Field label={tr('访问类型', 'Access type')} required><Select disabled={saving||!!createdScanAccess} value={scanAccessType} onChange={(event) => { setScanAccessType(event.target.value as AccessType); setScanPublicPort('');setScanConnection(emptyConnection());setScanAccessError(''); }}>{scanAccessApplication ? accessTypesForApplication(scanAccessApplication.application_type).map((item) => <option key={item.value} value={item.value}>{item.label}</option>) : null}</Select></Field>{scanAccessType === 'http' && <div className="liaison-scan-entry-mode"><WebEntryModeField {...webEntry}/></div>}{!isWebAccessType(scanAccessType) && (scanAccessType !== 'http' || webEntry.mode === 'port') ? <div className="liaison-scan-port"><Field label={tr('访问端口', 'Access port')} hint={tr('留空自动分配', 'Leave empty for automatic assignment')}><Input type="number" min={1} max={65535} value={scanPublicPort} onChange={(event) => setScanPublicPort(event.target.value)} placeholder={tr('自动分配', 'Auto')} /></Field></div> : null}{(scanAccessType!=='webssh')&&<InitialConnectionFields type={scanAccessType} value={scanConnection} onChange={setScanConnection} disabled={saving}/>}{scanAccessError&&<Notice tone="danger">{scanAccessError}</Notice>}</form></Modal>
  </div>;
};

export default ConnectorPage;
