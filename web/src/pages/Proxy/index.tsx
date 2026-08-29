import { Button, Column, DataTable, Drawer, Field, Input, Modal, Notice, Pager, Select, StatusPill } from '@/components/ui';
import { ACCESS_TYPES, ACCESS_TYPES_CHANGED_EVENT, getProxyAccessType, isProxyPublicPortExposed } from '@/constants/accessTypes';
import { useI18n } from '@/i18n';
import { history, useSearchParams } from '@/lib/runtime';
import { createProxy, deleteProxy, deleteProxyFirewall, getApplicationList, getClientIP, getProxyFirewall, getProxyList, updateProxy, upsertProxyFirewall } from '@/services/api';
import { ExternalLink, Plus, Shield } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useState } from 'react';

const pageSize = 10;
const webProtocols = new Set(['ssh', 'rdp', 'vnc', 'mysql', 'postgresql', 'redis', 'mongodb', 'database']);

const ProxyPage: React.FC = () => {
  const { tr } = useI18n();
  const [routeSearch] = useSearchParams();
  const routeType = routeSearch.get('access_type') || '';
  const [rows, setRows] = useState<API.Proxy[]>([]);
  const [applications, setApplications] = useState<API.Application[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: '', access_type: routeType });
  const [applied, setApplied] = useState(filters);
  const [createOpen, setCreateOpen] = useState(false);
  const [editRow, setEditRow] = useState<API.Proxy>();
  const [deleteRow, setDeleteRow] = useState<API.Proxy>();
  const [form, setForm] = useState({ name: '', application_id: '', mode: 'webssh', port: '', description: '' });
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  const [firewallRow, setFirewallRow] = useState<API.Proxy>();
  const [cidrs, setCidrs] = useState<string[]>([]);
  const [cidrDraft, setCidrDraft] = useState('');
  const [clientIP, setClientIP] = useState('');

  useEffect(() => { setFilters((value) => ({ ...value, access_type: routeType })); setApplied((value) => ({ ...value, access_type: routeType })); setPage(1); }, [routeType]);
  const loadApplications = useCallback(async () => { try { const response = await getApplicationList({ page_size: 1000 }); if (response.code === 200) setApplications(response.data?.applications || []); } catch { setApplications([]); } }, []);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getProxyList({ page: 1, page_size: 1000, name: applied.name || undefined });
      if (response.code !== 200) throw new Error(response.message);
      const all = response.data?.proxies || [];
      const filtered = applied.access_type ? all.filter((row) => getProxyAccessType(row) === applied.access_type) : all;
      setTotal(filtered.length); setRows(filtered.slice((page - 1) * pageSize, page * pageSize));
      window.dispatchEvent(new CustomEvent(ACCESS_TYPES_CHANGED_EVENT));
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('加载访问失败', 'Failed to load access') }); }
    finally { setLoading(false); }
  }, [applied, page, tr]);
  useEffect(() => { void loadApplications(); }, [loadApplications]);
  useEffect(() => { void load(); }, [load]);

  const selectedApplication = useMemo(() => applications.find((item) => item.id === Number(form.application_id)), [applications, form.application_id]);
  const openCreate = () => { setForm({ name: '', application_id: '', mode: 'webssh', port: '', description: '' }); setCreateOpen(true); };
  const create = async (event: FormEvent) => {
    event.preventDefault(); if (!form.name.trim() || !selectedApplication) { setNotice({ tone: 'danger', text: tr('请选择应用并填写访问名称', 'Select an application and enter a name') }); return; }
    const sshWeb = selectedApplication.application_type === 'ssh' && form.mode === 'webssh';
    setSaving(true);
    try { const response = await createProxy({ name: form.name.trim(), description: form.description, application_id: selectedApplication.id, expose_public_port: !sshWeb, port: !sshWeb && form.port ? Number(form.port) : undefined }); if (response.code !== 200) throw new Error(response.message); setCreateOpen(false); setNotice({ tone: 'success', text: tr('访问已创建', 'Access created') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('创建失败', 'Create failed') }); } finally { setSaving(false); }
  };
  const update = async (event: FormEvent) => {
    event.preventDefault(); if (!editRow) return; setSaving(true);
    try { const response = await updateProxy(editRow.id, { name: form.name.trim(), description: form.description, port: form.port ? Number(form.port) : undefined, expose_public_port: isProxyPublicPortExposed(editRow) }); if (response.code !== 200) throw new Error(response.message); setEditRow(undefined); setNotice({ tone: 'success', text: tr('访问已更新', 'Access updated') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); } finally { setSaving(false); }
  };
  const remove = async () => { if (!deleteRow) return; try { const response = await deleteProxy(deleteRow.id); if (response.code !== 200) throw new Error(response.message); setDeleteRow(undefined); setNotice({ tone: 'success', text: tr('访问已删除', 'Access deleted') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('删除失败', 'Delete failed') }); } };
  const toggle = async (row: API.Proxy) => { try { const response = await updateProxy(row.id, { status: row.status === 'active' ? 'stopped' : 'active' }); if (response.code !== 200) throw new Error(response.message); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('状态更新失败', 'Status update failed') }); } };

  const openFirewall = async (row: API.Proxy) => {
    setFirewallRow(row); setCidrs([]); setCidrDraft('');
    try { const [rule, client] = await Promise.all([getProxyFirewall(row.id), getClientIP()]); if (rule.code === 200) setCidrs(rule.data?.allowed_cidrs || []); if (client.code === 200) setClientIP(client.data?.ip || ''); } catch { /* empty rule */ }
  };
  const addCIDR = () => { const value = cidrDraft.trim(); if (!value || cidrs.includes(value)) return; if (!/^([\da-f:.]+)(\/\d{1,3})?$/i.test(value)) { setNotice({ tone: 'danger', text: tr('请输入有效 IP 或 CIDR', 'Enter a valid IP or CIDR') }); return; } setCidrs((items) => [...items, value]); setCidrDraft(''); };
  const saveFirewall = async () => { if (!firewallRow) return; setSaving(true); try { const response = await upsertProxyFirewall(firewallRow.id, { allowed_cidrs: cidrs }); if (response.code !== 200) throw new Error(response.message); setFirewallRow(undefined); setNotice({ tone: 'success', text: tr('访问边界已更新', 'Access boundary updated') }); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('保存失败', 'Save failed') }); } finally { setSaving(false); } };
  const clearFirewall = async () => { if (!firewallRow) return; try { const response = await deleteProxyFirewall(firewallRow.id); if (response.code !== 200) throw new Error(response.message); setCidrs([]); setNotice({ tone: 'success', text: tr('访问边界已清除', 'Access boundary cleared') }); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('清除失败', 'Clear failed') }); } };

  const openAccess = (row: API.Proxy) => {
    const type = getProxyAccessType(row);
    if (type === 'webssh') history.push(`/webssh/${row.id}`);
    else if (row.application?.application_type === 'rdp' || row.application?.application_type === 'vnc') history.push(`/webdesktop/${row.id}`);
    else if (['mysql', 'postgresql', 'redis', 'mongodb', 'database'].includes(row.application?.application_type || '')) history.push(`/webdata/${row.id}`);
    else if (row.access_url) window.open(row.access_url, '_blank', 'noopener,noreferrer');
  };

  const columns: Column<API.Proxy>[] = [
    { key: 'name', title: tr('访问名称', 'Access'), width: 175, render: (row) => row.name },
    { key: 'type', title: tr('类型', 'Type'), width: 90, render: (row) => <StatusPill tone="info">{(getProxyAccessType(row) || '-').toUpperCase()}</StatusPill> },
    { key: 'application', title: tr('应用', 'Application'), width: 175, render: (row) => row.application?.name || '-' },
    { key: 'target', title: tr('目标', 'Target'), width: 140, render: (row) => row.application ? <code>{row.application.ip}:{row.application.port}</code> : '-' },
    { key: 'public', title: tr('公网端口', 'Public port'), width: 95, render: (row) => isProxyPublicPortExposed(row) ? row.port || tr('自动', 'Auto') : '-' },
    { key: 'status', title: tr('状态', 'Status'), width: 90, render: (row) => <StatusPill tone={row.effective_status === 'active' || row.status === 'active' ? 'success' : 'neutral'}>{row.effective_status_message || row.status}</StatusPill> },
    { key: 'actions', title: tr('操作', 'Actions'), width: 245, render: (row) => <span className="liaison-table-actions">{webProtocols.has(row.application?.application_type || '') || row.access_url ? <button className="liaison-table-link" onClick={() => openAccess(row)}>{tr('访问', 'Open')}</button> : null}<button className="liaison-table-link" onClick={() => void toggle(row)}>{row.status === 'active' ? tr('停用', 'Stop') : tr('启用', 'Start')}</button>{isProxyPublicPortExposed(row) ? <button className="liaison-table-link" onClick={() => void openFirewall(row)}>{tr('边界', 'Boundary')}</button> : null}<button className="liaison-table-link" onClick={() => { setEditRow(row); setForm({ name: row.name, application_id: String(row.application?.id || ''), mode: getProxyAccessType(row) === 'webssh' ? 'webssh' : 'ssh', port: row.port ? String(row.port) : '', description: row.description || '' }); }}>{tr('编辑', 'Edit')}</button><button className="liaison-table-link is-danger" onClick={() => setDeleteRow(row)}>{tr('删除', 'Delete')}</button></span> },
  ];

  const accessForm = (id: string, submit: (event: FormEvent) => void, editing = false) => <form id={id} className="native-modal-form" onSubmit={submit}><Field label={tr('访问名称', 'Access name')} required><Input value={form.name} onChange={(event) => setForm((value) => ({ ...value, name: event.target.value }))} /></Field>{!editing ? <Field label={tr('应用', 'Application')} required><Select value={form.application_id} onChange={(event) => { const app = applications.find((item) => item.id === Number(event.target.value)); setForm((value) => ({ ...value, application_id: event.target.value, name: value.name || app?.name || '', mode: app?.application_type === 'ssh' ? 'webssh' : 'ssh' })); }}><option value="">{tr('选择应用', 'Select application')}</option>{applications.filter((item) => !item.proxy).map((item) => <option key={item.id} value={item.id}>{item.name} · {item.application_type.toUpperCase()} · {item.ip}:{item.port}</option>)}</Select></Field> : null}{selectedApplication?.application_type === 'ssh' && !editing ? <Field label={tr('访问方式', 'Access mode')}><div className="liaison-choice-row"><button type="button" className={form.mode === 'ssh' ? 'is-active' : ''} onClick={() => setForm((value) => ({ ...value, mode: 'ssh' }))}>SSH</button><button type="button" className={form.mode === 'webssh' ? 'is-active' : ''} onClick={() => setForm((value) => ({ ...value, mode: 'webssh' }))}>WebSSH</button></div></Field> : null}{(editing ? isProxyPublicPortExposed(editRow) : selectedApplication && !(selectedApplication.application_type === 'ssh' && form.mode === 'webssh')) ? <Field label={tr('公网端口', 'Public port')} hint={tr('留空自动分配', 'Leave empty for automatic assignment')}><Input type="number" min={1} max={65535} value={form.port} onChange={(event) => setForm((value) => ({ ...value, port: event.target.value }))} /></Field> : null}<Field label={tr('描述', 'Description')}><Input value={form.description} onChange={(event) => setForm((value) => ({ ...value, description: event.target.value }))} /></Field></form>;

  return <div className="liaison-page-stack">
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('访问名称', 'Access')}</span><input value={filters.name} onChange={(event) => setFilters((value) => ({ ...value, name: event.target.value }))} placeholder={tr('输入访问名称', 'Access name')} /></label><label className="liaison-compound"><span>{tr('类型', 'Type')}</span><select value={filters.access_type} onChange={(event) => setFilters((value) => ({ ...value, access_type: event.target.value }))}><option value="">{tr('全部', 'All')}</option>{ACCESS_TYPES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label><div className="liaison-filter-actions"><Button onClick={() => { const reset = { name: '', access_type: routeType }; setFilters(reset); setApplied(reset); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('访问列表', 'Access')}</h2><Button variant="primary" onClick={openCreate}><Plus size={14} />{tr('新建访问', 'Create access')}</Button></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无访问', 'No access')} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
    <Modal open={createOpen} title={tr('新建访问', 'Create access')} onClose={() => setCreateOpen(false)} width={560} footer={<><Button onClick={() => setCreateOpen(false)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-proxy" disabled={saving}>{tr('确定', 'Create')}</Button></>}>{accessForm('create-proxy', create)}</Modal>
    <Modal open={!!editRow} title={tr('编辑访问', 'Edit access')} onClose={() => setEditRow(undefined)} width={520} footer={<><Button onClick={() => setEditRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-proxy" disabled={saving}>{tr('确定', 'Save')}</Button></>}>{accessForm('edit-proxy', update, true)}</Modal>
    <Modal open={!!deleteRow} title={tr('删除访问', 'Delete access')} onClose={() => setDeleteRow(undefined)} width={430} footer={<><Button onClick={() => setDeleteRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="danger" onClick={() => void remove()}>{tr('删除', 'Delete')}</Button></>}><p className="native-confirm-copy">{tr(`确定删除访问“${deleteRow?.name || ''}”吗？`, `Delete access “${deleteRow?.name || ''}”?`)}</p></Modal>
    <Drawer open={!!firewallRow} title={tr('访问边界', 'Access boundary')} onClose={() => setFirewallRow(undefined)}><Notice>{tr('仅允许列表内的 IP/CIDR 访问此公网端口。列表为空时不限制。', 'Only listed IPs/CIDRs may reach this public port. Empty means unrestricted.')}</Notice>{clientIP ? <Button className="liaison-firewall-current" onClick={() => { if (!cidrs.includes(`${clientIP}/32`)) setCidrs((items) => [...items, `${clientIP}/32`]); }}><Shield size={14} />{tr('添加当前 IP', 'Add current IP')} · {clientIP}</Button> : null}<div className="liaison-firewall-add"><Input value={cidrDraft} onChange={(event) => setCidrDraft(event.target.value)} placeholder="203.0.113.8/32" onKeyDown={(event) => { if (event.key === 'Enter') { event.preventDefault(); addCIDR(); } }} /><Button onClick={addCIDR}>{tr('添加', 'Add')}</Button></div><div className="liaison-cidr-list">{cidrs.map((cidr) => <div key={cidr}><code>{cidr}</code><button onClick={() => setCidrs((items) => items.filter((item) => item !== cidr))}>×</button></div>)}{cidrs.length === 0 ? <p>{tr('当前未设置限制', 'No restrictions')}</p> : null}</div><div className="liaison-drawer-actions"><Button variant="danger" onClick={() => void clearFirewall()}>{tr('清除规则', 'Clear')}</Button><Button variant="primary" onClick={() => void saveFirewall()} disabled={saving}>{tr('保存', 'Save')}</Button></div></Drawer>
  </div>;
};

export default ProxyPage;
