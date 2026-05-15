import { useI18n } from '@/i18n';
import { getAccessAuditList, getProxyList } from '@/services/api';
import { defaultPagination, defaultSearch } from '@/utils/tableConfig';
import {
  AuditOutlined,
  CheckCircleOutlined,
  CloseCircleOutlined,
  CloudServerOutlined,
  DatabaseOutlined,
  FileSearchOutlined,
  LinkOutlined,
  ReloadOutlined,
  UserOutlined,
} from '@ant-design/icons';
import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
} from '@ant-design/pro-components';
import { history, useSearchParams } from '@umijs/max';
import {
  Button,
  Empty,
  Space,
  Tabs,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useEffect, useMemo, useRef, useState } from 'react';
import './index.less';

const { Text } = Typography;

const protocolLabels: Record<string, string> = {
  ssh: 'SSH',
  mysql: 'MySQL',
  postgresql: 'PostgreSQL',
  redis: 'Redis',
  mongodb: 'MongoDB',
};

const protocolColors: Record<string, string> = {
  ssh: 'geekblue',
  mysql: 'volcano',
  postgresql: 'processing',
  redis: 'red',
  mongodb: 'success',
};

const actionColors: Record<string, string> = {
  execute: 'blue',
  test_connection: 'cyan',
  open_session: 'geekblue',
  close_session: 'default',
  save_credential: 'green',
  delete_credential: 'red',
};

