import { Button, Column, DataTable, Notice, Pager, StatusPill } from '@/components/ui';
import { useI18n } from '@/i18n';
import { useSearchParams } from '@/lib/runtime';
import { getAccessAuditList, getProxyList } from '@/services/api';
import { ChevronDown, ChevronRight, User } from 'lucide-react';
import { useCallback, useEffect, useState } from 'react';
import './index.less';

const pageSize = 10;
const emptyFilters = { start: '', end: '', keyword: '', proxy_id: '', protocol: '', action: '', success: '' };

const AuditPage: React.FC = () => {
  const { tr } = useI18n();
  const [routeSearch] = useSearchParams();
  const [rows, setRows] = useState<API.WebDataAuditItem[]>([]);
  const [proxies, setProxies] = useState<API.Proxy[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ ...emptyFilters, proxy_id: routeSearch.get('proxy_id') || '' });
  const [applied, setApplied] = useState(filters);
  const [expanded, setExpanded] = useState<Set<number>>(new Set());
  const [notice, setNotice] = useState<string>();

  useEffect(() => { void getProxyList({ page_size: 1000 }).then((response) => { if (response.code === 200) setProxies(response.data?.proxies || []); }).catch(() => setProxies([])); }, []);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getAccessAuditList({ page, page_size: pageSize, proxy_id: applied.proxy_id ? Number(applied.proxy_id) : undefined, protocol: applied.protocol || undefined, action: applied.action || undefined, success: applied.success ? applied.success === 'true' : undefined, keyword: applied.keyword || undefined, start_time: applied.start ? new Date(applied.start).toISOString() : undefined, end_time: applied.end ? new Date(applied.end).toISOString() : undefined });
      if (response.code !== 200) throw new Error(response.message);
      setRows(response.data?.items || []); setTotal(response.data?.total || 0);
    } catch (error: any) { setNotice(error?.message || tr('加载审计记录失败', 'Failed to load audit records')); }
    finally { setLoading(false); }
  }, [applied, page, tr]);
  useEffect(() => { void load(); }, [load]);

  const columns: Column<API.WebDataAuditItem>[] = [
    { key: 'expand', title: '', width: 40, render: (row) => <button className="audit-native-expand" onClick={() => setExpanded((current) => { const next = new Set(current); if (next.has(row.id)) next.delete(row.id); else next.add(row.id); return next; })}>{expanded.has(row.id) ? <ChevronDown size={14} /> : <ChevronRight size={14} />}</button> },
    { key: 'type', title: tr('类型', 'Type'), width: 110, render: (row) => <StatusPill tone="info">{row.action || '-'}</StatusPill> },
    { key: 'time', title: tr('时间', 'Time'), width: 160, render: (row) => row.created_at },
    { key: 'user', title: tr('用户', 'User'), width: 180, render: (row) => <span className="liaison-inline-name"><User size={13} />{row.user_email || `#${row.user_id}`}</span> },
    { key: 'access', title: tr('访问类型', 'Access type'), width: 100, render: (row) => <StatusPill>{row.protocol?.toUpperCase() || '-'}</StatusPill> },
    { key: 'object', title: tr('数据对象', 'Object'), width: 210, render: (row) => <span className="audit-native-object"><strong>{row.proxy_name || `#${row.proxy_id}`}</strong><small>{row.application_name || `#${row.application_id}`}</small></span> },
    { key: 'status', title: tr('状态', 'Status'), width: 85, render: (row) => <StatusPill tone={row.success ? 'success' : 'danger'}>{row.success ? 'OK' : tr('失败', 'Failed')}</StatusPill> },
  ];

  return <div className="liaison-page-stack audit-native-page">
    {notice ? <Notice tone="danger">{notice}</Notice> : null}
    <div className="liaison-filter-bar audit-native-filter">
      <label className="liaison-compound audit-native-time"><span>{tr('开始', 'From')}</span><input type="datetime-local" value={filters.start} onChange={(event) => setFilters((value) => ({ ...value, start: event.target.value }))} /></label>
      <label className="liaison-compound audit-native-time"><span>{tr('结束', 'To')}</span><input type="datetime-local" value={filters.end} onChange={(event) => setFilters((value) => ({ ...value, end: event.target.value }))} /></label>
      <label className="liaison-compound"><span>{tr('关键词', 'Keyword')}</span><input value={filters.keyword} onChange={(event) => setFilters((value) => ({ ...value, keyword: event.target.value }))} placeholder={tr('命令或哈希', 'Command or hash')} /></label>
      <label className="liaison-compound"><span>{tr('访问', 'Access')}</span><select value={filters.proxy_id} onChange={(event) => setFilters((value) => ({ ...value, proxy_id: event.target.value }))}><option value="">{tr('全部', 'All')}</option>{proxies.map((proxy) => <option key={proxy.id} value={proxy.id}>{proxy.name}</option>)}</select></label>
      <label className="liaison-compound"><span>{tr('协议', 'Protocol')}</span><select value={filters.protocol} onChange={(event) => setFilters((value) => ({ ...value, protocol: event.target.value }))}><option value="">{tr('全部', 'All')}</option>{['ssh', 'mysql', 'postgresql', 'redis', 'mongodb'].map((value) => <option key={value} value={value}>{value.toUpperCase()}</option>)}</select></label>
      <label className="liaison-compound"><span>{tr('类型', 'Type')}</span><select value={filters.action} onChange={(event) => setFilters((value) => ({ ...value, action: event.target.value }))}><option value="">{tr('全部', 'All')}</option><option value="execute">{tr('命令执行', 'Execute')}</option><option value="open_session">{tr('打开会话', 'Open session')}</option><option value="close_session">{tr('关闭会话', 'Close session')}</option><option value="test_connection">{tr('连接测试', 'Connection test')}</option></select></label>
      <label className="liaison-compound"><span>{tr('结果', 'Result')}</span><select value={filters.success} onChange={(event) => setFilters((value) => ({ ...value, success: event.target.value }))}><option value="">{tr('全部', 'All')}</option><option value="true">{tr('成功', 'Success')}</option><option value="false">{tr('失败', 'Failed')}</option></select></label>
      <div className="liaison-filter-actions"><Button onClick={() => { setFilters(emptyFilters); setApplied(emptyFilters); setPage(1); }}>{tr('重置', 'Reset')}</Button><Button variant="primary" onClick={() => { setApplied(filters); setPage(1); }}>{tr('查询', 'Search')}</Button></div>
    </div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{tr('记录', 'Records')}</h2></header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr('暂无审计记录', 'No audit records')} />{rows.filter((row) => expanded.has(row.id)).map((row) => <div className="audit-native-detail" key={`detail-${row.id}`}><div><span>{tr('语句预览', 'Statement')}</span><code>{row.statement_preview || '-'}</code></div><div><span>SHA-256</span><code>{row.statement_sha256 || '-'}</code></div><div><span>{tr('耗时', 'Elapsed')}</span><strong>{row.elapsed_ms} ms</strong></div>{row.error ? <div><span>{tr('错误', 'Error')}</span><strong>{row.error}</strong></div> : null}{row.details && Object.keys(row.details).length ? <div className="is-full"><span>{tr('详情', 'Details')}</span><pre>{JSON.stringify(row.details, null, 2)}</pre></div> : null}</div>)}<Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
  </div>;
};

export default AuditPage;
