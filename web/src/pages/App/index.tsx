import { CreateButton, DeleteLink } from '@/components/TableButtons';
import { useI18n } from '@/i18n';
import {
  createApplication,
  createProxy,
  deleteApplication,
  getApplicationList,
  getDeviceDetail,
  getDeviceList,
  getEdgeList,
  getProxyList,
  updateApplication,
} from '@/services/api';
import { executeAction, tableRequest } from '@/utils/request';
import {
  buildSearchParams,
  defaultPagination,
  defaultSearch,
} from '@/utils/tableConfig';
import {
  ApiOutlined,
  CheckCircleOutlined,
  LinkOutlined,
} from '@ant-design/icons';
import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormSwitch,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { Alert, AutoComplete, Form, Space, Tag, Typography } from 'antd';
import { useRef, useState } from 'react';

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

type SelectOption = {
  label: string;
  value: string;
};

type EdgeOption = {
  label: string;
  value: number;
  deviceId?: number;
};

const AppPage: React.FC = () => {
  const { tr } = useI18n();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<any>();
  const [createForm] = Form.useForm();
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [proxyModalVisible, setProxyModalVisible] = useState(false);
  const [currentRow, setCurrentRow] = useState<API.Application>();
  const [proxyExposePublicPort, setProxyExposePublicPort] = useState(false);
  const [selectedApplicationType, setSelectedApplicationType] = useState<
    string | undefined
  >();
  const [selectedCreateEdgeId, setSelectedCreateEdgeId] = useState<
    number | undefined
  >();
  const [applicationIpOptions, setApplicationIpOptions] = useState<
    SelectOption[]
  >([]);
  const [applicationIpDropdownOpen, setApplicationIpDropdownOpen] =
    useState(false);
  const suppressNextIpDropdownOpenRef = useRef(false);
  const [deviceOptions, setDeviceOptions] = useState<
    { label: string; value: string }[]
  >([]);

  const reload = () => actionRef.current?.reload();

  const isWebOnlyCapableType = (type?: string) =>
    webOnlyApplicationTypes.has(String(type || '').toLowerCase());

  const normalizeInterfaceIP = (ip?: string) => {
    const value = String(ip || '').trim();
    if (!value) return '';
    return value.split('/')[0].trim();
  };

  const buildDeviceIpOptions = (device?: API.Device): SelectOption[] => {
    const seen = new Set<string>();
    const options: SelectOption[] = [];
    const deviceName = device?.name?.trim();
    for (const iface of device?.interfaces || []) {
      const ips = [...(iface.ip || []), ...(iface.ipv4 || [])];
      for (const rawIP of ips) {
        const ip = normalizeInterfaceIP(rawIP);
        if (!ip || ip.includes(':') || seen.has(ip)) continue;
        seen.add(ip);
        const interfaceText = iface.name ? ` (${iface.name})` : '';
        options.push({
          label: deviceName
            ? `${deviceName} - ${ip}${interfaceText}`
            : `${ip}${interfaceText}`,
          value: ip,
        });
      }
    }
    return options;
  };

  const formatEntryPort = (proxy?: API.Proxy, applicationType?: string) =>
    proxy && proxy.port === 0 && isWebOnlyCapableType(applicationType)
      ? tr('仅 Web', 'Web only')
      : proxy?.port;

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

  const isValidApplicationHost = (value: string): boolean => {
    const host = value.trim();
    if (
      !host ||
      host.length > 253 ||
      host.includes('/') ||
      host.includes(' ')
    ) {
      return false;
    }
    if (host.toLowerCase() === 'localhost') return true;
    if (host.includes(':')) return false;
    if (isValidIPv4Address(host)) return true;
    if (/^\d+\.\d+\.\d+\.\d+$/.test(host)) return false;
    return host.split('.').every((label) => {
      if (!label || label.length > 63) return false;
      if (label.startsWith('-') || label.endsWith('-')) return false;
      return /^[A-Za-z0-9-]+$/.test(label);
    });
  };

  const validatePublicPort = async (value?: number) => {
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
        (proxy: API.Proxy) => proxy.port === value,
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

  // 加载设备列表
  const loadDeviceOptions = async () => {
    if (deviceOptions.length > 0) return; // 已加载过，不再重复加载
    try {
      const res = await getDeviceList({ page_size: 100 });
      const options = (res.data?.devices || []).map((device: API.Device) => ({
        label: device.name,
        value: device.name,
      }));
      setDeviceOptions(options);
    } catch {
      // 忽略错误
    }
  };

  const loadCreateEdgeOptions = async (): Promise<EdgeOption[]> => {
    try {
      const res = await getEdgeList({ page_size: 100 });
      const edges = res.data?.edges || [];
      return edges.map((item: API.Edge) => ({
        label: item.device?.name
          ? `${item.name} (${item.device.name})`
          : item.name,
        value: item.id,
        deviceId: item.device?.id,
      }));
    } catch {
      return [];
    }
  };

  const loadCreateEdgeDeviceIps = async (deviceID?: number) => {
    setApplicationIpOptions([]);
    setApplicationIpDropdownOpen(false);
    if (!deviceID) return;

    try {
      const res = await getDeviceDetail(deviceID);
      const options = buildDeviceIpOptions(res.data);
      setApplicationIpOptions(options);
      setApplicationIpDropdownOpen(false);
    } catch {
      setApplicationIpOptions([]);
      setApplicationIpDropdownOpen(false);
    }
  };

  const handleAdd = async (values: any) => {
    return executeAction(
      () =>
        createApplication({
          name: values.name?.trim(),
          application_type: values.application_type,
          ip: values.ip?.trim(),
          port: values.port,
          edge_id: values.edge_id,
          device_id: values.device_id,
        }),
      {
        successMessage: tr('创建成功', 'Created successfully'),
        errorMessage: tr('创建失败', 'Create failed'),
        onSuccess: () => {
          setCreateModalVisible(false);
          reload();
        },
      },
    );
  };

  const handleEdit = async (values: any) => {
    if (!currentRow?.id) return false;
    return executeAction(
      () => updateApplication(currentRow.id, { name: values.name }),
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
    await executeAction(() => deleteApplication(id), {
      successMessage: tr('删除成功', 'Deleted successfully'),
      errorMessage: tr('删除失败', 'Delete failed'),
      onSuccess: reload,
    });
  };

  const handleCreateProxy = async (values: any) => {
    if (!currentRow?.id) return false;
    const webCapable = isWebOnlyCapableType(currentRow.application_type);
    const exposePublicPort = webCapable
      ? Boolean(values.expose_public_port)
      : true;
    const createPort = exposePublicPort ? values.port || undefined : 0;
    let createdProxy: API.Proxy | null = null;

    const result = await executeAction(
      () =>
        createProxy({
          name: values.name?.trim() || currentRow.name,
          description: values.description,
          port: createPort,
          expose_public_port: exposePublicPort,
          application_id: currentRow.id,
        }),
      {
        successMessage: tr('访问创建成功', 'Entry created successfully'),
        errorMessage: tr('访问创建失败', 'Failed to create entry'),
        onSuccess: (data) => {
          // 保存创建的访问信息
          if (data) {
            createdProxy = data as API.Proxy;
          }
        },
      },
    );

    // 如果创建时端口为空，创建后获取动态分配的端口
    // 端口已经在响应中返回，刷新列表即可显示动态分配的端口
    setProxyModalVisible(false);
    reload();

    return result;
  };

  const columns: ProColumns<API.Application>[] = [
    {
      title: tr('应用名称', 'Application Name'),
      dataIndex: 'name',
      ellipsis: true,
      fieldProps: {
        placeholder: tr('请输入应用名称', 'Please input application name'),
      },
      render: (_, record) => (
        <Space>
          <ApiOutlined />
          <span>{record.name}</span>
        </Space>
      ),
    },
    {
      title: tr('类型', 'Type'),
      dataIndex: 'application_type',
      width: 100,
      valueType: 'select',
      valueEnum: {
        http: { text: 'HTTP' },
        tcp: { text: 'TCP' },
        ssh: { text: 'SSH' },
        rdp: { text: 'RDP' },
        vnc: { text: 'VNC' },
        mysql: { text: 'MySQL' },
        postgresql: { text: 'PostgreSQL' },
        redis: { text: 'Redis' },
        mongodb: { text: 'MongoDB' },
      },
      fieldProps: {
        placeholder: tr('请选择应用类型', 'Please select application type'),
        allowClear: true,
        onChange: (val: string) => {
          // 使用 formRef 获取表单实例并设置值
          if (formRef.current) {
            formRef.current.setFieldsValue({ application_type: val });
            // 触发表单提交
            formRef.current.submit();
          }
        },
      },
    },
    {
      title: tr('IP 地址', 'IP Address'),
      dataIndex: 'ip',
      width: 140,
      search: false,
      render: (ip) => <Text code>{ip}</Text>,
    },
    {
      title: tr('端口', 'Port'),
      dataIndex: 'port',
      width: 80,
      search: false,
      render: (port) => <Tag>{port}</Tag>,
    },
    {
      title: tr('所在设备', 'Device'),
      dataIndex: 'device_name',
      ellipsis: true,
      width: 150,
      valueType: 'select',
      render: (_, record) => record.device?.name || '-',
      fieldProps: {
        placeholder: tr('请选择设备', 'Please select device'),
        showSearch: true,
        allowClear: true,
        options: deviceOptions,
        filterOption: (
          input: string,
          option?: { label: string; value: string },
        ) => (option?.label ?? '').toLowerCase().includes(input.toLowerCase()),
        onFocus: loadDeviceOptions,
        onChange: (val: string) => {
          // 使用 formRef 获取表单实例并设置值
          if (formRef.current) {
            formRef.current.setFieldsValue({ device_name: val });
            // 触发表单提交
            formRef.current.submit();
          }
        },
      },
      formItemProps: {
        style: { marginBottom: 0, marginRight: 16 },
      },
    },
    {
      title: tr('已关联访问', 'Linked Entry'),
      dataIndex: 'proxy',
      ellipsis: true,
      width: 150,
      search: false,
      render: (_, record) => {
        if (record.proxy) {
          return (
            <Tag color="blue">
              <LinkOutlined /> {record.proxy.name}:
              {formatEntryPort(record.proxy, record.application_type)}
            </Tag>
          );
        }
        return <Tag>{tr('未关联', 'Not Linked')}</Tag>;
      },
    },
    {
      title: tr('创建时间', 'Created At'),
      dataIndex: 'created_at',
      valueType: 'dateTime',
      width: 170,
      search: false,
    },
    {
      title: tr('操作', 'Actions'),
      valueType: 'option',
      width: 180,
      fixed: 'right',
      align: 'center',
      render: (_, record) => (
        <Space>
          <a
            onClick={() => {
              setCurrentRow(record);
              setProxyExposePublicPort(
                !isWebOnlyCapableType(record.application_type),
              );
              setProxyModalVisible(true);
            }}
          >
            {tr('创建访问', 'Create Entry')}
          </a>
          <a
            onClick={() => {
              setCurrentRow(record);
              setEditModalVisible(true);
            }}
          >
            {tr('编辑', 'Edit')}
          </a>
          <DeleteLink
            title={tr('确定要删除这个应用吗？', 'Delete this application?')}
            description={tr(
              '将连带删除该应用下的所有访问和访问规则，历史流量记录会保留',
              'All entries and access rules under this application will be removed. Traffic history will be retained',
            )}
            onConfirm={() => handleDelete(record.id)}
          />
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <div className="table-search-wrapper">
        <ProTable<API.Application>
          headerTitle={tr('应用列表', 'Applications')}
          actionRef={actionRef}
          formRef={formRef}
          rowKey="id"
          columns={columns}
          request={async (params) => {
            console.log('ProTable request params:', params);
            const searchParams = buildSearchParams<API.ApplicationListParams>(
              params,
              ['name', 'device_name', 'application_type'],
            );
            console.log('buildSearchParams result:', searchParams);
            return tableRequest(
              () => getApplicationList(searchParams),
              'applications',
            );
          }}
          onSubmit={(values) => {
            console.log('ProTable onSubmit:', values);
            // 触发表格刷新，此时会使用表单值
            actionRef.current?.reload();
          }}
          toolBarRender={() => [
            <CreateButton
              key="create"
              onClick={() => setCreateModalVisible(true)}
            >
              {tr('新建应用', 'New Application')}
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
        title={tr('新建应用', 'New Application')}
        open={createModalVisible}
        onOpenChange={(visible) => {
          setCreateModalVisible(visible);
          if (!visible) {
            setSelectedApplicationType(undefined);
            setSelectedCreateEdgeId(undefined);
            setApplicationIpOptions([]);
            setApplicationIpDropdownOpen(false);
            createForm.resetFields();
          }
        }}
        onFinish={handleAdd}
        modalProps={{ destroyOnClose: true }}
        form={createForm}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('应用名称', 'Application Name')}
          placeholder={tr('请输入应用名称', 'Please input application name')}
          rules={[
            {
              required: true,
              message: tr('请输入应用名称', 'Please input application name'),
            },
          ]}
        />
        <ProFormSelect
          name="edge_id"
          label={tr('连接器', 'Edge')}
          placeholder={tr('请先选择连接器', 'Select an edge first')}
          rules={[
            {
              required: true,
              message: tr('请选择连接器', 'Please select edge'),
            },
          ]}
          request={loadCreateEdgeOptions}
          fieldProps={{
            showSearch: true,
            optionFilterProp: 'label',
            onChange: (value, option) => {
              const edgeID = Number(value) || undefined;
              const optionItem = Array.isArray(option) ? option[0] : option;
              const deviceID = Number((optionItem as EdgeOption)?.deviceId);
              setSelectedCreateEdgeId(edgeID);
              createForm.setFieldsValue({ ip: undefined });
              void loadCreateEdgeDeviceIps(deviceID || undefined);
            },
          }}
          extra={tr(
            '选择后，IP 输入框会列出该连接器所在设备的网卡 IP',
            'After selection, the IP field lists interface IPs from that edge device',
          )}
        />
        <ProFormSelect
          name="application_type"
          label={tr('应用类型', 'Application Type')}
          placeholder={tr(
            '请选择应用类型（不填默认为TCP）',
            'Please select application type (default TCP)',
          )}
          options={[
            { label: 'HTTP', value: 'http' },
            { label: 'TCP', value: 'tcp' },
            { label: 'SSH', value: 'ssh' },
            { label: 'RDP', value: 'rdp' },
            { label: 'VNC', value: 'vnc' },
            { label: 'MySQL', value: 'mysql' },
            { label: 'PostgreSQL', value: 'postgresql' },
            { label: 'Redis', value: 'redis' },
            { label: 'MongoDB', value: 'mongodb' },
          ]}
          extra={tr('不填默认为TCP', 'Default is TCP')}
          fieldProps={{
            onChange: (value: string) => {
              setSelectedApplicationType(value);
              // 根据应用类型设置默认端口
              const defaultPorts: Record<string, number> = {
                http: 80,
                ssh: 22,
                rdp: 3389,
                vnc: 5900,
                mysql: 3306,
                postgresql: 5432,
                redis: 6379,
                mongodb: 27017,
              };
              const defaultPort = defaultPorts[value as string];
              if (defaultPort) {
                createForm.setFieldsValue({ port: defaultPort });
              }
            },
          }}
        />
        {selectedApplicationType === 'http' && (
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
            messageStyle={{ marginBottom: 0 }}
            descriptionStyle={{ marginTop: 0 }}
          />
        )}
        <Form.Item
          name="ip"
          label={tr('IP 地址', 'IP Address')}
          rules={[
            {
              required: true,
              message: tr('请输入 IP 地址', 'Please input IP address'),
            },
            {
              validator: (_: any, value?: string) => {
                const trimmed = value?.trim();
                if (!trimmed) return Promise.resolve();
                if (!isValidApplicationHost(trimmed)) {
                  return Promise.reject(
                    new Error(
                      tr(
                        '请输入合法的 IPv4 地址、localhost 或主机名',
                        'Please input a valid IPv4 address, localhost, or hostname',
                      ),
                    ),
                  );
                }
                return Promise.resolve();
              },
            },
          ]}
          extra={tr(
            '可从当前设备 IP 中选择，也可以直接输入 IP、localhost 或主机名',
            'Select a current device IP or type an IP, localhost, or hostname',
          )}
        >
          <AutoComplete
            allowClear
            disabled={!selectedCreateEdgeId}
            open={
              Boolean(selectedCreateEdgeId) &&
              applicationIpOptions.length > 0 &&
              applicationIpDropdownOpen
            }
            options={applicationIpOptions}
            placeholder={
              selectedCreateEdgeId
                ? tr(
                    '选择当前设备 IP，或直接输入',
                    'Select a device IP or type one',
                  )
                : tr('请先选择连接器', 'Select an edge first')
            }
            filterOption={(input, option) => {
              const text = `${option?.label ?? ''} ${option?.value ?? ''}`;
              return text.toLowerCase().includes(input.toLowerCase());
            }}
            onFocus={() => {
              if (
                applicationIpOptions.length > 0 &&
                !suppressNextIpDropdownOpenRef.current
              ) {
                setApplicationIpDropdownOpen(true);
              }
            }}
            onClick={() => {
              if (
                applicationIpOptions.length > 0 &&
                !suppressNextIpDropdownOpenRef.current
              ) {
                setApplicationIpDropdownOpen(true);
              }
            }}
            onBlur={() => {
              window.setTimeout(() => setApplicationIpDropdownOpen(false), 100);
            }}
            onOpenChange={(open) => {
              if (open && suppressNextIpDropdownOpenRef.current) {
                return;
              }
              setApplicationIpDropdownOpen(Boolean(open));
            }}
            onSelect={() => {
              suppressNextIpDropdownOpenRef.current = true;
              setApplicationIpDropdownOpen(false);
              createForm.validateFields(['ip']).catch(() => undefined);
              window.setTimeout(() => {
                suppressNextIpDropdownOpenRef.current = false;
              }, 120);
            }}
          />
        </Form.Item>
        <ProFormDigit
          name="port"
          label={tr('端口', 'Port')}
          placeholder={tr('请输入端口号', 'Please input port')}
          min={1}
          max={65535}
          fieldProps={{ precision: 0 }}
          rules={[
            {
              required: true,
              message: tr('请输入端口号', 'Please input port'),
            },
            {
              validator: (_: any, value: number) => {
                if (!value || value === 0) {
                  return Promise.reject(
                    new Error(
                      tr(
                        '端口号不能为0，请输入1-65535之间的端口号',
                        'Port cannot be 0, valid range is 1-65535',
                      ),
                    ),
                  );
                }
                if (value < 1 || value > 65535) {
                  return Promise.reject(
                    new Error(
                      tr(
                        '端口号必须在1-65535之间',
                        'Port must be between 1 and 65535',
                      ),
                    ),
                  );
                }
                return Promise.resolve();
              },
            },
          ]}
        />
      </ModalForm>

      <ModalForm
        title={tr('编辑应用', 'Edit Application')}
        open={editModalVisible}
        onOpenChange={setEditModalVisible}
        onFinish={handleEdit}
        modalProps={{ destroyOnClose: true }}
        initialValues={currentRow}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('应用名称', 'Application Name')}
          placeholder={tr('请输入应用名称', 'Please input application name')}
          rules={[
            {
              required: true,
              message: tr('请输入应用名称', 'Please input application name'),
            },
          ]}
        />
      </ModalForm>

      <ModalForm
        title={tr('为应用创建访问', 'Create Entry for Application')}
        open={proxyModalVisible}
        onOpenChange={(visible) => {
          setProxyModalVisible(visible);
          if (!visible) {
            setProxyExposePublicPort(false);
          }
        }}
        onFinish={handleCreateProxy}
        modalProps={{ destroyOnClose: true }}
        initialValues={{ expose_public_port: false }}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('访问名称', 'Entry Name')}
          placeholder={tr('请输入访问名称', 'Please input entry name')}
          initialValue={currentRow?.name}
          rules={[
            {
              required: true,
              message: tr('请输入访问名称', 'Please input entry name'),
            },
          ]}
        />
        {currentRow?.application_type === 'http' && (
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
            messageStyle={{ marginBottom: 0 }}
            descriptionStyle={{ marginTop: 0 }}
          />
        )}
        {isWebOnlyCapableType(currentRow?.application_type) && (
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
        {isWebOnlyCapableType(currentRow?.application_type) && (
          <ProFormSwitch
            name="expose_public_port"
            label={tr('开放公网端口', 'Expose Public Port')}
            initialValue={false}
            fieldProps={{
              onChange: (checked) => setProxyExposePublicPort(Boolean(checked)),
            }}
            extra={tr(
              '关闭后仅允许网页访问，不创建对外监听端口',
              'Disable to allow web-only access without an external listener',
            )}
          />
        )}
        {(!isWebOnlyCapableType(currentRow?.application_type) ||
          proxyExposePublicPort) && (
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
        <ProFormText
          name="description"
          label={tr('描述', 'Description')}
          placeholder={tr('请输入描述', 'Please input description')}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default AppPage;