const AuditPage: React.FC = () => {
  const { tr } = useI18n();
  const actionRef = useRef<ActionType>();
  const searchFormRef = useRef<any>();
  const [searchParams] = useSearchParams();
  const [proxyOptions, setProxyOptions] = useState<
    { label: string; value: number }[]
  >([]);
  const [summary, setSummary] = useState({
    total: 0,
    success: 0,
    failed: 0,
  });

  const initialProxyID = useMemo(() => {
    const raw = searchParams.get('proxy_id');
    if (!raw) return undefined;
    const value = Number(raw);
    return Number.isFinite(value) && value > 0 ? value : undefined;
  }, [searchParams]);

  useEffect(() => {
    let mounted = true;
    const loadProxyOptions = async () => {
      try {
        const res = await getProxyList({ page_size: 10000 });
        if (!mounted) return;
        if (res.code === 200 && res.data?.proxies) {
          setProxyOptions(
            res.data.proxies
              .map((proxy) => {
                const protocol = String(
                  proxy.application?.application_type || '',
                ).toLowerCase();
                const protocolLabel =
                  protocolLabels[protocol] || protocol.toUpperCase() || '-';
                return {
                  value: proxy.id,
                  label: [proxy.name, protocolLabel, proxy.application?.name]
                    .filter(Boolean)
                    .join(' · '),
                };
              })
              .sort((left, right) => left.label.localeCompare(right.label)),
          );
          return;
        }
        message.error(
          res.message || tr('加载访问列表失败', 'Failed to load entries'),
        );
      } catch (err: any) {
        if (mounted) {
          message.error(
            err?.message || tr('加载访问列表失败', 'Failed to load entries'),
          );
        }
      }
    };
    loadProxyOptions();
    return () => {
      mounted = false;
    };
  }, [tr]);

  const actionLabels = useMemo(
    () => ({
      execute: tr('命令执行', 'Command'),
      test_connection: tr('测试连接', 'Test'),
      open_session: tr('进入会话', 'Session'),
      close_session: tr('关闭会话', 'Close'),
      save_credential: tr('保存连接', 'Save'),
      delete_credential: tr('删除连接', 'Delete'),
    }),
    [tr],
  );

  const submitSearchAfterChange = () => {
    window.setTimeout(() => searchFormRef.current?.submit?.(), 0);
  };

  const columns: ProColumns<API.WebDataAuditItem>[] = [
    {
      title: tr('时间', 'Time'),
      dataIndex: 'time_range',
      valueType: 'dateTimeRange',
      hideInTable: true,
    },
    {
      title: tr('关键字', 'Keyword'),
      dataIndex: 'keyword',
      hideInTable: true,
      fieldProps: {
        placeholder: tr(
          '命令 / Hash / 错误 / IP',
          'Command / Hash / Error / IP',
        ),
      },
    },
    {
      title: tr('访问', 'Entry'),
      dataIndex: 'proxy_id',
      hideInTable: true,
      initialValue: initialProxyID,
      valueType: 'select',
      fieldProps: {
        allowClear: true,
        showSearch: true,
        optionFilterProp: 'label',
        options: proxyOptions,
        placeholder: tr('选择访问名称', 'Select entry'),
        onChange: submitSearchAfterChange,
      },
    },
    {
      title: tr('协议', 'Protocol'),
      dataIndex: 'protocol',
      hideInTable: true,
      fieldProps: {
        allowClear: true,
        onChange: submitSearchAfterChange,
      },
      valueEnum: {
        ssh: { text: 'SSH' },
        mysql: { text: 'MySQL' },
        postgresql: { text: 'PostgreSQL' },
        redis: { text: 'Redis' },
        mongodb: { text: 'MongoDB' },
      },
    },
    {
      title: tr('类型', 'Action'),
      dataIndex: 'action',
      valueEnum: {
        execute: { text: actionLabels.execute },
        test_connection: { text: actionLabels.test_connection },
        open_session: { text: actionLabels.open_session },
        close_session: { text: actionLabels.close_session },
        save_credential: { text: actionLabels.save_credential },
        delete_credential: { text: actionLabels.delete_credential },
      },
      fieldProps: {
        allowClear: true,
        onChange: submitSearchAfterChange,
      },
      width: 122,
      render: (_, record) => (
        <Tag color={actionColors[record.action] || 'default'}>
          {actionLabels[record.action as keyof typeof actionLabels] ||
            record.action}
        </Tag>
      ),
    },
    {
      title: tr('时间', 'Time'),
      dataIndex: 'created_at',
      width: 176,
      search: false,
      render: (value) => formatAuditTime(String(value || '')),
    },
    {
      title: tr('用户', 'User'),
      dataIndex: 'user_email',
      search: false,
      width: 200,
      render: (_, record) => (
        <div className="audit-user">
          <UserOutlined />
          <span>{record.user_email || `User #${record.user_id}`}</span>
        </div>
      ),
    },
    {
      title: tr('访问类型', 'Access Type'),
      dataIndex: 'protocol',
      key: 'access_type',
      search: false,
      width: 112,
      render: (_, record) => (
        <Tag color={protocolColors[record.protocol] || 'default'}>
          {protocolLabels[record.protocol] || record.protocol || '-'}
        </Tag>
      ),
    },
    {
      title: tr('数据对象', 'Data Object'),
      dataIndex: 'proxy_name',
      search: false,
      width: 360,
      render: (_, record) => (
        <div className="audit-source">
          <button
            type="button"
            className="audit-source-link"
            onClick={() => history.push(accessDetailPath(record))}
          >
            <CloudServerOutlined />
            <span>{record.proxy_name || `#${record.proxy_id}`}</span>
          </button>
          <div className="audit-source-sub">
            {record.application_name || `APP #${record.application_id}`}
          </div>
        </div>
      ),
    },
    {
      title: tr('状态', 'Status'),
      dataIndex: 'success',
      valueEnum: {
        true: { text: tr('成功', 'Success') },
        false: { text: tr('失败', 'Failed') },
      },
      fieldProps: {
        allowClear: true,
        onChange: submitSearchAfterChange,
      },
      width: 98,
      render: (_, record) =>
        record.success ? (
          <Tag icon={<CheckCircleOutlined />} color="success">
            OK
          </Tag>
        ) : (
          <Tag icon={<CloseCircleOutlined />} color="error">
            ERR
          </Tag>
        ),
    },
    {
      title: tr('摘要', 'Summary'),
      dataIndex: 'statement_preview',
      search: false,
      ellipsis: true,
      width: 420,
      render: (value) => (
        <Tooltip title={value}>
          <span className="audit-command">{value}</span>
        </Tooltip>
      ),
    },
    {
      title: tr('耗时', 'Elapsed'),
      dataIndex: 'elapsed_ms',
      search: false,
      width: 88,
      render: (value) => `${value || 0} ms`,
    },
  ];

  return (
    <PageContainer title={false} className="audit-page">
      <div className="audit-header">
        <div className="audit-header-main">
          <div className="audit-header-icon">
            <AuditOutlined />
          </div>
          <div>
            <Text type="secondary">{tr('安全审计', 'Security Audit')}</Text>
            <h1>{tr('审计中心', 'Audit Center')}</h1>
          </div>
        </div>
      </div>

      <Tabs
        className="audit-tabs"
        defaultActiveKey="access"
        items={[
          {
            key: 'access',
            label: tr('访问审计', 'Access Audit'),
            children: (
              <div className="audit-tab-panel">
                <Space className="audit-stats" size={12} wrap>
                  <div className="audit-stat">
                    <FileSearchOutlined />
                    <span>{tr('访问记录', 'Access Records')}</span>
                    <strong>{summary.total}</strong>
                  </div>
                  <div className="audit-stat">
                    <CheckCircleOutlined />
                    <span>{tr('本页成功', 'Page OK')}</span>
                    <strong>{summary.success}</strong>
                  </div>
                  <div className="audit-stat is-danger">
                    <CloseCircleOutlined />
                    <span>{tr('本页失败', 'Page ERR')}</span>
                    <strong>{summary.failed}</strong>
                  </div>
                </Space>

                <ProTable<API.WebDataAuditItem>
                  actionRef={actionRef}
                  formRef={searchFormRef}
                  rowKey="id"
                  className="audit-table"
                  headerTitle={
                    <Space>
                      <DatabaseOutlined />
                      <span>{tr('访问操作', 'Access Operations')}</span>
                    </Space>
                  }
                  columns={columns}
                  search={{
                    ...defaultSearch,
                    span: 6,
                    defaultCollapsed: false,
                    searchText: tr('查询', 'Search'),
                    resetText: tr('重置', 'Reset'),
                  }}
                  pagination={defaultPagination}
                  options={{
                    density: true,
                    fullScreen: true,
                    reload: true,
                  }}
                  toolBarRender={() => [
                    <Button
                      key="refresh"
                      icon={<ReloadOutlined />}
                      onClick={() => actionRef.current?.reload()}
                    >
                      {tr('刷新', 'Refresh')}
                    </Button>,
                  ]}
                  request={async (params) => {
                    const query = buildAuditQuery(params);
                    try {
                      const res = await getAccessAuditList(query);
                      if (res.code !== 200 || !res.data) {
                        message.error(
                          res.message ||
                            tr('加载审计失败', 'Failed to load audits'),
                        );
                        setSummary({ total: 0, success: 0, failed: 0 });
                        return { data: [], total: 0, success: false };
                      }
                      const items = res.data.items || [];
                      setSummary({
                        total: res.data.total || 0,
                        success: items.filter((item) => item.success).length,
                        failed: items.filter((item) => !item.success).length,
                      });
                      return {
                        data: items,
                        total: res.data.total || 0,
                        success: true,
                      };
                    } catch (err: any) {
                      message.error(
                        err?.message ||
                          tr('加载审计失败', 'Failed to load audits'),
                      );
                      setSummary({ total: 0, success: 0, failed: 0 });
                      return { data: [], total: 0, success: false };
                    }
                  }}
                  expandable={{
                    expandedRowRender: (record) => (
                      <div className="audit-expand">
                        <Space direction="vertical" size={6}>
                          <Text>
                            <LinkOutlined /> SHA256: {record.statement_sha256}
                          </Text>
                          <Text>
                            <DatabaseOutlined />{' '}
                            {record.user_email || `User #${record.user_id}`}
                          </Text>
                          {renderAuditDetails(record.details)}
                          {record.error && (
                            <Text type="danger">{record.error}</Text>
                          )}
                        </Space>
                      </div>
                    ),
                    rowExpandable: () => true,
                  }}
                  scroll={{ x: 1320 }}
                />
              </div>
            ),
          },
          {
            key: 'management',
            label: tr('管理审计', 'Management Audit'),
            children: (
              <div className="audit-empty-panel">
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description={tr(
                    '管理审计暂未接入，后续会归档登录、用户、连接、配置等管理操作。',
                    'Management audit is not connected yet. Login, user, connection, and configuration changes will appear here.',
                  )}
                />
              </div>
            ),
          },
        ]}
      />
    </PageContainer>
  );
};

