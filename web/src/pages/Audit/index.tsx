import { Button, Column, DataTable, DateRangeField, Notice, Pager, StatusPill, Timestamp } from '@/components/ui';
import { accessTypeLabel } from '@/constants/accessTypes';
import { useI18n } from '@/i18n';
import { useDebouncedValue } from '@/hooks/useDebouncedValue';
import { useSearchParams } from '@/lib/runtime';
import { getAccessAuditList, getProxyList } from '@/services/api';
import { User } from 'lucide-react';
import { useCallback, useEffect, useMemo, useState } from 'react';
import './index.less';

const pageSize = 10;
const auditProtocols = ['ssh', 'webssh', 'rdp', 'vnc', 'mysql', 'postgresql', 'redis', 'mongodb'] as const;
type AuditProtocol = typeof auditProtocols[number];
const routeProtocolAliases: Record<string, AuditProtocol> = {
  webssh: 'webssh',
  webrdp: 'rdp',
  webvnc: 'vnc',
  webmysql: 'mysql',
  webpostgresql: 'postgresql',
  webredis: 'redis',
  webmongodb: 'mongodb',
};
const emptyFilters = { start: '', end: '', keyword: '', proxy_id: '', protocol: 'ssh' as AuditProtocol, action: '', success: '' };

const AuditPage: React.FC = () => {
  const { tr } = useI18n();
  const [routeSearch] = useSearchParams();
  const rawRouteProtocol = (routeSearch.get('protocol') || '').toLowerCase().replace(/[\s_-]/g, '');
  const routeProtocol = routeProtocolAliases[rawRouteProtocol] || rawRouteProtocol;
  const initialProtocol = auditProtocols.includes(routeProtocol as AuditProtocol) ? routeProtocol as AuditProtocol : 'ssh';
  const [rows, setRows] = useState<API.WebDataAuditItem[]>([]);
  const [proxies, setProxies] = useState<API.Proxy[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(false);
  const [filters, setFilters] = useState({ ...emptyFilters, proxy_id: routeSearch.get('proxy_id') || '', protocol: initialProtocol });
  const debouncedKeyword = useDebouncedValue(filters.keyword);
  const applied = useMemo(() => ({ start: filters.start, end: filters.end, keyword: debouncedKeyword, proxy_id: filters.proxy_id, protocol: filters.protocol, action: filters.action, success: filters.success }), [debouncedKeyword, filters.action, filters.end, filters.protocol, filters.proxy_id, filters.start, filters.success]);
  const [notice, setNotice] = useState<string>();

  useEffect(() => { void getProxyList({ page_size: 1000 }).then((response) => { if (response.code === 200) setProxies(response.data?.proxies || []); }).catch(() => setProxies([])); }, []);
  const load = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getAccessAuditList({ page, page_size: pageSize, proxy_id: applied.proxy_id ? Number(applied.proxy_id) : undefined, protocol: applied.protocol, action: applied.action || undefined, success: applied.success ? applied.success === 'true' : undefined, keyword: applied.keyword || undefined, start_time: applied.start ? new Date(applied.start).toISOString() : undefined, end_time: applied.end ? new Date(applied.end).toISOString() : undefined });
      if (response.code !== 200) throw new Error(response.message);
      setRows(response.data?.items || []); setTotal(response.data?.total || 0); setNotice(undefined);
    } catch (error: any) { setNotice(error?.message || tr('加载审计记录失败', 'Failed to load audit records')); }
    finally { setLoading(false); }
  }, [applied, page, tr]);
  useEffect(() => { void load(); }, [load]);

  const detail = (row: API.WebDataAuditItem, key: string) => row.details?.[key];
  const isSSHProtocol = applied.protocol === 'ssh' || applied.protocol === 'webssh';
  const isDesktopProtocol = applied.protocol === 'rdp' || applied.protocol === 'vnc';
  const protocolLabel = (protocol: AuditProtocol) => {
    if (protocol === 'rdp' || protocol === 'vnc' || protocol === 'mysql' || protocol === 'postgresql' || protocol === 'redis' || protocol === 'mongodb') return `Web ${accessTypeLabel(protocol)}`;
    return accessTypeLabel(protocol);
  };
  const sshAuthLabel = (row: API.WebDataAuditItem) => {
    const method = String(detail(row, 'auth_method') || '');
    if (method === 'agent') return 'Agent';
    if (method === 'password') return tr('密码', 'Password');
    return applied.protocol === 'webssh' ? 'Web' : '';
  };
  const actionLabel = (action: string) => {
    if (action === 'execute') {
      if (isSSHProtocol) return tr('命令执行', 'Command');
      if (applied.protocol === 'redis') return tr('Redis 命令', 'Redis command');
      if (applied.protocol === 'mongodb') return tr('MongoDB 操作', 'MongoDB operation');
      return tr('SQL 执行', 'SQL statement');
    }
    return ({ open_session: tr('打开会话', 'Open session'), close_session: tr('关闭会话', 'Close session'), test_connection: tr('连接测试', 'Connection test'), save_credential: tr('保存连接', 'Save connection'), delete_credential: tr('删除连接', 'Delete connection') }[action] || action || '-');
  };
  const referenceCell = (value: string | undefined, id: number) => <span className="audit-native-reference" title={value || `#${id}`}>{value || `#${id}`}</span>;
  const statusCell = (row: API.WebDataAuditItem) => {
    if (!row.success) return <span className="audit-status-hint" title={row.error || tr('上游连接或请求失败', 'Upstream connection or request failed')}><StatusPill tone="danger">{tr('失败', 'Failed')}</StatusPill></span>;
    if (isSSHProtocol && row.action === 'execute') return <StatusPill tone="neutral">{tr('已发送', 'Sent')}</StatusPill>;
    if (row.action === 'open_session') return <StatusPill tone="success">{tr('已连接', 'Connected')}</StatusPill>;
    if (row.action === 'close_session') return <StatusPill tone="neutral">{tr('已结束', 'Closed')}</StatusPill>;
    return <StatusPill tone="success">{tr('成功', 'Success')}</StatusPill>;
  };
  const columns = useMemo<Column<API.WebDataAuditItem>[]>(() => {
    if (isSSHProtocol) return [
      { key: 'time', title: tr('时间', 'Time'), width: 156, render: (row) => <Timestamp value={row.created_at} /> },
      { key: 'identity', title: tr('SSH 用户', 'SSH user'), width: 132, render: (row) => <span className="audit-ssh-user" title={`${String(detail(row, 'ssh_user') || '-')}${sshAuthLabel(row) ? `（${sshAuthLabel(row)}）` : ''}`}>{String(detail(row, 'ssh_user') || '-')}{sshAuthLabel(row) ? `（${sshAuthLabel(row)}）` : ''}</span> },
      { key: 'action', title: tr('事件', 'Event'), width: 90, render: (row) => actionLabel(row.action) },
      { key: 'statement', title: tr('命令', 'Command'), width: 220, render: (row) => <code className="audit-native-statement" title={row.statement_preview || ''}>{row.action === 'execute' ? row.statement_preview || '-' : '—'}</code> },
      { key: 'access', title: tr('访问', 'Access'), width: 112, render: (row) => referenceCell(row.proxy_name, row.proxy_id) },
      { key: 'application', title: tr('应用', 'Application'), width: 120, render: (row) => referenceCell(row.application_name, row.application_id) },
      { key: 'client', title: tr('来源 IP', 'Source IP'), width: 116, render: (row) => <code>{String(detail(row, 'client_ip') || '-')}</code> },
      { key: 'status', title: tr('状态', 'Status'), width: 78, render: statusCell },
    ];
    if (isDesktopProtocol) return [
      { key: 'time', title: tr('时间', 'Time'), width: 156, render: (row) => <Timestamp value={row.created_at} /> },
      { key: 'user', title: tr('用户', 'User'), width: 150, render: (row) => <span className="liaison-inline-name"><User size={13} />{row.user_email || `#${row.user_id}`}</span> },
      { key: 'identity', title: applied.protocol === 'rdp' ? tr('远程用户', 'Remote user') : tr('连接身份', 'Identity'), width: 140, render: (row) => String(detail(row, 'remote_user') || '-') },
      { key: 'action', title: tr('事件', 'Event'), width: 112, render: (row) => actionLabel(row.action) },
      { key: 'access', title: tr('访问', 'Access'), width: 140, render: (row) => referenceCell(row.proxy_name, row.proxy_id) },
      { key: 'application', title: tr('应用', 'Application'), width: 150, render: (row) => referenceCell(row.application_name, row.application_id) },
      { key: 'client', title: tr('来源 IP', 'Source IP'), width: 126, render: (row) => <code>{String(detail(row, 'client_ip') || '-')}</code> },
      { key: 'elapsed', title: tr('会话时长', 'Duration'), width: 100, render: (row) => row.action === 'close_session' ? `${Math.max(0, row.elapsed_ms)} ms` : '—' },
      { key: 'status', title: tr('状态', 'Status'), width: 86, render: statusCell },
    ];
    const databaseLabel = applied.protocol === 'redis' ? tr('Redis DB', 'Redis DB') : applied.protocol === 'mongodb' ? tr('数据库', 'Database') : tr('数据库 / Schema', 'Database / schema');
    const statementLabel = applied.protocol === 'redis' ? tr('命令或事件', 'Command or event') : applied.protocol === 'mongodb' ? tr('操作或事件', 'Operation or event') : tr('SQL 或事件', 'SQL or event');
    return [
      { key: 'time', title: tr('时间', 'Time'), width: 156, render: (row) => <Timestamp value={row.created_at} /> },
      { key: 'user', title: tr('用户', 'User'), width: 150, render: (row) => <span className="liaison-inline-name"><User size={13} />{row.user_email || `#${row.user_id}`}</span> },
      { key: 'action', title: tr('事件', 'Event'), width: 112, render: (row) => actionLabel(row.action) },
      { key: 'statement', title: statementLabel, width: 240, render: (row) => <code className="audit-native-statement" title={row.statement_preview || ''}>{row.statement_preview || '-'}</code> },
      { key: 'database', title: databaseLabel, width: 130, render: (row) => String(detail(row, 'database') || '-') },
      { key: 'access', title: tr('访问', 'Access'), width: 112, render: (row) => referenceCell(row.proxy_name, row.proxy_id) },
      { key: 'application', title: tr('应用', 'Application'), width: 120, render: (row) => referenceCell(row.application_name, row.application_id) },
      { key: 'status', title: tr('状态', 'Status'), width: 86, render: statusCell },
    ];
  }, [applied.protocol, isDesktopProtocol, isSSHProtocol, tr]);

  const selectProtocol = (protocol: AuditProtocol) => {
    setFilters((value) => ({ ...value, protocol, action: '' }));
    setPage(1);
  };
  const actionOptions = isSSHProtocol ? ['execute', 'open_session', 'close_session', 'save_credential', 'delete_credential'] : isDesktopProtocol ? ['open_session', 'close_session', 'save_credential', 'delete_credential'] : ['execute', 'open_session', 'close_session', 'test_connection', 'save_credential', 'delete_credential'];

  return <div className="liaison-page-stack audit-native-page">
    {notice ? <Notice tone="danger">{notice}</Notice> : null}
    <nav className="audit-protocol-tabs" aria-label={tr('按协议查看审计日志', 'Audit logs by protocol')}>
      {auditProtocols.map((protocol) => <button key={protocol} type="button" role="tab" aria-selected={applied.protocol === protocol} className={applied.protocol === protocol ? 'is-active' : ''} onClick={() => selectProtocol(protocol)}>{protocolLabel(protocol)}</button>)}
    </nav>
    <div className="liaison-filter-bar audit-native-filter">
      <DateRangeField className="audit-native-time" label={tr('操作时间', 'Time')} start={filters.start} end={filters.end} startPlaceholder={tr('开始日期', 'Start date')} endPlaceholder={tr('结束日期', 'End date')} onStartChange={(start) => { setFilters((value) => ({ ...value, start })); setPage(1); }} onEndChange={(end) => { setFilters((value) => ({ ...value, end })); setPage(1); }} />
      <label className="liaison-compound"><span>{tr('关键词', 'Keyword')}</span><input value={filters.keyword} onChange={(event) => { setFilters((value) => ({ ...value, keyword: event.target.value })); setPage(1); }} placeholder={isSSHProtocol ? tr('命令或哈希', 'Command or hash') : tr('语句或哈希', 'Statement or hash')} /></label>
      <label className="liaison-compound"><span>{tr('访问', 'Access')}</span><select value={filters.proxy_id} onChange={(event) => { setFilters((value) => ({ ...value, proxy_id: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option>{proxies.map((proxy) => <option key={proxy.id} value={proxy.id}>{proxy.name}</option>)}</select></label>
      <label className="liaison-compound"><span>{tr('操作', 'Operation')}</span><select value={filters.action} onChange={(event) => { setFilters((value) => ({ ...value, action: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option>{actionOptions.map((action) => <option key={action} value={action}>{actionLabel(action)}</option>)}</select></label>
      <label className="liaison-compound"><span>{tr('结果', 'Result')}</span><select value={filters.success} onChange={(event) => { setFilters((value) => ({ ...value, success: event.target.value })); setPage(1); }}><option value="">{tr('全部', 'All')}</option><option value="true">{tr('成功', 'Success')}</option><option value="false">{tr('失败', 'Failed')}</option></select></label>
      <div className="liaison-filter-actions"><Button onClick={() => { setFilters({ ...emptyFilters, protocol: applied.protocol }); setPage(1); }}>{tr('重置', 'Reset')}</Button></div>
    </div>
    <section className="liaison-list-panel"><header className="liaison-list-header"><h2>{protocolLabel(applied.protocol)} · {tr('访问操作', 'Access activity')}</h2>{isSSHProtocol ? <span className="audit-status-note">{tr('命令状态表示已发送至终端；连接失败会单独标记。', 'Command status means sent to the terminal; connection failures are marked separately.')}</span> : null}</header><DataTable columns={columns} rows={rows} rowKey={(row) => row.id} loading={loading} emptyText={tr(`暂无 ${protocolLabel(applied.protocol)} 审计记录`, `No ${protocolLabel(applied.protocol)} audit records`)} /><Pager page={page} pageSize={pageSize} total={total} onPageChange={setPage} /></section>
  </div>;
};

export default AuditPage;
