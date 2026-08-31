import { Button, Column, DangerConfirm, DataTable, Drawer, Field, Input, Modal, Notice, Pager, Select, StatusPill, Timestamp } from '@/components/ui';
import { ACCESS_TYPES, ACCESS_TYPES_CHANGED_EVENT, accessProtocolForType, accessTypeLabel, applicationTypeForAccess, getProxyAccessType, isAccessType, isProxyPublicPortExposed, isWebAccessType } from '@/constants/accessTypes';
import { useI18n } from '@/i18n';
import { history, useSearchParams } from '@/lib/runtime';
import { createProxy, deleteProxy, deleteProxyFirewall, getApplicationList, getClientIP, getProxyFirewall, getProxyList, updateProxy, upsertProxyFirewall } from '@/services/api';
import { Check, Copy, Globe2, Plus, Shield, Terminal, Trash2 } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

const pageSize = 10;
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
  const [form, setForm] = useState({ name: '', application_id: '', access_type: routeType, port: '', description: '' });
  const [suggestedAccessName, setSuggestedAccessName] = useState(defaultAccessName);
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();
  const [firewallRow, setFirewallRow] = useState<API.Proxy>();
  const [cidrs, setCidrs] = useState<string[]>([]);
  const [cidrDraft, setCidrDraft] = useState('');
  const [clientIP, setClientIP] = useState('');
  const [firewallUpdatedAt, setFirewallUpdatedAt] = useState('');
  const [firewallDirty, setFirewallDirty] = useState(false);

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

  const selectedAccessType = routeType || form.access_type;
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
    return applications.filter((item) => item.application_type === applicationType);
  }, [applications, selectedAccessType]);
  const selectedApplication = useMemo(() => applications.find((item) => String(item.id) === form.application_id), [applications, form.application_id]);
  const openCreate = () => {
    setSuggestedAccessName(defaultAccessName());
    setForm({ name: '', application_id: '', access_type: routeType, port: '', description: '' });
    setCreateOpen(true);
  };
  const create = async (event: FormEvent) => {
    event.preventDefault();
    if (!selectedAccessType || !selectedApplication) { setNotice({ tone: 'danger', text: tr('请选择访问协议和应用', 'Select an access protocol and application') }); return; }
    if (!isAccessType(selectedAccessType)) return;
    const webOnly = isWebAccessType(selectedAccessType);
    setSaving(true);
    const accessProtocol = accessProtocolForType(selectedAccessType);
    try { const response = await createProxy({ name: form.name.trim() || suggestedAccessName, description: form.description, application_id: selectedApplication.id, access_protocol: accessProtocol, expose_public_port: !webOnly, port: !webOnly && form.port ? Number(form.port) : undefined }); if (response.code !== 200) throw new Error(response.message); setCreateOpen(false); setNotice({ tone: 'success', text: tr('访问已创建', 'Access created') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('创建失败', 'Create failed') }); } finally { setSaving(false); }
  };
  const update = async (event: FormEvent) => {
    event.preventDefault(); if (!editRow) return; setSaving(true);
    try { const response = await updateProxy(editRow.id, { name: form.name.trim(), description: form.description, port: form.port ? Number(form.port) : undefined, expose_public_port: isProxyPublicPortExposed(editRow) }); if (response.code !== 200) throw new Error(response.message); setEditRow(undefined); setNotice({ tone: 'success', text: tr('访问已更新', 'Access updated') }); await load(); }
    catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); } finally { setSaving(false); }
  };
  const remove = async () => { if (!deleteRow) return; try { const response = await deleteProxy(deleteRow.id); if (response.code !== 200) throw new Error(response.message); setDeleteRow(undefined); setNotice({ tone: 'success', text: tr('访问已删除', 'Access deleted') }); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('删除失败', 'Delete failed') }); } };
  const toggle = async (row: API.Proxy) => { try { const response = await updateProxy(row.id, { status: row.status === 'running' ? 'stopped' : 'running' }); if (response.code !== 200) throw new Error(response.message); await load(); } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('状态更新失败', 'Status update failed') }); } };

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

  const openAccess = (row: API.Proxy) => {
    const type = getProxyAccessType(row);
    const search = routeSearch.toString();
    const returnTo = `/proxy${search ? `?${search}` : ''}`;
    const internalPath = (path: string) => `${path}?from=${encodeURIComponent(returnTo)}`;
    if (type === 'webssh') history.push(internalPath(`/webssh/${row.id}`));
    else if (type === 'webrdp' || type === 'webvnc') history.push(internalPath(`/webdesktop/${row.id}`));
    else if (['webmysql', 'webpostgresql', 'webredis', 'webmongodb'].includes(type || '')) history.push(internalPath(`/webdata/${row.id}`));
    else if (row.access_url) window.open(row.access_url, '_blank', 'noopener,noreferrer');
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
    { key: 'application', title: tr('应用', 'Application'), width: 175, render: (row) => row.application?.name || '-' },
    { key: 'application_protocol', title: tr('应用协议', 'Application protocol'), width: 110, render: (row) => <StatusPill tone="neutral">{accessTypeLabel(row.application?.application_type)}</StatusPill> },
    ...endpointColumn,
    { key: 'enabled', title: tr('启用', 'Enabled'), width: 82, render: (row) => <button className={`liaison-switch${row.status === 'running' ? ' is-on' : ''}`} title={row.status === 'running' ? tr('点击停用', 'Click to disable') : tr('点击启用', 'Click to enable')} aria-label={row.status === 'running' ? tr('停用访问', 'Disable access') : tr('启用访问', 'Enable access')} aria-pressed={row.status === 'running'} onClick={() => void toggle(row)}><i /><span>{row.status === 'running' ? tr('启用', 'On') : tr('停用', 'Off')}</span></button> },
    { key: 'created', title: tr('创建时间', 'Created'), width: 150, render: (row) => <Timestamp value={row.created_at} /> },
    { key: 'description', title: tr('描述', 'Description'), width: 180, render: (row) => row.description || '-' },
    { key: 'actions', title: tr('操作', 'Actions'), width: 285, fixed: 'right', render: (row) => <span className="liaison-table-actions">{isProxyPublicPortExposed(row) && getProxyAccessType(row) !== 'http' ? <ConnectionCommand row={row} commandLabel={tr('连接命令', 'Command')} exampleLabel={tr('连接示例', 'Connection example')} copyHintLabel={tr('点击复制', 'Click to copy')} copiedLabel={tr('已复制', 'Copied')} /> : <button className="liaison-table-link" onClick={() => openAccess(row)}>{isWebAccessType(getProxyAccessType(row)) ? tr('详情', 'Details') : tr('访问', 'Open')}</button>}{isProxyPublicPortExposed(row) ? <button className="liaison-table-link" onClick={() => void openFirewall(row)}>{tr('防火墙', 'Firewall')}</button> : null}<button className="liaison-table-link" onClick={() => { setEditRow(row); setForm({ name: row.name, application_id: String(row.application?.id || ''), access_type: getProxyAccessType(row) || row.application?.application_type || '', port: row.port ? String(row.port) : '', description: row.description || '' }); }}>{tr('编辑', 'Edit')}</button><button className="liaison-table-link is-danger" onClick={() => setDeleteRow(row)}>{tr('删除', 'Delete')}</button></span> },
  ];

  const accessForm = (id: string, submit: (event: FormEvent) => void, editing = false) => {
    const exposesPublicPort = editing ? isProxyPublicPortExposed(editRow) : Boolean(selectedApplication && !isWebAccessType(selectedAccessType));
    return <form id={id} className={`liaison-access-form${editing ? ' is-editing' : ''}${routeType ? ' is-protocol-fixed' : ''}`} onSubmit={submit}>
      <Field label={tr('访问名称', 'Access name')}><Input value={form.name} onChange={(event) => setForm((value) => ({ ...value, name: event.target.value }))} placeholder={editing ? undefined : suggestedAccessName} /></Field>
      {!editing ? <Field label={tr('访问协议', 'Protocol')} required><Select value={selectedAccessType} disabled={Boolean(routeType)} onChange={(event) => setForm((value) => ({ ...value, access_type: event.target.value, application_id: '', port: '' }))}><option value="">{tr('选择协议', 'Select protocol')}</option>{ACCESS_TYPES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</Select></Field> : null}
      {!editing ? <div className="is-full"><Field label={tr('应用', 'Application')} required hint={!selectedAccessType ? tr('请先选择访问协议', 'Select a protocol first') : availableApplications.length === 0 ? tr('该协议暂无可用应用', 'No available applications for this protocol') : undefined}><Select value={form.application_id} disabled={!selectedAccessType || availableApplications.length === 0} onChange={(event) => setForm((value) => ({ ...value, application_id: event.target.value }))}><option value="">{!selectedAccessType ? tr('先选择协议', 'Select protocol first') : tr('选择应用', 'Select application')}</option>{availableApplications.map((item) => <option key={item.id} value={item.id}>{item.name} · {item.ip}:{item.port}</option>)}</Select></Field></div> : null}
      <div className={editing || !exposesPublicPort ? 'is-full' : 'liaison-access-description'}><Field label={tr('描述', 'Description')}><Input value={form.description} onChange={(event) => setForm((value) => ({ ...value, description: event.target.value }))} placeholder={tr('选填', 'Optional')} /></Field></div>
      {exposesPublicPort ? <div className="liaison-access-port"><Field label={tr('访问端口', 'Access port')} hint={tr('留空自动分配', 'Leave empty for automatic assignment')}><Input type="number" min={1} max={65535} value={form.port} onChange={(event) => setForm((value) => ({ ...value, port: event.target.value }))} placeholder={tr('自动分配', 'Auto')} /></Field></div> : null}
    </form>;
  };

  return <div className="liaison-page-stack">
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar"><label className="liaison-compound"><span>{tr('访问名称', 'Access')}</span><input value={filters.name} onChange={(event) => setFilters((value) => ({ ...value, name: event.target.value }))} placeholder={tr('输入访问名称', 'Access name')} /></label>{!routeType ? <label className="liaison-compound"><span>{tr('协议', 'Protocol')}</span><select value={filters.access_type} onChange={(event) => setFilters((value) => ({ ...value, access_type: event.target.value }))}><option value="">{tr('全部', 'All')}</option>{ACCESS_TYPES.map((item) => <option key={item.value} value={item.value}>{item.label}</option>)}</select></label> : null}<div className="liaison-filter-actions"><Button onClick={() => { const reset = { name: '', access_type: routeType }; setFilters(reset); setApplied(reset); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div></div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('访问列表', 'Access')}</h2><Button variant="primary" onClick={openCreate}><Plus size={14} />{tr('新建访问', 'Create access')}</Button></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无访问', 'No access')} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
    <Modal open={createOpen} title={tr('新建访问', 'Create access')} onClose={() => setCreateOpen(false)} width={520} footer={<><Button onClick={() => setCreateOpen(false)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="create-proxy" disabled={saving}>{tr('确定', 'Create')}</Button></>}>{accessForm('create-proxy', create)}</Modal>
    <Modal open={!!editRow} title={tr('编辑访问', 'Edit access')} onClose={() => setEditRow(undefined)} width={480} footer={<><Button onClick={() => setEditRow(undefined)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-proxy" disabled={saving}>{tr('确定', 'Save')}</Button></>}>{accessForm('edit-proxy', update, true)}</Modal>
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