const buildAuditQuery = (
  params: Record<string, any>,
): API.WebDataAuditListParams => {
  const range = Array.isArray(params.time_range) ? params.time_range : [];
  const successValue = params.success;
  const proxyID = Number(params.proxy_id);
  return {
    page: params.current,
    page_size: params.pageSize,
    proxy_id: Number.isFinite(proxyID) && proxyID > 0 ? proxyID : undefined,
    protocol: params.protocol,
    action: params.action,
    success:
      successValue === true || successValue === 'true'
        ? true
        : successValue === false || successValue === 'false'
        ? false
        : undefined,
    keyword: params.keyword,
    start_time: range[0] ? new Date(range[0]).toISOString() : undefined,
    end_time: range[1] ? new Date(range[1]).toISOString() : undefined,
  };
};

const formatAuditTime = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
};

const accessDetailPath = (record: API.WebDataAuditItem) => {
  if (record.protocol === 'ssh') {
    return `/webssh/${record.proxy_id}`;
  }
  return `/webdata/${record.proxy_id}`;
};

const auditDetailLabels: Record<string, string> = {
  database: '数据库',
  schema: 'Schema',
  redis_db: 'Redis DB',
  ssh_user: 'SSH 用户',
  affected_rows: '影响行',
  client_ip: '来源 IP',
  client_ip_source: 'IP 来源',
};

const renderAuditDetails = (details?: Record<string, any>) => {
  const entries = Object.entries(details || {}).filter(([, value]) => {
    if (value === undefined || value === null || value === '') return false;
    if (Array.isArray(value)) return value.length > 0;
    if (typeof value === 'object') return Object.keys(value).length > 0;
    return true;
  });
  if (!entries.length) return null;
  return (
    <div className="audit-detail-grid">
      {entries.map(([key, value]) => (
        <div className="audit-detail-item" key={key}>
          <span>{auditDetailLabels[key] || key}</span>
          <strong>{formatAuditDetailValue(value)}</strong>
        </div>
      ))}
    </div>
  );
};

const formatAuditDetailValue = (value: any) => {
  if (typeof value === 'boolean') return value ? 'true' : 'false';
  if (typeof value === 'number') return String(value);
  if (typeof value === 'string') return value;
  return JSON.stringify(value);
};

export default AuditPage;
