import { Button, Column, DataTable, DateRangeField, Notice, Pager, StatusPill, Timestamp } from '@/components/ui';
import { useI18n } from '@/i18n';
import { getManagementAuditList } from '@/services/api';
import { User } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import './index.less';

const pageSize = 10;
const emptyFilters = { start: '', end: '', keyword: '', module: '', success: '' };

const ManagementLogPage: React.FC = () => {
  const { tr } = useI18n();
  const [rows, setRows] = useState<API.ManagementAuditItem[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<string>();
  const [filters, setFilters] = useState(emptyFilters);
  const [applied, setApplied] = useState(emptyFilters);

  const moduleLabel = (value: string) => ({ account: tr('账户', 'Account'), connector: tr('连接器', 'Connector'), device: tr('设备', 'Device'), application: tr('应用', 'Application'), access: tr('访问', 'Access'), firewall: tr('防火墙', 'Firewall'), token: tr('访问令牌', 'Token') }[value] || value || '-');
  const actionLabel = (value: string) => ({ login: tr('登录', 'Sign in'), logout: tr('退出登录', 'Sign out'), create: tr('新建', 'Create'), update: tr('更新', 'Update'), delete: tr('删除', 'Delete'), change_password: tr('修改密码', 'Change password'), scan_applications: tr('扫描应用', 'Scan applications'), restore_default: tr('恢复默认', 'Restore default') }[value] || value || '-');

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getManagementAuditList({ page, page_size: pageSize, module: applied.module || undefined, success: applied.success ? applied.success === 'true' : undefined, keyword: applied.keyword || undefined, start_time: applied.start ? new Date(applied.start).toISOString() : undefined, end_time: applied.end ? new Date(applied.end).toISOString() : undefined });
      if (response.code !== 200) throw new Error(response.message);
      setRows(response.data?.items || []);
      setTotal(response.data?.total || 0);
      setNotice(undefined);
    } catch (error: any) {
      setNotice(error?.message || tr('加载管理日志失败', 'Failed to load management logs'));
    } finally {
      setLoading(false);
    }
  }, [applied, page, tr]);
  useEffect(() => { void load(); }, [load]);

  const columns: Column<API.ManagementAuditItem>[] = [
    { key: 'time', title: tr('时间', 'Time'), width: 156, render: (row) => <Timestamp value={row.created_at} /> },
    { key: 'user', title: tr('用户', 'User'), width: 190, render: (row) => <span className="liaison-inline-name"><User size={13} />{row.user_email || `#${row.user_id}`}</span> },
    { key: 'module', title: tr('模块', 'Module'), width: 105, render: (row) => <StatusPill>{moduleLabel(row.module)}</StatusPill> },
    { key: 'action', title: tr('操作', 'Operation'), width: 120, render: (row) => <StatusPill tone="info">{actionLabel(row.action)}</StatusPill> },
    { key: 'resource', title: tr('目标', 'Target'), width: 250, render: (row) => <code className="management-log-resource">{row.resource.replace('/api/v1/', '')}</code> },
    { key: 'ip', title: tr('来源 IP', 'Source IP'), width: 135, render: (row) => <code>{row.client_ip || '-'}</code> },
    { key: 'elapsed', title: tr('耗时', 'Elapsed'), width: 85, render: (row) => `${row.elapsed_ms} ms` },
    { key: 'result', title: tr('结果', 'Result'), width: 85, render: (row) => <StatusPill tone={row.success ? 'success' : 'danger'}>{row.success ? tr('成功', 'Success') : tr('失败', 'Failed')}</StatusPill> },
  ];

  return <div className="liaison-page-stack management-log-page">
    {notice ? <Notice tone="danger">{notice}</Notice> : null}
    <div className="liaison-filter-bar management-log-filter">
      <DateRangeField className="management-log-time" label={tr('操作时间', 'Time')} start={filters.start} end={filters.end} startPlaceholder={tr('开始日期', 'Start date')} endPlaceholder={tr('结束日期', 'End date')} onStartChange={(start) => setFilters((value) => ({ ...value, start }))} onEndChange={(end) => setFilters((value) => ({ ...value, end }))} />
      <label className="liaison-compound"><span>{tr('关键词', 'Keyword')}</span><input value={filters.keyword} onChange={(event) => setFilters((value) => ({ ...value, keyword: event.target.value }))} placeholder={tr('用户、目标或 IP', 'User, target or IP')} /></label>
      <label className="liaison-compound"><span>{tr('模块', 'Module')}</span><select value={filters.module} onChange={(event) => setFilters((value) => ({ ...value, module: event.target.value }))}><option value="">{tr('全部', 'All')}</option>{['account', 'connector', 'device', 'application', 'access', 'firewall', 'token'].map((value) => <option key={value} value={value}>{moduleLabel(value)}</option>)}</select></label>
      <label className="liaison-compound"><span>{tr('结果', 'Result')}</span><select value={filters.success} onChange={(event) => setFilters((value) => ({ ...value, success: event.target.value }))}><option value="">{tr('全部', 'All')}</option><option value="true">{tr('成功', 'Success')}</option><option value="false">{tr('失败', 'Failed')}</option></select></label>
      <div className="liaison-filter-actions"><Button onClick={() => { setFilters(emptyFilters); setApplied(emptyFilters); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div>
    </div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('用户操作', 'User operations')}</h2></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无管理日志', 'No management logs')} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
  </div>;
};

export default ManagementLogPage;
