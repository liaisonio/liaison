import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProTable,
} from '@ant-design/pro-components';
import { Space, Tag, Typography, Select, Form, Alert } from 'antd';
import { CheckCircleOutlined } from '@ant-design/icons';
import { LinkOutlined, ApiOutlined } from '@ant-design/icons';
import { useRef, useState } from 'react';
import {
  getApplicationList,
  createApplication,
  updateApplication,
  deleteApplication,
  getEdgeList,
  createProxy,
  getDeviceList,
} from '@/services/api';
import { executeAction, tableRequest } from '@/utils/request';
import { CreateButton, DeleteLink } from '@/components/TableButtons';
import { defaultPagination, defaultSearch, buildSearchParams } from '@/utils/tableConfig';

const { Text } = Typography;

const AppPage: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const formRef = useRef<any>();
  const [createForm] = Form.useForm();
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [proxyModalVisible, setProxyModalVisible] = useState(false);
  const [currentRow, setCurrentRow] = useState<API.Application>();
  const [selectedApplicationType, setSelectedApplicationType] = useState<string | undefined>();
  const [deviceOptions, setDeviceOptions] = useState<{ label: string; value: string }[]>([]);

  const reload = () => actionRef.current?.reload();

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

  const handleAdd = async (values: any) => {
    return executeAction(
      () =>
        createApplication({
          name: values.name,
          application_type: values.application_type,
          ip: values.ip,
          port: values.port,
          edge_id: values.edge_id,
          device_id: values.device_id,
        }),
      {
        successMessage: '创建成功',
        errorMessage: '创建失败',
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
        successMessage: '更新成功',
        errorMessage: '更新失败',
        onSuccess: () => {
          setEditModalVisible(false);
          reload();
        },
      },
    );
  };

  const handleDelete = async (id: number) => {
    await executeAction(() => deleteApplication(id), {
      successMessage: '删除成功',
      errorMessage: '删除失败',
      onSuccess: reload,
    });
  };

  const handleCreateProxy = async (values: any) => {
    if (!currentRow?.id) return false;
    const createPort = values.port || undefined;
    let createdProxy: API.Proxy | null = null;
    
    const result = await executeAction(
      () =>
        createProxy({
          name: values.name || currentRow.name,
          description: values.description,
          port: createPort,
          application_id: currentRow.id,
        }),
      {
        successMessage: '访问创建成功',
        errorMessage: '访问创建失败',
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
      title: '应用名称',
      dataIndex: 'name',
      ellipsis: true,
      fieldProps: {
        placeholder: '请输入应用名称',
      },
      render: (_, record) => (
        <Space>
          <ApiOutlined />
          <span>{record.name}</span>
        </Space>
      ),
    },
    {
      title: '类型',
      dataIndex: 'application_type',
      width: 100,
      valueType: 'select',
      valueEnum: {
        http: { text: 'HTTP' },
        tcp: { text: 'TCP' },
        ssh: { text: 'SSH' },
        rdp: { text: 'RDP' },
        mysql: { text: 'MySQL' },
        postgresql: { text: 'PostgreSQL' },
        redis: { text: 'Redis' },
        mongodb: { text: 'MongoDB' },
      },
      fieldProps: {
        placeholder: '请选择应用类型',
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
      title: 'IP 地址',
      dataIndex: 'ip',
      width: 140,
      search: false,
      render: (ip) => <Text code>{ip}</Text>,
    },
    {
      title: '端口',
      dataIndex: 'port',
      width: 80,
      search: false,
      render: (port) => <Tag>{port}</Tag>,
    },
    {
      title: '所在设备',
      dataIndex: 'device_name',
      ellipsis: true,
      width: 150,
      valueType: 'select',
      render: (_, record) => record.device?.name || '-',
      fieldProps: {
        placeholder: '请选择设备',
        showSearch: true,
        allowClear: true,
        options: deviceOptions,
        filterOption: (input: string, option?: { label: string; value: string }) =>
          (option?.label ?? '').toLowerCase().includes(input.toLowerCase()),
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
      title: '已关联访问',
      dataIndex: 'proxy',
      ellipsis: true,
      width: 150,
      search: false,
      render: (_, record) => {
        if (record.proxy) {
          return (
            <Tag color="blue">
              <LinkOutlined /> {record.proxy.name}:{record.proxy.port}
            </Tag>
          );
        }
        return <Tag>未关联</Tag>;
      },
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      width: 170,
      search: false,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 180,
      fixed: 'right',
      align: 'center',
      render: (_, record) => (
        <Space>
          <a onClick={() => {
            setCurrentRow(record);
            setProxyModalVisible(true);
          }}>
            创建访问
          </a>
          <a onClick={() => {
            setCurrentRow(record);
            setEditModalVisible(true);
          }}>
            编辑
          </a>
          <DeleteLink
            title="确定要删除这个应用吗？"
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
        headerTitle="应用列表"
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        columns={columns}
        request={async (params) => {
          console.log('ProTable request params:', params);
          const searchParams = buildSearchParams<API.ApplicationListParams>(params, ['name', 'device_name', 'application_type']);
          console.log('buildSearchParams result:', searchParams);
          return tableRequest(() => getApplicationList(searchParams), 'applications');
        }}
        onSubmit={(values) => {
          console.log('ProTable onSubmit:', values);
          // 触发表格刷新，此时会使用表单值
          actionRef.current?.reload();
        }}
        toolBarRender={() => [
          <CreateButton key="create" onClick={() => setCreateModalVisible(true)}>
            新建应用
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
        title="新建应用"
        open={createModalVisible}
        onOpenChange={(visible) => {
          setCreateModalVisible(visible);
          if (!visible) {
            setSelectedApplicationType(undefined);
          }
        }}
        onFinish={handleAdd}
        modalProps={{ destroyOnClose: true }}
        form={createForm}
        width={500}
      >
        <ProFormText
          name="name"
          label="应用名称"
          placeholder="请输入应用名称"
          rules={[{ required: true, message: '请输入应用名称' }]}
        />
        <ProFormSelect
          name="application_type"
          label="应用类型"
          placeholder="请选择应用类型（不填默认为TCP）"
          options={[
            { label: 'HTTP', value: 'http' },
            { label: 'TCP', value: 'tcp' },
            { label: 'SSH', value: 'ssh' },
            { label: 'RDP', value: 'rdp' },
            { label: 'MySQL', value: 'mysql' },
            { label: 'PostgreSQL', value: 'postgresql' },
            { label: 'Redis', value: 'redis' },
            { label: 'MongoDB', value: 'mongodb' },
          ]}
          extra="不填默认为TCP"
          fieldProps={{
            onChange: (value: string) => {
              setSelectedApplicationType(value);
              // 根据应用类型设置默认端口
              const defaultPorts: Record<string, number> = {
                http: 80,
                ssh: 22,
                rdp: 3389,
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
            message={<span style={{ fontSize: '11px', lineHeight: '16px', marginBottom: 0, display: 'block' }}>将开启 HTTPS</span>}
            description={<span style={{ fontSize: '10px', lineHeight: '14px', marginTop: 0, display: 'block' }}>HTTP 应用将默认使用 HTTPS 协议访问，使用系统配置的 TLS 证书</span>}
            type="info"
            icon={<CheckCircleOutlined style={{ color: '#52c41a', fontSize: '14px' }} />}
            style={{ marginBottom: 16, padding: '8px 12px' }}
            messageStyle={{ marginBottom: 0 }}
            descriptionStyle={{ marginTop: 0 }}
          />
        )}
        <ProFormText
          name="ip"
          label="IP 地址"
          placeholder="请输入应用 IP 地址，如 192.168.1.100"
          rules={[{ required: true, message: '请输入 IP 地址' }]}
        />
        <ProFormDigit
          name="port"
          label="端口"
          placeholder="请输入端口号"
          min={1}
          max={65535}
          rules={[
            { required: true, message: '请输入端口号' },
            {
              validator: (_: any, value: number) => {
                if (!value || value === 0) {
                  return Promise.reject(new Error('端口号不能为0，请输入1-65535之间的端口号'));
                }
                if (value < 1 || value > 65535) {
                  return Promise.reject(new Error('端口号必须在1-65535之间'));
                }
                return Promise.resolve();
              },
            },
          ]}
        />
        <ProFormSelect
          name="edge_id"
          label="连接器"
          placeholder="请选择连接器"
          rules={[{ required: true, message: '请选择连接器' }]}
          request={async () => {
            try {
              const res = await getEdgeList({ page_size: 100 });
              return (
                res.data?.edges?.map((item: API.Edge) => ({
                  label: item.name,
                  value: item.id,
                })) || []
              );
            } catch {
              return [];
            }
          }}
        />
      </ModalForm>

      <ModalForm
        title="编辑应用"
        open={editModalVisible}
        onOpenChange={setEditModalVisible}
        onFinish={handleEdit}
        modalProps={{ destroyOnClose: true }}
        initialValues={currentRow}
        width={500}
      >
        <ProFormText
          name="name"
          label="应用名称"
          placeholder="请输入应用名称"
          rules={[{ required: true, message: '请输入应用名称' }]}
        />
      </ModalForm>

      <ModalForm
        title="为应用创建访问"
        open={proxyModalVisible}
        onOpenChange={setProxyModalVisible}
        onFinish={handleCreateProxy}
        modalProps={{ destroyOnClose: true }}
        width={500}
      >
        <ProFormText
          name="name"
          label="访问名称"
          placeholder="请输入访问名称"
          initialValue={currentRow?.name}
          rules={[{ required: true, message: '请输入访问名称' }]}
        />
        {currentRow?.application_type === 'http' && (
          <Alert
            message={<span style={{ fontSize: '11px', lineHeight: '16px', marginBottom: 0, display: 'block' }}>将开启 HTTPS</span>}
            description={<span style={{ fontSize: '10px', lineHeight: '14px', marginTop: 0, display: 'block' }}>HTTP 应用将默认使用 HTTPS 协议访问，使用系统配置的 TLS 证书</span>}
            type="info"
            icon={<CheckCircleOutlined style={{ color: '#52c41a', fontSize: '14px' }} />}
            style={{ marginBottom: 16, padding: '8px 12px' }}
            messageStyle={{ marginBottom: 0 }}
            descriptionStyle={{ marginTop: 0 }}
          />
        )}
        <ProFormDigit
          name="port"
          label="公网端口"
          placeholder="留空自动分配"
          min={1}
          max={65535}
          extra="映射到公网的端口号，留空则自动分配"
        />
        <ProFormText
          name="description"
          label="描述"
          placeholder="请输入描述"
        />
      </ModalForm>
    </PageContainer>
  );
};

export default AppPage;
