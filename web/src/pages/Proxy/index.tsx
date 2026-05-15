import { CreateButton, DeleteLink, EditLink } from '@/components/TableButtons';
import { useI18n } from '@/i18n';
import {
  createProxy,
  deleteProxy,
  deleteProxyFirewall,
  getApplicationList,
  getClientIP,
  getProxyFirewall,
  getProxyList,
  updateProxy,
  upsertProxyFirewall,
} from '@/services/api';
import { executeAction, tableRequest } from '@/utils/request';
import {
  buildSearchParams,
  defaultPagination,
  defaultSearch,
} from '@/utils/tableConfig';
import { CheckCircleOutlined } from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { history, useLocation, useSearchParams } from '@umijs/max';
import {
  Alert,
  Button,
  Drawer,
  Input,
  Popconfirm,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  Tooltip,
  Typography,
  message,
} from 'antd';
import { useEffect, useRef, useState } from 'react';

const { Text } = Typography;

const webOnlyApplicationTypes = new Set([
  'ssh',
  'rdp',
  'vnc',
  'mysql',
  'postgresql',
  'redis',
  'mongodb',
  'database',
]);
const protocolTagColors: Record<string, string> = {
  http: 'green',
  tcp: 'blue',
  ssh: 'purple',
  rdp: 'geekblue',
  vnc: 'cyan',
  mysql: 'volcano',
  postgresql: 'processing',
  redis: 'red',
  mongodb: 'success',
};
const protocolLabels: Record<string, string> = {
  http: 'HTTP',
  tcp: 'TCP',
  ssh: 'SSH',
  rdp: 'RDP',
  vnc: 'VNC',
  mysql: 'MySQL',
  postgresql: 'PostgreSQL',
  redis: 'Redis',
  mongodb: 'MongoDB',
};

