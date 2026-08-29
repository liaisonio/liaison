import { Button, Column, DataTable, Field, Input, Modal, Notice, Pager, Select, StatusPill } from '@/components/ui';
import { ACCESS_TYPES_CHANGED_EVENT } from '@/constants/accessTypes';
import { APPLICATION_TYPES, APPLICATION_TYPES_CHANGED_EVENT } from '@/constants/applicationTypes';
import { useI18n } from '@/i18n';
import { useSearchParams } from '@/lib/runtime';
import { createApplication, createProxy, deleteApplication, getApplicationList, getEdgeList, updateApplication } from '@/services/api';
import { AppWindow, Link2, Plus } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';

const pageSize = 10;
const emptyApplication = { name: '', application_type: 'tcp', edge_id: '', ip: '', port: '' };

const AppPage: React.FC = () => {
  const { tr } = useI18n();
  const [routeSearch] = useSearchParams();
  const [rows, setRows] = useState<API.Application[]>([]);
  const [edges, setEdges] = useState<API.Edge[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const routeType = routeSearch.get('application_type') || '';
  const [filters, setFilters] = useState({ name: '', application_type: routeType, device_name: '' });
  const [applied, setApplied] = useState(filters);
  const [createOpen, setCreateOpen] = useState(false);
  const [editRow, setEditRow] = useState<API.Application>();
  const [deleteRow, setDeleteRow] = useState<API.Application>();
  const [accessRow, setAccessRow] = useState<API.Application>();
  const [form, setForm] = useState(emptyApplication);
  const [editName, setEditName] = useState('');
  const [accessName, setAccessName] = useState('');
  const [accessMode, setAccessMode] = useState<'ssh' | 'webssh'>('ssh');
  const [publicPort, setPublicPort] = useState('');
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();

  useEffect(() => {
    setFilters((value) => ({ ...value, application_type: routeType }));
    setApplied((value) => ({ ...value, application_type: routeType }));
    setPage(1);
  }, [routeType]);

  const loadEdges = useCallback(async () => {
    try { const response = await getEdgeList({ page_size: 1000 }); if (response.code === 200) setEdges(response.data?.edges || []); } catch { setEdges([]); }
  }, []);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getApplicationList({ page, page_size: pageSize, name: applied.name || undefined, application_type: applied.application_type || undefined, device_name: applied.device_name || undefined });
      if (response.code !== 200) throw new Error(response.message);
      setRows(response.data?.applications || []); setTotal(response.data?.total || 0);
      window.dispatchEvent(new CustomEvent(APPLICATION_TYPES_CHANGED_EVENT));
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('加载应用失败', 'Failed to load applications') }); }
    finally { setLoading(false); }
  }, [applied, page, tr]);
  useEffect(() => { void loadEdges(); }, [loadEdges]);
  useEffect(() => { void load(); }, [load]);

  const availableIPs = useMemo(() => {
    const edge = edges.find((item) => item.id === Number(form.edge_id));
    return edge?.device?.interfaces?.flatMap((item) => item.ip || []).filter((ip) => !ip.includes(':')) || [];
  }, [edges, form.edge_id]);

  const create = async (event: FormEvent) => {
    event.preventDefault();
    if (!form.name.trim() || !form.edge_id || !form.ip.trim() || !Number(form.port)) { setNotice({ tone: 'danger', text: tr('请填写全部必填项', 'Complete all required fields') }); return; }
    setSaving(true);
    try {
      const response = await createApplication({ name: form.name.trim(), application_type: form.application_type, edge_id: Number(form.edge_id), ip: form.ip.trim(), port: Number(form.port) });
      if (response.code !== 200) throw new Error(response.message);
      setCreateOpen(false); setForm(emptyApplication); setNotice({ tone: 'success', text: tr('应用已创建', 'Application created') }); await load();
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('创建失败', 'Create failed') }); }
    finally { setSaving(false); }
  };
  const update = async (event: FormEvent) => {
    event.preventDefault(); if (!editRow || !editName.trim()) return; setSaving(true);
    try { const response = await updateApplication(editRow.id, { name: editName.trim() }); if (response.code !== 200) throw new Error(response.message); setEditRow(undefined); setNotice({ tone: 'success', text: tr('应用已更新', 'Application updated') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); } finally { setSaving(false); }
  };
  const remove = async () => {
    if (!deleteRow) return;
    try { const response = await deleteApplication(deleteRow.id); if (response.code !== 200) throw new Error(response.message); setDeleteRow(undefined); setNotice({ tone: 'success', text: tr('应用已删除', 'Application deleted') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('删除失败', 'Delete failed') }); }
  };
  const openAccess = (row: API.Application) => { setAccessRow(row); setAccessName(row.name); setAccessMode('ssh'); setPublicPort(''); };
  const createAccess = async (event: FormEvent) => {
    event.preventDefault(); if (!accessRow || !accessName.trim()) return;
    const isSSH = accessRow.application_type === 'ssh';
    const expose = !isSSH || accessMode === 'ssh';
    setSaving(true);
    try {
      const response = await createProxy({ name: accessName.trim(), application_id: accessRow.id, expose_public_port: expose, port: expose && publicPort ? Number(publicPort) : undefined });
      if (response.code !== 200) throw new Error(response.message);
      setAccessRow(undefined); setNotice({ tone: 'success', text: tr('访问已创建', 'Access created') }); window.dispatchEvent(new CustomEvent(ACCESS_TYPES_CHANGED_EVENT)); await load();
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('创建访问失败', 'Failed to create access') }); }
    finally { setSaving(false); }
  };

  const columns: Column<API.Application>[] = [
    { key: 'name', title: tr('应用名称', 'Application'), width: 190, render: (row) => <span className="liaison-inline-name"><AppWindow size={14} />{row.name}</span> },
    { key: 'type', title: tr('类型', 'Type'), width: 95, render: (row) => <StatusPill tone="info">{row.application_type.toUpperCase()}</StatusPill> },
    { key: 'target', title: tr('目标', 'Target'), width: 155, render: (row) => <code>{row.ip}:{row.port}</code> },
    { key: 'device', title: tr('所在设备', 'Device'), width: 150, render: (row) => row.device?.name || '-' },
    { key: 'proxy', title: tr('关联访问', 'Access'), width: 170, render: (row) => row.proxy ? <StatusPill tone="success">{row.proxy.name}</StatusPill> : '-' },
    { key: 'created', title: tr('创建时间', 'Created'), width: 150, render: (row) => row.created_at },
    { key: 'actions', title: tr('操作', 'Actions'), width: 180, render: (row) => <span className="liaison-table-actions">{!row.proxy ? <button className="liaison-table-link" onClick={() => openAccess(row)}>{tr('创建访问', 'Create access')}</button> : null}<button className="liaison-table-link" onClick={() => { setEditRow(row); setEditName(row.name); }}>{tr('编辑', 'Edit')}</button><button className="liaison-table-link is-danger" onClick={() => setDeleteRow(row)}>{tr('删除', 'Delete')}</button></span> },
  ];

  return <div className="liaison-page-stack">
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('应用名称', 'Application')}</span><input value={filters.name} onChange={(event) => setFilters((value) => ({ ...value, name: event.target.value }))} placeholder={tr('输入应用名称', 'Application name')} /></label><label className="liaison-compound"><span>{tr('类型', 'Type')}</span><select value={filters.application_type} onChange={(event) => setFilters((value) => ({ ...value, application_type: event.target.value }))}><option value="">{tr('全部', 'All')}</option>{APPLICATION_TYPES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label><label className="liaison-compound"><span>{tr('所在设备', 'Device')}</span><input value={filters.device_name} onChange={(event) => setFilters((value) => ({ ...value, device_name: event.target.value }))} placeholder={tr('输入设备名称', 'Device name')} /></label><div className="liaison-filter-actions"><Button onClick={() => { const reset = { name: '', application_type: routeType, device_name: '' }; setFilters(reset); setApplied(reset); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('应用列表', 'Applications')}</h2><Button variant="primary" onClick={() => setCreateOpen(true)}><Plus size={14} />{tr('新建应用', 'Create application')}</Button></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无应用', 'No applications')} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
    <Modal open={createOpen} title={tr('新建应用', 'Create application')} onClose={() => setCreateOpen(false)} width={650} footer={<><Button onClick={() => setCreateOpen(false)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-application" disabled={saving}>{tr('确定', 'Create')}</Button></>}><form id="create-application" className="liaison-form-grid" onSubmit={create}><Field label={tr('应用名称', 'Application name')} required><Input value={form.name} onChange={(event) => setForm((value) => ({ ...value, name: event.target.value }))} /></Field><Field label={tr('应用类型', 'Application type')}><Select value={form.application_type} onChange={(event) => setForm((value) => ({ ...value, application_type: event.target.value }))}>{APPLICATION_TYPES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</Select></Field><div className="is-full"><Field label={tr('连接器', 'Connector')} required hint={tr('应用通过该连接器访问所在局域网', 'The application uses this connector to reach its LAN')}><Select value={form.edge_id} onChange={(event) => setForm((value) => ({ ...value, edge_id: event.target.value, ip: '' }))}><option value="">{tr('选择连接器', 'Select connector')}</option>{edges.map((edge) => <option key={edge.id} value={edge.id}>{edge.name}{edge.device?.name ? ` (${edge.device.name})` : ''}</option>)}</Select></Field></div><Field label={tr('IP 地址', 'IP address')} required><Input list="application-ip-options" value={form.ip} onChange={(event) => setForm((value) => ({ ...value, ip: event.target.value }))} placeholder="192.168.1.100" /><datalist id="application-ip-options">{availableIPs.map((ip) => <option key={ip} value={ip} />)}</datalist></Field><Field label={tr('端口', 'Port')} required><Input type="number" min={1} max={65535} value={form.port} onChange={(event) => setForm((value) => ({ ...value, port: event.target.value }))} /></Field></form></Modal>
    <Modal open={!!editRow} title={tr('编辑应用', 'Edit application')} onClose={() => setEditRow(undefined)} width={460} footer={<><Button onClick={() => setEditRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-application">{tr('确定', 'Save')}</Button></>}><form id="edit-application" onSubmit={update}><Field label={tr('应用名称', 'Application name')} required><Input value={editName} onChange={(event) => setEditName(event.target.value)} /></Field></form></Modal>
    <Modal open={!!deleteRow} title={tr('删除应用', 'Delete application')} onClose={() => setDeleteRow(undefined)} width={430} footer={<><Button onClick={() => setDeleteRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={() => void remove()}>{tr('删除', 'Delete')}</Button></>}><p className="native-confirm-copy">{tr(`确定删除应用“${deleteRow?.name || ''}”吗？`, `Delete application “${deleteRow?.name || ''}”?`)}</p></Modal>
    <Modal open={!!accessRow} title={tr('为应用创建访问', 'Create application access')} onClose={() => setAccessRow(undefined)} width={520} footer={<><Button onClick={() => setAccessRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-access" disabled={saving}><Link2 size={14} />{tr('创建', 'Create')}</Button></>}><form id="create-access" className="native-modal-form" onSubmit={createAccess}><Field label={tr('访问名称', 'Access name')} required><Input value={accessName} onChange={(event) => setAccessName(event.target.value)} /></Field>{accessRow?.application_type === 'ssh' ? <Field label={tr('访问方式', 'Access mode')}><div className="liaison-choice-row"><button type="button" className={accessMode === 'ssh' ? 'is-active' : ''} onClick={() => setAccessMode('ssh')}>SSH</button><button type="button" className={accessMode === 'webssh' ? 'is-active' : ''} onClick={() => setAccessMode('webssh')}>WebSSH</button></div><small>{tr('SSH 开放公网端口；WebSSH 仅通过浏览器访问。', 'SSH exposes a public port; WebSSH is browser-only.')}</small></Field> : null}{(accessRow?.application_type !== 'ssh' || accessMode === 'ssh') ? <Field label={tr('公网端口', 'Public port')} hint={tr('留空自动分配', 'Leave empty to assign automatically')}><Input type="number" min={1} max={65535} value={publicPort} onChange={(event) => setPublicPort(event.target.value)} /></Field> : null}</form></Modal>
  </div>;
};

export default AppPage;
