import { Button, Column, DataTable, Drawer, Field, Input, Modal, Notice, Pager, StatusPill } from '@/components/ui';
import { useI18n } from '@/i18n';
import { getDeviceDetail, getDeviceList, updateDevice } from '@/services/api';
import { formatMBSize } from '@/utils/format';
import { Monitor } from 'lucide-react';
import { FormEvent, useCallback, useEffect, useState } from 'react';

const pageSize = 10;

const DevicePage: React.FC = () => {
  const { tr } = useI18n();
  const [rows, setRows] = useState<API.Device[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ name: '', ip: '' });
  const [applied, setApplied] = useState(filters);
  const [current, setCurrent] = useState<API.Device>();
  const [detailOpen, setDetailOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editName, setEditName] = useState('');
  const [editDescription, setEditDescription] = useState('');
  const [saving, setSaving] = useState(false);
  const [notice, setNotice] = useState<{ tone: 'danger' | 'success'; text: string }>();

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getDeviceList({ page, page_size: pageSize, name: applied.name || undefined, ip: applied.ip || undefined });
      if (response.code !== 200) throw new Error(response.message);
      setRows(response.data?.devices || []);
      setTotal(response.data?.total || 0);
    } catch (error: any) {
      setNotice({ tone: 'danger', text: error?.message || tr('加载设备失败', 'Failed to load devices') });
    } finally { setLoading(false); }
  }, [applied, page, tr]);

  useEffect(() => { void load(); }, [load]);

  const openDetail = async (row: API.Device) => {
    setCurrent(row); setDetailOpen(true);
    try { const response = await getDeviceDetail(row.id); if (response.code === 200 && response.data) setCurrent(response.data); } catch { /* keep list snapshot */ }
  };
  const openEdit = (row: API.Device) => { setCurrent(row); setEditName(row.name); setEditDescription(row.description || ''); setEditOpen(true); };
  const save = async (event: FormEvent) => {
    event.preventDefault();
    if (!current || !editName.trim()) return;
    setSaving(true);
    try {
      const response = await updateDevice(current.id, { name: editName.trim(), description: editDescription });
      if (response.code !== 200) throw new Error(response.message);
      setEditOpen(false); setNotice({ tone: 'success', text: tr('更新成功', 'Updated successfully') }); await load();
    } catch (error: any) { setNotice({ tone: 'danger', text: error?.message || tr('更新失败', 'Update failed') }); }
    finally { setSaving(false); }
  };

  const columns: Column<API.Device>[] = [
    { key: 'name', title: tr('设备名称', 'Device'), width: 180, render: (row) => <span className="liaison-inline-name"><Monitor size={14} />{row.name || `${tr('设备', 'Device')}-${row.id}`}</span> },
    { key: 'online', title: tr('状态', 'Status'), width: 90, render: (row) => <StatusPill tone={row.online === 1 ? 'success' : 'neutral'}>{row.online === 1 ? tr('在线', 'Online') : tr('离线', 'Offline')}</StatusPill> },
    { key: 'os', title: tr('操作系统', 'OS'), width: 110, render: (row) => row.os || '-' },
    { key: 'version', title: tr('版本', 'Version'), width: 130, render: (row) => row.version || '-' },
    { key: 'cpu', title: 'CPU', width: 75, render: (row) => `${row.cpu} ${tr('核', 'cores')}` },
    { key: 'memory', title: tr('内存', 'Memory'), width: 90, render: (row) => formatMBSize(row.memory) },
    { key: 'interfaces', title: tr('网卡', 'Interfaces'), width: 220, render: (row) => row.interfaces?.map((item) => `${item.name}: ${(item.ip || []).filter((ip) => !ip.includes(':')).join(', ') || '-'}`).join(' · ') || '-' },
    { key: 'updated', title: tr('更新时间', 'Updated'), width: 150, render: (row) => row.updated_at || '-' },
    { key: 'description', title: tr('描述', 'Description'), width: 180, render: (row) => row.description || '-' },
    { key: 'actions', title: tr('操作', 'Actions'), width: 100, render: (row) => <span className="liaison-table-actions"><button className="liaison-table-link" onClick={() => void openDetail(row)}>{tr('详情', 'Detail')}</button><button className="liaison-table-link" onClick={() => openEdit(row)}>{tr('编辑', 'Edit')}</button></span> },
  ];

  const detailItems = current ? [
    [tr('设备 ID', 'Device ID'), current.id], [tr('设备名称', 'Device Name'), current.name], [tr('状态', 'Status'), current.online === 1 ? tr('在线', 'Online') : tr('离线', 'Offline')], [tr('操作系统', 'OS'), current.os], [tr('版本', 'Version'), current.version], ['CPU', `${current.cpu} ${tr('核', 'cores')}`], [tr('内存', 'Memory'), formatMBSize(current.memory)], [tr('磁盘', 'Disk'), formatMBSize(current.disk)], [tr('描述', 'Description'), current.description || '-'], [tr('创建时间', 'Created'), current.created_at], [tr('更新时间', 'Updated'), current.updated_at],
  ] : [];

  return <div className="liaison-page-stack">
    {notice ? <Notice tone={notice.tone}>{notice.text}</Notice> : null}
    <div className="liaison-filter-bar">
      <label className="liaison-compound"><span>{tr('设备名称', 'Device')}</span><input value={filters.name} onChange={(event) => setFilters((value) => ({ ...value, name: event.target.value }))} placeholder={tr('输入设备名称', 'Device name')} /></label>
      <label className="liaison-compound"><span>{tr('网卡 IP', 'NIC IP')}</span><input value={filters.ip} onChange={(event) => setFilters((value) => ({ ...value, ip: event.target.value }))} placeholder={tr('输入 IP', 'IP address')} /></label>
      <div className="liaison-filter-actions"><Button onClick={() => { setFilters({ name: '', ip: '' }); setApplied({ name: '', ip: '' }); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div>
    </div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('设备列表', 'Devices')}</h2></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无设备', 'No devices')} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
    <Drawer open={detailOpen} title={tr('设备详情', 'Device Detail')} onClose={() => setDetailOpen(false)}><dl className="liaison-detail-list">{detailItems.map(([label, value]) => <div key={String(label)}><dt>{label}</dt><dd>{String(value ?? '-')}</dd></div>)}</dl>{current?.interfaces?.length ? <section className="liaison-detail-section"><h3>{tr('网卡信息', 'Network interfaces')}</h3>{current.interfaces.map((item) => <div key={item.name}><b>{item.name}</b><span>{(item.ip || []).join(', ') || '-'}</span><small>MAC: {item.mac || '-'}</small></div>)}</section> : null}</Drawer>
    <Modal open={editOpen} title={tr('编辑设备', 'Edit Device')} onClose={() => setEditOpen(false)} width={500} footer={<><Button onClick={() => setEditOpen(false)}>{tr('取消', 'Cancel')}</Button><Button variant="primary" type="submit" form="edit-device-form" disabled={saving}>{tr('确定', 'Save')}</Button></>}><form id="edit-device-form" className="native-modal-form" onSubmit={save}><Field label={tr('设备名称', 'Device Name')} required><Input value={editName} onChange={(event) => setEditName(event.target.value)} /></Field><Field label={tr('描述', 'Description')}><Input value={editDescription} onChange={(event) => setEditDescription(event.target.value)} /></Field></form></Modal>
  </div>;
};

export default DevicePage;
