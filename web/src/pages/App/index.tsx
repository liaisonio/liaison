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
import { useI18n } from '@/i18n';

const { Text } = Typography;

const AppPage: React.FC = () => {
  const { tr } = useI18n();
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
      title: tr('已关联访问', 'Linked Entry'),
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
          <a onClick={() => {
            setCurrentRow(record);
            setProxyModalVisible(true);
          }}>
            {tr('创建访问', 'Create Entry')}
          </a>
          <a onClick={() => {
            setCurrentRow(record);
            setEditModalVisible(true);
          }}>
            {tr('编辑', 'Edit')}
          </a>
          <DeleteLink
            title={tr('确定要删除这个应用吗？', 'Delete this application?')}
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
          rules={[{ required: true, message: tr('请输入应用名称', 'Please input application name') }]}
        />
        <ProFormSelect
          name="application_type"
          label={tr('应用类型', 'Application Type')}
          placeholder={tr('请选择应用类型（不填默认为TCP）', 'Please select application type (default TCP)')}
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
          extra={tr('不填默认为TCP', 'Default is TCP')}
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
            message={<span style={{ fontSize: '11px', lineHeight: '16px', marginBottom: 0, display: 'block' }}>{tr('将开启 HTTPS', 'HTTPS will be enabled')}</span>}
            description={<span style={{ fontSize: '10px', lineHeight: '14px', marginTop: 0, display: 'block' }}>{tr('HTTP 应用将默认使用 HTTPS 协议访问，使用系统配置的 TLS 证书', 'HTTP applications will be exposed over HTTPS with configured TLS certificates')}</span>}
            type="info"
            icon={<CheckCircleOutlined style={{ color: '#52c41a', fontSize: '14px' }} />}
            style={{ marginBottom: 16, padding: '8px 12px' }}
            messageStyle={{ marginBottom: 0 }}
            descriptionStyle={{ marginTop: 0 }}
          />
        )}
        <ProFormText
          name="ip"
          label={tr('IP 地址', 'IP Address')}
          placeholder={tr('请输入应用 IP 地址，如 192.168.1.100', 'Please input application IP, e.g. 192.168.1.100')}
          rules={[{ required: true, message: tr('请输入 IP 地址', 'Please input IP address') }]}
        />
        <ProFormDigit
          name="port"
          label={tr('端口', 'Port')}
          placeholder={tr('请输入端口号', 'Please input port')}
          min={1}
          max={65535}
          rules={[
            { required: true, message: tr('请输入端口号', 'Please input port') },
            {
              validator: (_: any, value: number) => {
                if (!value || value === 0) {
                  return Promise.reject(new Error(tr('端口号不能为0，请输入1-65535之间的端口号', 'Port cannot be 0, valid range is 1-65535')));
                }
                if (value < 1 || value > 65535) {
                  return Promise.reject(new Error(tr('端口号必须在1-65535之间', 'Port must be between 1 and 65535')));
                }
                return Promise.resolve();
              },
            },
          ]}
        />
        <ProFormSelect
          name="edge_id"
          label={tr('连接器', 'Edge')}
          placeholder={tr('请选择连接器', 'Please select edge')}
          rules={[{ required: true, message: tr('请选择连接器', 'Please select edge') }]}
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
          rules={[{ required: true, message: tr('请输入应用名称', 'Please input application name') }]}
        />
      </ModalForm>

      <ModalForm
        title={tr('为应用创建访问', 'Create Entry for Application')}
        open={proxyModalVisible}
        onOpenChange={setProxyModalVisible}
        onFinish={handleCreateProxy}
        modalProps={{ destroyOnClose: true }}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('访问名称', 'Entry Name')}
          placeholder={tr('请输入访问名称', 'Please input entry name')}
          initialValue={currentRow?.name}
          rules={[{ required: true, message: tr('请输入访问名称', 'Please input entry name') }]}
        />
        {currentRow?.application_type === 'http' && (
          <Alert
            message={<span style={{ fontSize: '11px', lineHeight: '16px', marginBottom: 0, display: 'block' }}>{tr('将开启 HTTPS', 'HTTPS will be enabled')}</span>}
            description={<span style={{ fontSize: '10px', lineHeight: '14px', marginTop: 0, display: 'block' }}>{tr('HTTP 应用将默认使用 HTTPS 协议访问，使用系统配置的 TLS 证书', 'HTTP applications will be exposed over HTTPS with configured TLS certificates')}</span>}
            type="info"
            icon={<CheckCircleOutlined style={{ color: '#52c41a', fontSize: '14px' }} />}
            style={{ marginBottom: 16, padding: '8px 12px' }}
            messageStyle={{ marginBottom: 0 }}
            descriptionStyle={{ marginTop: 0 }}
          />
        )}
        <ProFormDigit
          name="port"
          label={tr('公网端口', 'Public Port')}
          placeholder={tr('留空自动分配', 'Leave empty for auto allocation')}
          min={1}
          max={65535}
          extra={tr('映射到公网的端口号，留空则自动分配', 'Mapped public port, empty means auto allocation')}
        />
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
