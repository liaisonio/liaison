import { Button, Column, DataTable, Drawer, Field, Input, Modal, Notice, Pager, StatusPill } from '@/components/ui';
import { useI18n } from '@/i18n';
import { history } from '@/lib/runtime';
import { createApplication, createEdge, createEdgeScanTask, deleteEdge, getEdgeList, getEdgeScanTask, updateEdge } from '@/services/api';
import { Check, Copy, Plus, Radar } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';

const pageSize = 10;
const portTypes: Record<number, string> = { 22: 'ssh', 80: 'http', 443: 'http', 3389: 'rdp', 5900: 'vnc', 3306: 'mysql', 5432: 'postgresql', 6379: 'redis', 27017: 'mongodb' };

const ConnectorPage: React.FC = () => {
  const { tr } = useI18n();
  const [rows, setRows] = useState<API.Edge[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: '', device_name: '' });
  const [applied, setApplied] = useState(filters);
  const [createOpen, setCreateOpen] = useState(false);
  const [createName, setCreateName] = useState('');
  const [createDescription, setCreateDescription] = useState('');
  const [keys, setKeys] = useState<API.EdgeCreateResult>();
  const [installOS, setInstallOS] = useState<'other' | 'windows'>('other');
  const [editRow, setEditRow] = useState<API.Edge>();
  const [deleteRow, setDeleteRow] = useState<API.Edge>();
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  const [scanRow, setScanRow] = useState<API.Edge>();
  const [scanTask, setScanTask] = useState<API.EdgeScanApplicationTask>();
  const [scanning, setScanning] = useState(false);
  const [discovered, setDiscovered] = useState<string>();

  const load = useCallback(async () => {
    setLoading(true);
    try { const response = await getEdgeList({ page, page_size: pageSize, name: applied.name || undefined, device_name: applied.device_name || undefined }); if (response.code !== 200) throw new Error(response.message); setRows(response.data?.edges || []); setTotal(response.data?.total || 0); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('加载连接器失败', 'Failed to load connectors') }); }
    finally { setLoading(false); }
  }, [applied, page, tr]);
  useEffect(() => { void load(); }, [load]);

  const create = async (event: FormEvent) => {
    event.preventDefault(); if (!createName.trim()) return; setSaving(true);
    try { const response = await createEdge({ name: createName.trim(), description: createDescription }); if (response.code !== 200 || !response.data) throw new Error(response.message); setKeys(response.data); setNotice({ tone: 'success', text: tr('连接器已创建', 'Connector created') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('创建失败', 'Create failed') }); } finally { setSaving(false); }
  };
  const closeCreate = () => { setCreateOpen(false); setCreateName(''); setCreateDescription(''); setKeys(undefined); setInstallOS('other'); };
  const update = async (event: FormEvent) => { event.preventDefault(); if (!editRow || !createName.trim()) return; setSaving(true); try { const response = await updateEdge(editRow.id, { name: createName.trim(), description: createDescription }); if (response.code !== 200) throw new Error(response.message); setEditRow(undefined); setNotice({ tone: 'success', text: tr('连接器已更新', 'Connector updated') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); } finally { setSaving(false); } };
  const toggle = async (row: API.Edge) => { try { const response = await updateEdge(row.id, { status: row.status === 1 ? 2 : 1 }); if (response.code !== 200) throw new Error(response.message); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('操作失败', 'Operation failed') }); } };
  const remove = async () => { if (!deleteRow) return; try { const response = await deleteEdge(deleteRow.id); if (response.code !== 200) throw new Error(response.message); setDeleteRow(undefined); setNotice({ tone: 'success', text: tr('连接器已删除', 'Connector deleted') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('删除失败', 'Delete failed') }); } };

  const refreshScan = async (edge: API.Edge, force = false) => {
    if (edge.status !== 1 || edge.online !== 1) { setNotice({ tone: 'danger', text: tr('连接器需处于启用且在线状态', 'Connector must be enabled and online') }); return; }
    setScanRow(edge); setScanning(true);
    try {
      if (force) { const created = await createEdgeScanTask({ edge_id: edge.id, protocol: 'tcp' }); if (created.code !== 200) throw new Error(created.message); }
      let response = await getEdgeScanTask(edge.id);
      if ((!response.data || response.code !== 200) && !force) { const created = await createEdgeScanTask({ edge_id: edge.id, protocol: 'tcp' }); if (created.code !== 200) throw new Error(created.message); await new Promise((resolve) => window.setTimeout(resolve, 900)); response = await getEdgeScanTask(edge.id); }
      if (response.code === 200 && response.data) setScanTask(response.data);
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('扫描失败', 'Scan failed') }); }
    finally { setScanning(false); }
  };

  const parsedDiscovery = useMemo(() => {
    if (!discovered) return undefined; const [ip, portRaw, provided] = discovered.split(':'); const port = Number(portRaw); return { ip, port, type: provided || portTypes[port] || 'tcp', name: `App-${ip}:${port}` };
  }, [discovered]);
  const addDiscovered = async (createAccess: boolean) => {
    if (!scanRow || !parsedDiscovery) return; setSaving(true);
    try { const response = await createApplication({ name: parsedDiscovery.name, application_type: parsedDiscovery.type, ip: parsedDiscovery.ip, port: parsedDiscovery.port, edge_id: scanRow.id }); if (response.code !== 200 || !response.data) throw new Error(response.message); setScanTask((task) => task ? { ...task, applications: task.applications.filter((item) => item !== discovered) } : task); setDiscovered(undefined); if (createAccess) history.push(`/proxy?application_id=${response.data.id}&application_name=${encodeURIComponent(response.data.name)}&autoCreate=true`); else setNotice({ tone: 'success', text: tr('应用已添加', 'Application added') }); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('添加应用失败', 'Failed to add application') }); } finally { setSaving(false); }
  };

  const installCommand = useMemo(() => {
    if (!keys) return ''; if (installOS === 'other') return keys.command || `curl -k -sSL ${window.location.origin}/install.sh | bash -s -- --access-key=${keys.access_key} --secret-key=${keys.secret_key} --server-http-addr=${window.location.host} --server-edge-addr=${window.location.hostname}:30012`;
    return `curl.exe -fsSL "${window.location.origin}/install.ps1" -o install.ps1; powershell -ExecutionPolicy Bypass -File install.ps1 -AccessKey "${keys.access_key}" -SecretKey "${keys.secret_key}" -ServerHttpAddr "${window.location.host}" -ServerEdgeAddr "${window.location.hostname}:30012"`;
  }, [installOS, keys]);
  const copy = async (value: string) => { await navigator.clipboard.writeText(value); setNotice({ tone: 'success', text: tr('已复制', 'Copied') }); };

  const columns: Column<API.Edge>[] = [
    { key: 'name', title: tr('连接器名称', 'Connector'), width: 170, render: (row) => row.name },
    { key: 'device', title: tr('所在设备', 'Device'), width: 160, render: (row) => row.device?.name || '-' },
    { key: 'description', title: tr('描述', 'Description'), width: 180, render: (row) => row.description || '-' },
    { key: 'online', title: tr('在线状态', 'Online'), width: 90, render: (row) => <StatusPill tone={row.online === 1 ? 'success' : 'neutral'}>{row.online === 1 ? tr('在线', 'Online') : tr('离线', 'Offline')}</StatusPill> },
    { key: 'runtime', title: tr('运行状态', 'Runtime'), width: 100, render: (row) => <button className={`liaison-switch${row.status === 1 ? ' is-on' : ''}`} onClick={() => void toggle(row)}><i /><span>{row.status === 1 ? tr('运行', 'On') : tr('停止', 'Off')}</span></button> },
    { key: 'created', title: tr('创建时间', 'Created'), width: 150, render: (row) => row.created_at },
    { key: 'actions', title: tr('操作', 'Actions'), width: 175, render: (row) => <span className="liaison-table-actions"><button className="liaison-table-link" disabled={row.status !== 1 || row.online !== 1} onClick={() => void refreshScan(row)}>{tr('扫描应用', 'Scan')}</button><button className="liaison-table-link" onClick={() => { setEditRow(row); setCreateName(row.name); setCreateDescription(row.description || ''); }}>{tr('编辑', 'Edit')}</button><button className="liaison-table-link is-danger" onClick={() => setDeleteRow(row)}>{tr('删除', 'Delete')}</button></span> },
  ];

  return <div className="liaison-page-stack">
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('连接器名称', 'Connector')}</span><input value={filters.name} onChange={(event) => setFilters((value) => ({ ...value, name: event.target.value }))} placeholder={tr('输入连接器名称', 'Connector name')} /></label><label className="liaison-compound"><span>{tr('所在设备', 'Device')}</span><input value={filters.device_name} onChange={(event) => setFilters((value) => ({ ...value, device_name: event.target.value }))} placeholder={tr('输入设备名称', 'Device name')} /></label><div className="liaison-filter-actions"><Button onClick={() => { const reset = { name: '', device_name: '' }; setFilters(reset); setApplied(reset); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('连接器列表', 'Connectors')}</h2><Button variant="primary" onClick={() => setCreateOpen(true)}><Plus size={14} />{tr('新建连接器', 'Create connector')}</Button></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无连接器', 'No connectors')} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
    <Modal open={createOpen} title={keys ? tr('安装连接器', 'Install connector') : tr('新建连接器', 'Create connector')} onClose={closeCreate} width={650} closeOnMask={!keys} footer={keys ? <Button variant="primary" onClick={closeCreate}><Check size={14} />{tr('完成', 'Done')}</Button> : <><Button onClick={closeCreate}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-edge" disabled={saving}>{tr('创建', 'Create')}</Button></>}>
      {!keys ? <form id="create-edge" className="native-modal-form" onSubmit={create}><Field label={tr('连接器名称', 'Connector name')} required><Input value={createName} onChange={(event) => setCreateName(event.target.value)} /></Field><Field label={tr('描述', 'Description')}><Input value={createDescription} onChange={(event) => setCreateDescription(event.target.value)} /></Field></form> : <div className="liaison-install"><Notice tone="success">{tr('连接器已创建。请复制安装命令并在目标设备执行。', 'Connector created. Run the install command on the target device.')}</Notice><div className="liaison-secret"><span>Access Key</span><code>{keys.access_key}</code><button onClick={() => void copy(keys.access_key)}><Copy size={14} /></button></div><div className="liaison-secret"><span>Secret Key</span><code>{keys.secret_key}</code><button onClick={() => void copy(keys.secret_key)}><Copy size={14} /></button></div><div className="liaison-choice-row"><button className={installOS === 'other' ? 'is-active' : ''} onClick={() => setInstallOS('other')}>Linux / macOS</button><button className={installOS === 'windows' ? 'is-active' : ''} onClick={() => setInstallOS('windows')}>Windows</button></div><div className="liaison-command"><code>{installCommand}</code><button onClick={() => void copy(installCommand)}><Copy size={14} /></button></div><Notice tone="warning">{tr('密钥关闭后无法再次查看，请妥善保管。', 'Keys cannot be viewed again after closing.')}</Notice></div>}
    </Modal>
    <Modal open={!!editRow} title={tr('编辑连接器', 'Edit connector')} onClose={() => setEditRow(undefined)} width={500} footer={<><Button onClick={() => setEditRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-edge" disabled={saving}>{tr('确定', 'Save')}</Button></>}><form id="edit-edge" className="native-modal-form" onSubmit={update}><Field label={tr('连接器名称', 'Connector name')} required><Input value={createName} onChange={(event) => setCreateName(event.target.value)} /></Field><Field label={tr('描述', 'Description')}><Input value={createDescription} onChange={(event) => setCreateDescription(event.target.value)} /></Field></form></Modal>
    <Modal open={!!deleteRow} title={tr('删除连接器', 'Delete connector')} onClose={() => setDeleteRow(undefined)} width={450} footer={<><Button onClick={() => setDeleteRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={() => void remove()}>{tr('删除', 'Delete')}</Button></>}><p className="native-confirm-copy">{tr('删除会连带移除该连接器承载的应用、访问和密钥关系，历史记录会保留。', 'Applications, access entries and key relations on this connector will be removed. History is retained.')}</p></Modal>
    <Drawer open={!!scanRow} title={`${tr('扫描应用', 'Scan applications')} · ${scanRow?.name || ''}`} onClose={() => setScanRow(undefined)}><div className="liaison-scan-toolbar"><StatusPill tone={scanTask?.task_status === 'completed' ? 'success' : 'info'}>{scanning ? tr('扫描中', 'Scanning') : scanTask?.task_status || tr('未开始', 'Not started')}</StatusPill><Button onClick={() => scanRow && void refreshScan(scanRow, true)} disabled={scanning}><Radar size={14} />{tr('重新扫描', 'Rescan')}</Button></div>{scanTask?.error ? <Notice tone="danger">{scanTask.error}</Notice> : null}<div className="liaison-scan-list">{scanTask?.applications?.map((app) => { const [ip, port, type] = app.split(':'); return <div key={app}><div><strong>{ip}:{port}</strong><span>{(type || portTypes[Number(port)] || 'tcp').toUpperCase()}</span></div><Button onClick={() => setDiscovered(app)}>{tr('添加', 'Add')}</Button></div>; })}{!scanning && !scanTask?.applications?.length ? <p>{tr('未发现可用应用', 'No applications discovered')}</p> : null}</div></Drawer>
    <Modal open={!!discovered} title={tr('添加扫描到的应用', 'Add discovered application')} onClose={() => setDiscovered(undefined)} width={470} footer={<><Button onClick={() => void addDiscovered(false)} disabled={saving}>{tr('只添加应用', 'Add only')}</Button><Button variant="primary" onClick={() => void addDiscovered(true)} disabled={saving}>{tr('添加并创建访问', 'Add and create access')}</Button></>}><dl className="liaison-detail-list"><div><dt>{tr('名称', 'Name')}</dt><dd>{parsedDiscovery?.name}</dd></div><div><dt>{tr('目标', 'Target')}</dt><dd>{parsedDiscovery?.ip}:{parsedDiscovery?.port}</dd></div><div><dt>{tr('类型', 'Type')}</dt><dd>{parsedDiscovery?.type.toUpperCase()}</dd></div></dl></Modal>
  </div>;
};

export default ConnectorPage;