const ProxyPage: React.FC = () => {
  const { tr } = useI18n();
  const actionRef = useRef<ActionType>();
  const createFormRef = useRef<any>();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [currentRow, setCurrentRow] = useState<API.Proxy>();
  const [initialApplicationId, setInitialApplicationId] = useState<
    number | undefined
  >();
  const [applicationOptions, setApplicationOptions] = useState<
    { label: string; value: number; application_type?: string }[]
  >([]);
  const [applicationMap, setApplicationMap] = useState<
    Map<number, API.Application>
  >(new Map());
  const [selectedApplicationId, setSelectedApplicationId] = useState<
    number | undefined
  >();
  const [createExposePublicPort, setCreateExposePublicPort] = useState(false);
  const [editExposePublicPort, setEditExposePublicPort] = useState(false);
  const hasProcessedUrlRef = useRef(false); // 使用 ref 跟踪是否已处理过 URL 参数

  // 防火墙 Drawer 状态
  type FirewallDrawerState = {
    open: boolean;
    loading: boolean;
    record?: API.Proxy;
    draftCIDRs: string[];
    updatedAt: string;
    hasRule: boolean; // 后端返回 updated_at 非空代表显式规则
  };
  const [firewallDrawer, setFirewallDrawer] = useState<FirewallDrawerState>({
    open: false,
    loading: false,
    draftCIDRs: [],
    updatedAt: '',
    hasRule: false,
  });
  const [newFirewallCIDR, setNewFirewallCIDR] = useState('');
  const [clientIP, setClientIP] = useState<string | null>(null);
  const [firewallSaving, setFirewallSaving] = useState(false);

  const isValidIPv4Address = (value: string): boolean => {
    const parts = value.split('.');
    if (parts.length !== 4) return false;
    return parts.every((part) => {
      if (!/^\d+$/.test(part)) return false;
      if (part.length > 1 && part.startsWith('0')) return false;
      const octet = Number(part);
      return octet >= 0 && octet <= 255;
    });
  };

  // IPv4 地址或带前缀的 IPv4 CIDR。纯 IP 保存时会自动补 /32。
  const isValidCIDR = (value: string): boolean => {
    const trimmed = value.trim();
    const parts = trimmed.split('/');
    if (parts.length > 2 || !isValidIPv4Address(parts[0])) return false;
    if (parts.length === 1) return true;
    if (!/^\d+$/.test(parts[1])) return false;
    const prefix = Number(parts[1]);
    return prefix >= 0 && prefix <= 32;
  };
  const normalizeCIDR = (value: string) => {
    const trimmed = value.trim();
    return trimmed.includes('/') ? trimmed : `${trimmed}/32`;
  };

  const validatePublicPort = async (
    value?: number,
    excludeProxyID?: number,
  ) => {
    if (value === undefined || value === null) return Promise.resolve();
    if (!Number.isInteger(value) || value < 1 || value > 65535) {
      return Promise.reject(
        new Error(
          tr(
            '公网端口必须在1-65535之间',
            'Public port must be between 1 and 65535',
          ),
        ),
      );
    }
    try {
      const res = await getProxyList({ page_size: 10000 });
      const conflict = res.data?.proxies?.find(
        (proxy: API.Proxy) =>
          proxy.port === value && proxy.id !== excludeProxyID,
      );
      if (conflict) {
        return Promise.reject(
          new Error(
            tr(
              `公网端口 ${value} 已被访问「${conflict.name}」使用`,
              `Public port ${value} is already used by entry "${conflict.name}"`,
            ),
          ),
        );
      }
    } catch {
      // 后端仍会做最终冲突校验；列表预校验失败时不阻塞表单。
    }
    return Promise.resolve();
  };

  const openFirewallDrawer = async (record: API.Proxy) => {
    setFirewallDrawer({
      open: true,
      loading: true,
      record,
      draftCIDRs: [],
      updatedAt: '',
      hasRule: false,
    });
    setNewFirewallCIDR('');
    setClientIP(null);
    getClientIP()
      .then((res) => {
        if (res?.data?.ip) setClientIP(res.data.ip);
      })
      .catch(() => {});

    try {
      const res = await getProxyFirewall(record.id);
      if (res.code !== 200 || !res.data) {
        message.error(
          res.message || tr('获取防火墙失败', 'Failed to load firewall'),
        );
        setFirewallDrawer({
          open: false,
          loading: false,
          draftCIDRs: [],
          updatedAt: '',
          hasRule: false,
        });
        return;
      }
      const hasRule = !!res.data.updated_at;
      setFirewallDrawer({
        open: true,
        loading: false,
        record,
        draftCIDRs: hasRule ? [...(res.data.allowed_cidrs || [])] : [],
        updatedAt: res.data.updated_at || '',
        hasRule,
      });
    } catch (err: any) {
      message.error(
        err?.message || tr('获取防火墙失败', 'Failed to load firewall'),
      );
      setFirewallDrawer({
        open: false,
        loading: false,
        draftCIDRs: [],
        updatedAt: '',
        hasRule: false,
      });
    }
  };

  const closeFirewallDrawer = () => {
    setFirewallDrawer((prev) => ({ ...prev, open: false }));
  };

  const addFirewallCIDR = (value: string) => {
    const raw = value.trim();
    if (!raw) return;
    if (!isValidCIDR(raw)) {
      message.error(
        tr(
          '请输入合法的 IPv4 地址或 CIDR，例如 203.0.113.1 或 203.0.113.0/24',
          'Enter a valid IPv4 address or CIDR, e.g. 203.0.113.1 or 203.0.113.0/24',
        ),
      );
      return;
    }
    const cidr = normalizeCIDR(raw);
    if (firewallDrawer.draftCIDRs.includes(cidr)) {
      message.warning(tr('该 CIDR 已存在', 'This CIDR already exists'));
      return;
    }
    setFirewallDrawer((prev) => ({
      ...prev,
      draftCIDRs: [...prev.draftCIDRs, cidr],
    }));
    setNewFirewallCIDR('');
  };

  const removeFirewallCIDR = (cidr: string) => {
    setFirewallDrawer((prev) => ({
      ...prev,
      draftCIDRs: prev.draftCIDRs.filter((c) => c !== cidr),
    }));
  };

  const handleFirewallSave = async () => {
    const record = firewallDrawer.record;
    if (!record?.id) return;
    const invalidCIDR = firewallDrawer.draftCIDRs.find(
      (cidr) => !isValidCIDR(cidr),
    );
    if (invalidCIDR) {
      message.error(
        tr(
          `来源规则 ${invalidCIDR} 不是合法的 IPv4 CIDR`,
          `Source rule ${invalidCIDR} is not a valid IPv4 CIDR`,
        ),
      );
      return;
    }
    setFirewallSaving(true);
    await executeAction(
      () =>
        upsertProxyFirewall(record.id, {
          allowed_cidrs: firewallDrawer.draftCIDRs,
        }),
      {
        successMessage:
          firewallDrawer.draftCIDRs.length === 0
            ? tr('已保存（空规则 = 拒绝全部）', 'Saved (empty rule = deny all)')
            : tr('防火墙已更新', 'Firewall updated'),
        errorMessage: tr('防火墙更新失败', 'Failed to update firewall'),
        onSuccess: () => {
          setFirewallDrawer({
            open: false,
            loading: false,
            draftCIDRs: [],
            updatedAt: '',
            hasRule: false,
          });
        },
      },
    );
    setFirewallSaving(false);
  };

  const handleFirewallReset = async () => {
    const record = firewallDrawer.record;
    if (!record?.id) return;
    await executeAction(() => deleteProxyFirewall(record.id), {
      successMessage: tr('已恢复为放行全部', 'Reset to allow-all'),
      errorMessage: tr('操作失败', 'Operation failed'),
      onSuccess: () => {
        setFirewallDrawer({
          open: false,
          loading: false,
          draftCIDRs: [],
          updatedAt: '',
          hasRule: false,
        });
      },
    });
  };

  // 从 URL 查询参数中读取 application_id、application_name 和 autoCreate（只执行一次）
  useEffect(() => {
    // 如果已经处理过 URL 参数，不再重复处理
    if (hasProcessedUrlRef.current) return;

    // 优先从 searchParams 读取
    let applicationId = searchParams.get('application_id');
    let applicationName = searchParams.get('application_name');
    let autoCreate = searchParams.get('autoCreate');

    // 如果 searchParams 没有，从 location.search 读取
    if (location.search) {
      const urlParams = new URLSearchParams(location.search);
      if (!applicationId) {
        applicationId = urlParams.get('application_id');
      }
      if (!applicationName) {
        applicationName = urlParams.get('application_name');
      }
      if (!autoCreate) {
        autoCreate = urlParams.get('autoCreate');
      }
    }

    // 如果 application_id 存在，设置初始值
    if (applicationId) {
      const id = parseInt(applicationId, 10);
      if (!isNaN(id)) {
        setInitialApplicationId(id);

        // 如果 URL 中有应用名称，立即添加到 options 中（避免等待列表加载）
        if (applicationName) {
          const decodedName = decodeURIComponent(applicationName);
          setApplicationOptions((prev) => {
            // 检查是否已经在 options 中
            const exists = prev.some((opt) => opt.value === id);
            if (exists) {
              // 如果已存在，更新 label（使用URL中的名称）
              return prev.map((opt) =>
                opt.value === id ? { ...opt, label: decodedName } : opt,
              );
            }
            // 如果不存在，添加到 options 中
            return [...prev, { label: decodedName, value: id }];
          });
        }

        // 重新加载应用列表，确保包含完整信息（IP和端口）
        getApplicationList({ page_size: 100 })
          .then((res) => {
            const apps = res.data?.applications || [];
            const appMap = new Map<number, API.Application>();
            apps.forEach((app: API.Application) => {
              appMap.set(app.id, app);
            });
            setApplicationMap(appMap);

            const options =
              apps.map((item: API.Application) => ({
                label: `${item.name} (${item.ip}:${item.port})`,
                value: item.id,
                application_type: item.application_type,
              })) || [];
            // 如果URL中有应用名称，确保对应的选项存在（即使列表中没有）
            if (applicationName) {
              const decodedName = decodeURIComponent(applicationName);
              const exists = options.some((opt) => opt.value === id);
              if (!exists) {
                // 尝试从 appMap 中获取应用类型，如果找不到则默认为空
                const app = appMap.get(id);
                options.push({
                  label: decodedName,
                  value: id,
                  application_type: app?.application_type || '',
                });
              }
            }
            setApplicationOptions(options);
          })
          .catch(() => {
            // 忽略错误，但如果URL中有应用名称，至少保留它
            if (applicationName) {
              const decodedName = decodeURIComponent(applicationName);
              setApplicationOptions((prev) => {
                const exists = prev.some((opt) => opt.value === id);
                if (exists) return prev;
                return [
                  ...prev,
                  { label: decodedName, value: id, application_type: '' },
                ];
              });
            }
          });
      }
    }

    // 如果 autoCreate 为 true，自动打开对话框
    if (autoCreate === 'true') {
      hasProcessedUrlRef.current = true; // 标记为已处理
      setCreateModalVisible(true);
      // 清除 URL 中的查询参数
      window.history.replaceState({}, '', '/proxy');
    } else if (applicationId) {
      // 如果没有 autoCreate，但有 application_id，也打开对话框（向后兼容）
      hasProcessedUrlRef.current = true; // 标记为已处理
      setCreateModalVisible(true);
      // 清除 URL 中的查询参数
      window.history.replaceState({}, '', '/proxy');
    } else {
      // 如果没有相关参数，也标记为已处理，避免重复检查
      hasProcessedUrlRef.current = true;
    }
  }, [searchParams, location.search]);

  // 页面加载时就拉应用列表
  useEffect(() => {
    const loadApplications = async () => {
      try {
        const res = await getApplicationList({ page_size: 100 });
        const apps = res.data?.applications || [];
        const appMap = new Map<number, API.Application>();
        apps.forEach((app: API.Application) => {
          appMap.set(app.id, app);
        });
        setApplicationMap(appMap);

        const options =
          apps.map((item: API.Application) => ({
            label: `${item.name} (${item.ip}:${item.port})`,
            value: item.id,
            application_type: item.application_type,
          })) || [];
        setApplicationOptions(options);
      } catch {
        setApplicationOptions([]);
      }
    };

    loadApplications();
  }, []);

  const reload = () => actionRef.current?.reload();

  const isWebOnlyCapableType = (type?: string) =>
    webOnlyApplicationTypes.has(String(type || '').toLowerCase());

  const isProxyPublicPortExposed = (record?: API.Proxy) =>
    Boolean(record?.expose_public_port ?? (record?.port || 0) > 0);

  const isWebOnlyProxy = (record?: API.Proxy) =>
    Boolean(
      record &&
        isWebOnlyCapableType(record.application?.application_type) &&
        !isProxyPublicPortExposed(record),
    );

  const renderProtocolTag = (applicationType?: string) => {
    const type = String(applicationType || '').toLowerCase();
    if (!type) return '-';
    return (
      <Tag color={protocolTagColors[type] || 'default'} bordered={false}>
        {protocolLabels[type] || type.toUpperCase()}
      </Tag>
    );
  };

  const selectedApplication = selectedApplicationId
    ? applicationMap.get(selectedApplicationId)
    : undefined;
  const selectedApplicationWebOnlyCapable = isWebOnlyCapableType(
    selectedApplication?.application_type,
  );

  const effectiveStatusMeta = (record: API.Proxy) => {
    const status =
      record.effective_status ||
      (record.status === 'running' ? 'active' : 'stopped');
    const map: Record<
      string,
      { text: string; color: string; reason?: string }
    > = {
      active: { text: tr('可用', 'Active'), color: 'success' },
      stopped: {
        text: tr('不可用', 'Unavailable'),
        color: 'default',
        reason: tr('访问未启用', 'Access disabled'),
      },
      edge_stopped: {
        text: tr('不可用', 'Unavailable'),
        color: 'warning',
        reason: tr('连接器已禁用', 'Edge disabled'),
      },
      edge_offline: {
        text: tr('不可用', 'Unavailable'),
        color: 'processing',
        reason: tr('连接器离线', 'Edge offline'),
      },
      invalid: {
        text: tr('不可用', 'Unavailable'),
        color: 'error',
        reason: tr('配置无效', 'Invalid'),
      },
    };
    return map[status] || { text: status, color: 'default' };
  };

  const handleAdd = async (values: any) => {
    const app = applicationMap.get(values.application_id);
    const webCapable = isWebOnlyCapableType(app?.application_type);
    const exposePublicPort = webCapable
      ? Boolean(values.expose_public_port)
      : true;
    const createPort = exposePublicPort ? values.port || undefined : 0;

    const result = await executeAction(
      () =>
        createProxy({
          name: values.name?.trim(),
          description: values.description,
          port: createPort,
          expose_public_port: exposePublicPort,
          application_id: values.application_id,
        }),
      {
        successMessage: tr('创建成功', 'Created successfully'),
        errorMessage: tr('创建失败', 'Create failed'),
        onSuccess: () => {
          // 如果创建时端口为空，后端会动态分配端口并在响应中返回
          // 刷新列表即可显示动态分配的端口
        },
      },
    );

    setCreateModalVisible(false);
    reload();

    return result;
  };

  const handleEdit = async (values: any) => {
    if (!currentRow?.id) return false;
    const webCapable = isWebOnlyCapableType(
      currentRow.application?.application_type,
    );
    const exposePublicPort = webCapable
      ? Boolean(values.expose_public_port)
      : true;
    return executeAction(
      () =>
        updateProxy(currentRow.id, {
          name: values.name?.trim(),
          description: values.description,
          port: exposePublicPort ? values.port : 0,
          expose_public_port: exposePublicPort,
        }),
      {
        successMessage: tr('更新成功', 'Updated successfully'),
        errorMessage: tr('更新失败', 'Update failed'),
        onSuccess: () => {
          setEditModalVisible(false);
          reload();
        },
      },
    );
  };

  const handleDelete = async (id: number) => {
    await executeAction(() => deleteProxy(id), {
      successMessage: tr('删除成功', 'Deleted successfully'),
      errorMessage: tr('删除失败', 'Delete failed'),
      onSuccess: reload,
    });
  };

  const columns: ProColumns<API.Proxy>[] = [
    {
      title: tr('访问名称', 'Entry Name'),
      dataIndex: 'name',
      ellipsis: true,
      copyable: true,
      fieldProps: {
        placeholder: tr('请输入访问名称', 'Please input entry name'),
      },
    },
    {
      title: tr('描述', 'Description'),
      dataIndex: 'description',
      ellipsis: true,
      search: false,
    },
    {
      title: tr('公网端口', 'Public Port'),
      dataIndex: 'port',
      width: 100,
      search: false,
      render: (_, record) =>
        isWebOnlyProxy(record) ? (
          <Tag color="purple">{tr('仅 Web', 'Web only')}</Tag>
        ) : (
          <Tag color="blue">{record.port}</Tag>
        ),
    },
    {
      title: tr('协议类型', 'Protocol'),
      dataIndex: ['application', 'application_type'],
      width: 110,
      search: false,
      render: (_, record) =>
        renderProtocolTag(record.application?.application_type),
    },
    {
      title: tr('关联应用', 'Application'),
      dataIndex: ['application', 'name'],
      ellipsis: true,
      search: false,
      render: (_, record) =>
        record.application ? (
          <Space direction="vertical" size={0}>
            <Space>
              <Text>{record.application.name}</Text>
              {record.application.application_type === 'http' && (
                <Tooltip
                  title={
                    <span style={{ fontSize: '11px' }}>
                      {tr('已开启 HTTPS', 'HTTPS enabled')}
                    </span>
                  }
                >
                  <CheckCircleOutlined
                    style={{ color: '#52c41a', fontSize: 16 }}
                  />
                </Tooltip>
              )}
            </Space>
            <Text type="secondary" className="text-xs">
              {record.application.ip}:{record.application.port}
            </Text>
          </Space>
        ) : (
          '-'
        ),
    },
    {
      title: tr('当前状态', 'Current Status'),
      dataIndex: 'effective_status',
      width: 140,
      search: false,
      render: (_, record) => {
        const meta = effectiveStatusMeta(record);
        return (
          <Tooltip
            title={record.effective_status_message || meta.reason || meta.text}
          >
            <Tag color={meta.color}>{meta.text}</Tag>
          </Tooltip>
        );
      },
    },
    {
      title: tr('启用', 'Enabled'),
      dataIndex: 'enabled',
      width: 80,
      search: false,
      align: 'center',
      render: (_, record) => (
        <Switch
          checked={record.status === 'running'}
          onChange={async (checked) => {
            const newStatus = checked ? 'running' : 'stopped';
            await executeAction(
              () =>
                updateProxy(record.id, {
                  name: record.name,
                  description: record.description,
                  port: record.port,
                  status: newStatus,
                }),
              {
                successMessage: checked
                  ? tr('已启用', 'Enabled')
                  : tr('已停用', 'Disabled'),
                errorMessage: tr('操作失败', 'Operation failed'),
                onSuccess: reload,
              },
            );
          }}
        />
      ),
    },
    {
      title: tr('创建时间', 'Created At'),
      dataIndex: 'created_at',
      valueType: 'dateTime',
      width: 180,
      search: false,
    },
    {
      title: tr('操作', 'Actions'),
      valueType: 'option',
      width: 240,
      fixed: 'right',
      align: 'center',
      render: (_, record) => {
        const accessUrl = record.access_url;
        const isSSH = record.application?.application_type === 'ssh';
        const isWebDesktop =
          record.application?.application_type === 'rdp' ||
          record.application?.application_type === 'vnc';
        const isWebData = [
          'mysql',
          'postgresql',
          'redis',
          'mongodb',
          'database',
        ].includes(
          String(record.application?.application_type || '').toLowerCase(),
        );
        const isActive =
          (record.effective_status ||
            (record.status === 'running' ? 'active' : 'stopped')) === 'active';
        const url =
          accessUrl && typeof accessUrl === 'string'
            ? accessUrl.startsWith('http://') ||
              accessUrl.startsWith('https://')
              ? accessUrl
              : `https://${accessUrl}`
            : null;

        return (
          <Space>
            {isActive && (isSSH || isWebDesktop || isWebData || url) && (
              <Tooltip
                title={
                  <span style={{ fontSize: '12px' }}>
                    {isSSH
                      ? tr('在网页终端中打开 SSH', 'Open SSH in web terminal')
                      : isWebDesktop
                      ? tr('在网页远程桌面中打开', 'Open in web desktop')
                      : isWebData
                      ? tr('在网页数据控制台中打开', 'Open in web data console')
                      : accessUrl}
                  </span>
                }
              >
                <Button
                  type="link"
                  size="small"
                  style={{ padding: 0, height: 'auto' }}
                  onClick={() => {
                    if (isSSH) {
                      history.push(`/webssh/${record.id}`);
                      return;
                    }
                    if (isWebDesktop) {
                      history.push(`/webdesktop/${record.id}`);
                      return;
                    }
                    if (isWebData) {
                      history.push(`/webdata/${record.id}`);
                      return;
                    }
                    if (url) {
                      window.open(url, '_blank');
                    }
                  }}
                >
                  {tr('去访问', 'Open')}
                </Button>
              </Tooltip>
            )}
            {isWebOnlyProxy(record) ? (
              <Tooltip
                title={tr(
                  '仅网页访问不创建公网端口，无需入口防火墙',
                  'Web-only entries do not expose a public port, so entry firewall is not needed',
                )}
              >
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {tr('防火墙', 'Firewall')}
                </Text>
              </Tooltip>
            ) : (
              <Button
                type="link"
                size="small"
                style={{ padding: 0, height: 'auto' }}
                onClick={() => openFirewallDrawer(record)}
              >
                {tr('防火墙', 'Firewall')}
              </Button>
            )}
            <EditLink
              onClick={() => {
                setCurrentRow(record);
                setEditExposePublicPort(isProxyPublicPortExposed(record));
                setEditModalVisible(true);
              }}
            />
            <DeleteLink
              title={tr('确定要删除这个访问吗？', 'Delete this entry?')}
              description={tr(
                '将同步停止公网监听并删除访问规则',
                'The public listener and access rules will be removed together',
              )}
              onConfirm={() => handleDelete(record.id)}
            />
          </Space>
        );
      },
    },
  ];

  return (
    <PageContainer title={tr('访问', 'Entries')}>
      <div className="table-search-wrapper">
        <ProTable<API.Proxy>
          headerTitle={tr('访问列表', 'Entries')}
          actionRef={actionRef}
          rowKey="id"
          columns={columns}
          request={async (params) => {
            const searchParams = buildSearchParams<API.ProxyListParams>(
              params,
              ['name'],
            );
            return tableRequest(() => getProxyList(searchParams), 'proxies');
          }}
          toolBarRender={() => [
            <CreateButton
              key="create"
              onClick={() => setCreateModalVisible(true)}
            >
              {tr('新建访问', 'New Entry')}
            </CreateButton>,
          ]}
          pagination={defaultPagination}
          search={{
            ...defaultSearch,
            labelWidth: 'auto',
          }}
          scroll={{ x: 'max-content' }}
        />
      </div>

      <ModalForm
        key={initialApplicationId ?? 'create'}
        title={tr('新建访问', 'New Entry')}
        open={createModalVisible}
        formRef={createFormRef}
        initialValues={
          initialApplicationId
            ? { application_id: initialApplicationId }
            : undefined
        }
        onOpenChange={(visible) => {
          setCreateModalVisible(visible);
          if (!visible) {
            setInitialApplicationId(undefined);
            setSelectedApplicationId(undefined);
            setCreateExposePublicPort(false);
          } else if (initialApplicationId) {
            setSelectedApplicationId(initialApplicationId);
            const app = applicationMap.get(initialApplicationId);
            const expose = !isWebOnlyCapableType(app?.application_type);
            setCreateExposePublicPort(expose);
            createFormRef.current?.setFieldsValue?.({
              expose_public_port: expose,
            });
          }
        }}
        onFinish={handleAdd}
        modalProps={{ destroyOnClose: true }}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('访问名称', 'Entry Name')}
          placeholder={tr('请输入访问名称', 'Please input entry name')}
          rules={[
            {
              required: true,
              message: tr('请输入访问名称', 'Please input entry name'),
            },
          ]}
        />
        <ProFormSelect
          name="application_id"
          label={tr('关联应用', 'Application')}
          placeholder={tr('请选择要访问的应用', 'Please select an application')}
          rules={[
            {
              required: true,
              message: tr('请选择应用', 'Please select an application'),
            },
          ]}
          options={applicationOptions}
          fieldProps={{
            onChange: (value: number) => {
              setSelectedApplicationId(value);
              const app = applicationMap.get(value);
              const expose = !isWebOnlyCapableType(app?.application_type);
              setCreateExposePublicPort(expose);
              createFormRef.current?.setFieldsValue?.({
                expose_public_port: expose,
                port: undefined,
              });
            },
          }}
        />
        {selectedApplicationId &&
          selectedApplication?.application_type === 'http' && (
            <Alert
              message={
                <span
                  style={{
                    fontSize: '11px',
                    lineHeight: '16px',
                    marginBottom: 0,
                    display: 'block',
                  }}
                >
                  {tr('将开启 HTTPS', 'HTTPS will be enabled')}
                </span>
              }
              description={
                <span
                  style={{
                    fontSize: '10px',
                    lineHeight: '14px',
                    marginTop: 0,
                    display: 'block',
                  }}
                >
                  {tr(
                    'HTTP 应用将默认使用 HTTPS 协议访问，使用系统配置的 TLS 证书',
                    'HTTP applications will be exposed over HTTPS with configured TLS certificates',
                  )}
                </span>
              }
              type="info"
              icon={
                <CheckCircleOutlined
                  style={{ color: '#52c41a', fontSize: '14px' }}
                />
              }
              style={{ marginBottom: 16, padding: '8px 12px' }}
            />
          )}
        {selectedApplicationWebOnlyCapable && (
          <Alert
            message={tr(
              '可独立控制是否开放公网端口',
              'Public port exposure is controlled separately',
            )}
            description={tr(
              '关闭时只能通过网页控制台访问；开启时会创建公网监听端口，端口留空则自动分配。',
              'When disabled, access is web-console only. When enabled, a public listener is created; leave the port empty to auto-allocate.',
            )}
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}
        {selectedApplicationWebOnlyCapable && (
          <ProFormSwitch
            name="expose_public_port"
            label={tr('开放公网端口', 'Expose Public Port')}
            initialValue={false}
            fieldProps={{
              onChange: (checked) => {
                setCreateExposePublicPort(Boolean(checked));
                if (!checked) {
                  createFormRef.current?.setFieldValue?.('port', undefined);
                }
              },
            }}
            extra={tr(
              '关闭后仅允许网页访问，不创建对外监听端口',
              'Disable to allow web-only access without an external listener',
            )}
          />
        )}
        {(!selectedApplicationWebOnlyCapable || createExposePublicPort) && (
          <ProFormDigit
            name="port"
            label={tr('公网端口', 'Public Port')}
            placeholder={tr('留空自动分配', 'Leave empty for auto allocation')}
            min={1}
            max={65535}
            fieldProps={{ precision: 0 }}
            rules={[
              {
                validator: (_: any, value?: number) =>
                  validatePublicPort(value),
              },
            ]}
            extra={tr(
              '映射到公网的端口号，留空则自动分配',
              'Mapped public port, leave empty to auto-allocate',
            )}
          />
        )}
        <ProFormTextArea
          name="description"
          label={tr('描述', 'Description')}
          placeholder={tr('请输入访问描述', 'Please input description')}
        />
      </ModalForm>

      <ModalForm
        title={tr('编辑访问', 'Edit Entry')}
        open={editModalVisible}
        onOpenChange={setEditModalVisible}
        onFinish={handleEdit}
        modalProps={{ destroyOnClose: true }}
        initialValues={{
          ...currentRow,
          port: isProxyPublicPortExposed(currentRow)
            ? currentRow?.port
            : undefined,
          expose_public_port: isProxyPublicPortExposed(currentRow),
        }}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('访问名称', 'Entry Name')}
          placeholder={tr('请输入访问名称', 'Please input entry name')}
          rules={[
            {
              required: true,
              message: tr('请输入访问名称', 'Please input entry name'),
            },
          ]}
        />
        {isWebOnlyCapableType(currentRow?.application?.application_type) && (
          <ProFormSwitch
            name="expose_public_port"
            label={tr('开放公网端口', 'Expose Public Port')}
            fieldProps={{
              onChange: (checked) => {
                setEditExposePublicPort(Boolean(checked));
                if (!checked) {
                  // 关闭时后端会把端口持久化为 0。
                }
              },
            }}
            extra={tr(
              '关闭后只能通过网页控制台访问；开启后可通过公网端口直连',
              'Disable for web-console-only access; enable to allow direct public-port connections',
            )}
          />
        )}
        {(!isWebOnlyCapableType(currentRow?.application?.application_type) ||
          editExposePublicPort) && (
          <ProFormDigit
            name="port"
            label={tr('公网端口', 'Public Port')}
            placeholder={tr('留空自动分配', 'Leave empty for auto allocation')}
            min={1}
            max={65535}
            fieldProps={{ precision: 0 }}
            rules={[
              {
                validator: (_: any, value?: number) =>
                  validatePublicPort(value, currentRow?.id),
              },
            ]}
          />
        )}
        <ProFormTextArea
          name="description"
          label={tr('描述', 'Description')}
          placeholder={tr('请输入访问描述', 'Please input description')}
        />
      </ModalForm>

      <Drawer
        title={
          firewallDrawer.record
            ? `${tr('设置防火墙', 'Configure Firewall')} · ${
                firewallDrawer.record.name
              }`
            : tr('设置防火墙', 'Configure Firewall')
        }
        open={firewallDrawer.open}
        onClose={closeFirewallDrawer}
        destroyOnClose
        placement="right"
        width={600}
        extra={
          <Space>
            <Popconfirm
              title={tr('恢复为放行全部？', 'Reset to allow-all?')}
              description={tr(
                '删除规则后，任何来源 IP 都能访问此代理。',
                'After removal, any source IP can reach this proxy.',
              )}
              okText={tr('确认', 'Confirm')}
              cancelText={tr('取消', 'Cancel')}
              onConfirm={handleFirewallReset}
              disabled={!firewallDrawer.hasRule}
            >
              <Button danger disabled={!firewallDrawer.hasRule}>
                {tr('恢复默认', 'Reset')}
              </Button>
            </Popconfirm>
            <Button onClick={closeFirewallDrawer}>
              {tr('取消', 'Cancel')}
            </Button>
            <Button
              type="primary"
              loading={firewallSaving}
              onClick={handleFirewallSave}
            >
              {tr('保存', 'Save')}
            </Button>
          </Space>
        }
      >
        {firewallDrawer.loading ? (
          <div style={{ textAlign: 'center', padding: 40 }}>
            <Spin />
          </div>
        ) : (
          <>
            {/* Info card */}
            <div
              style={{
                marginBottom: 16,
                padding: '12px 16px',
                borderRadius: 8,
                background: 'var(--ant-color-fill-quaternary)',
                border: '1px solid var(--ant-color-border-secondary)',
              }}
            >
              <div style={{ fontWeight: 600, marginBottom: 4 }}>
                {tr('入口访问规则', 'Entry access rules')}
              </div>
              <Text type="secondary" style={{ fontSize: 13 }}>
                {tr(
                  '为当前代理配置来源 IP 白名单。保存后立即生效；HTTP/TCP 代理均在 Accept 时按 L4 过滤。支持单个 IPv4 或 CIDR（纯 IP 将自动补 /32）。',
                  'Configure source IP allowlist for this proxy. Takes effect immediately; both HTTP and TCP are filtered at Accept. Supports single IPv4 or CIDR (a bare IP is auto-completed to /32).',
                )}
              </Text>
              <div
                style={{
                  marginTop: 10,
                  display: 'flex',
                  gap: 16,
                  flexWrap: 'wrap',
                }}
              >
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {tr('端口', 'Port')}:{' '}
                  <strong>{firewallDrawer.record?.port || '-'}</strong>
                </Text>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {tr('规则数', 'Rules')}:{' '}
                  <strong>{firewallDrawer.draftCIDRs.length}</strong>
                </Text>
                {firewallDrawer.updatedAt && (
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {tr('最近更新', 'Updated')}: {firewallDrawer.updatedAt}
                  </Text>
                )}
              </div>
              <div
                style={{
                  marginTop: 10,
                  display: 'flex',
                  alignItems: 'center',
                  gap: 8,
                }}
              >
                <Text type="secondary" style={{ fontSize: 12 }}>
                  {tr('我的 IP', 'My IP')}:{' '}
                  {clientIP ? (
                    <Text code style={{ fontSize: 12 }}>
                      {clientIP}
                    </Text>
                  ) : (
                    <Text type="secondary" style={{ fontSize: 12 }}>
                      ...
                    </Text>
                  )}
                </Text>
                {clientIP &&
                  !firewallDrawer.draftCIDRs.includes(`${clientIP}/32`) &&
                  !firewallDrawer.draftCIDRs.includes(clientIP) && (
                    <Button
                      size="small"
                      type="link"
                      style={{ padding: 0, fontSize: 12, height: 'auto' }}
                      onClick={() => addFirewallCIDR(clientIP)}
                    >
                      {tr('添加', 'Add')}
                    </Button>
                  )}
                {clientIP &&
                  (firewallDrawer.draftCIDRs.includes(`${clientIP}/32`) ||
                    firewallDrawer.draftCIDRs.includes(clientIP)) && (
                    <Text type="success" style={{ fontSize: 12 }}>
                      ✓ {tr('已添加', 'Added')}
                    </Text>
                  )}
              </div>
            </div>

            {/* Rules card */}
            <div
              style={{
                borderRadius: 8,
                border: '1px solid var(--ant-color-border-secondary)',
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: '10px 16px',
                  borderBottom: '1px solid var(--ant-color-border-secondary)',
                  background: 'var(--ant-color-bg-container)',
                }}
              >
                <Text strong style={{ fontSize: 13 }}>
                  {tr('来源规则', 'Source Rules')}
                </Text>
                <Input.Search
                  value={newFirewallCIDR}
                  onChange={(e) => setNewFirewallCIDR(e.target.value)}
                  onSearch={addFirewallCIDR}
                  placeholder={tr(
                    '输入 IP 或 CIDR，如 203.0.113.1 或 203.0.113.0/24',
                    'Enter IP or CIDR, e.g. 203.0.113.1 or 203.0.113.0/24',
                  )}
                  enterButton={tr('添加', 'Add')}
                  size="small"
                  style={{ width: 320 }}
                />
              </div>
              <Table
                size="middle"
                rowKey="cidr"
                pagination={{
                  pageSize: 10,
                  size: 'small',
                  hideOnSinglePage: true,
                  showTotal: (total) => `${total} ${tr('条', 'rules')}`,
                }}
                locale={{
                  emptyText: tr(
                    '暂无规则。保存空列表 = 拒绝全部；点右上角「恢复默认」= 放行全部。',
                    'No rules. Save empty list = deny all; click "Reset" top-right = allow all.',
                  ),
                }}
                dataSource={firewallDrawer.draftCIDRs.map((cidr) => ({ cidr }))}
                columns={[
                  {
                    title: tr('来源 CIDR', 'Source CIDR'),
                    dataIndex: 'cidr',
                    render: (v: string) => <Text code>{v}</Text>,
                  },
                  {
                    title: tr('协议', 'Protocol'),
                    width: 90,
                    render: () => {
                      const at =
                        firewallDrawer.record?.application?.application_type;
                      const isHTTP = at === 'http';
                      return (
                        <Tag color={isHTTP ? 'green' : 'blue'} bordered={false}>
                          {isHTTP ? 'HTTP' : 'TCP'}
                        </Tag>
                      );
                    },
                  },
                  {
                    title: tr('端口', 'Port'),
                    width: 90,
                    render: () => (
                      <Tag bordered={false}>
                        {firewallDrawer.record?.port || '-'}
                      </Tag>
                    ),
                  },
                  {
                    title: tr('策略', 'Policy'),
                    width: 80,
                    render: () => (
                      <Tag color="success" bordered={false}>
                        {tr('允许', 'Allow')}
                      </Tag>
                    ),
                  },
                  {
                    title: tr('操作', 'Actions'),
                    width: 70,
                    render: (_, row: { cidr: string }) => (
                      <Button
                        type="link"
                        danger
                        size="small"
                        style={{ padding: 0 }}
                        onClick={() => removeFirewallCIDR(row.cidr)}
                      >
                        {tr('删除', 'Delete')}
                      </Button>
                    ),
                  },
                ]}
              />
            </div>
          </>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default ProxyPage;
