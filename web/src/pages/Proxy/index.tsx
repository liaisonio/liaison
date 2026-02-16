import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProFormDigit,
  ProFormSelect,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Space, Tag, Typography, Switch, Alert, Tooltip, message } from 'antd';
import { CheckCircleOutlined } from '@ant-design/icons';
import { useRef, useState, useEffect } from 'react';
import { useSearchParams, useLocation } from '@umijs/max';
import {
  getProxyList,
  createProxy,
  updateProxy,
  deleteProxy,
  getApplicationList,
} from '@/services/api';
import { executeAction, tableRequest } from '@/utils/request';
import { CreateButton, EditLink, DeleteLink } from '@/components/TableButtons';
import { defaultPagination, defaultSearch, buildSearchParams } from '@/utils/tableConfig';

const { Text } = Typography;

const ProxyPage: React.FC = () => {
  const actionRef = useRef<ActionType>();
  const createFormRef = useRef<any>();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [currentRow, setCurrentRow] = useState<API.Proxy>();
  const [initialApplicationId, setInitialApplicationId] = useState<number | undefined>();
  const [applicationOptions, setApplicationOptions] = useState<
    { label: string; value: number; application_type?: string }[]
  >([]);
  const [applicationMap, setApplicationMap] = useState<Map<number, API.Application>>(new Map());
  const [selectedApplicationId, setSelectedApplicationId] = useState<number | undefined>();
  const hasProcessedUrlRef = useRef(false); // 使用 ref 跟踪是否已处理过 URL 参数

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
            const exists = prev.some(opt => opt.value === id);
            if (exists) {
              // 如果已存在，更新 label（使用URL中的名称）
              return prev.map(opt => 
                opt.value === id ? { ...opt, label: decodedName } : opt
              );
            }
            // 如果不存在，添加到 options 中
            return [...prev, { label: decodedName, value: id }];
          });
        }
        
        // 重新加载应用列表，确保包含完整信息（IP和端口）
        getApplicationList({ page_size: 100 }).then((res) => {
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
            const exists = options.some(opt => opt.value === id);
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
        }).catch(() => {
          // 忽略错误，但如果URL中有应用名称，至少保留它
          if (applicationName) {
            const decodedName = decodeURIComponent(applicationName);
            setApplicationOptions((prev) => {
              const exists = prev.some(opt => opt.value === id);
              if (exists) return prev;
              return [...prev, { label: decodedName, value: id, application_type: '' }];
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

  const handleAdd = async (values: any) => {
    const createPort = values.port || undefined;
    
    const result = await executeAction(
      () => createProxy({
        name: values.name,
        description: values.description,
        port: createPort,
        application_id: values.application_id,
      }),
      {
        successMessage: '创建成功',
        errorMessage: '创建失败',
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
    return executeAction(
      () => updateProxy(currentRow.id, {
        name: values.name,
        description: values.description,
        port: values.port,
      }),
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
    await executeAction(() => deleteProxy(id), {
      successMessage: '删除成功',
      errorMessage: '删除失败',
      onSuccess: reload,
    });
  };

  const columns: ProColumns<API.Proxy>[] = [
    {
      title: '访问名称',
      dataIndex: 'name',
      ellipsis: true,
      copyable: true,
      fieldProps: {
        placeholder: '请输入访问名称',
      },
    },
    {
      title: '描述',
      dataIndex: 'description',
      ellipsis: true,
      search: false,
    },
    {
      title: '访问地址',
      dataIndex: 'access_url',
      ellipsis: false,
      search: false,
      width: 300,
      render: (_, record) => {
        const accessUrl = record.access_url;
        if (!accessUrl || typeof accessUrl !== 'string') {
          return <Text type="secondary">-</Text>;
        }
        // 确保 URL 有协议前缀
        const url = accessUrl.startsWith('http://') || accessUrl.startsWith('https://') 
          ? accessUrl 
          : `https://${accessUrl}`;
        
        return (
          <Tag
            color="blue"
            style={{
              fontSize: '12px',
              cursor: 'pointer',
            }}
            onClick={() => {
              window.open(url, '_blank');
            }}
          >
            {accessUrl}
          </Tag>
        );
      },
    },
    {
      title: '公网端口',
      dataIndex: 'port',
      width: 100,
      search: false,
      render: (port) => <Tag color="blue">{port}</Tag>,
    },
    {
      title: '关联应用',
      dataIndex: ['application', 'name'],
      ellipsis: true,
      search: false,
      render: (_, record) =>
        record.application ? (
          <Space direction="vertical" size={0}>
            <Space>
              <Text>{record.application.name}</Text>
              {record.application.application_type === 'http' && (
                <Tooltip title={<span style={{ fontSize: '11px' }}>已开启 HTTPS</span>}>
                  <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 16 }} />
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
      title: '状态',
      dataIndex: 'status',
      width: 100,
      search: false,
      valueEnum: {
        running: { text: '运行中', status: 'Success' },
        stopped: { text: '已停止', status: 'Default' },
        error: { text: '异常', status: 'Error' },
      },
    },
    {
      title: '启用',
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
              () => updateProxy(record.id, {
                name: record.name,
                description: record.description,
                port: record.port,
                status: newStatus,
              }),
              {
                successMessage: checked ? '已启用' : '已停用',
                errorMessage: '操作失败',
                onSuccess: reload,
              },
            );
          }}
        />
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'created_at',
      valueType: 'dateTime',
      width: 180,
      search: false,
    },
    {
      title: '操作',
      valueType: 'option',
      width: 120,
      fixed: 'right',
      align: 'center',
      render: (_, record) => (
        <Space>
          <EditLink onClick={() => {
            setCurrentRow(record);
            setEditModalVisible(true);
          }} />
          <DeleteLink
            title="确定要删除这个访问吗？"
            onConfirm={() => handleDelete(record.id)}
          />
        </Space>
      ),
    },
  ];

  return (
    <PageContainer title="访问">
      <div className="table-search-wrapper">
        <ProTable<API.Proxy>
        headerTitle="访问列表"
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        request={async (params) => {
          const searchParams = buildSearchParams<API.ProxyListParams>(params, ['name']);
          return tableRequest(() => getProxyList(searchParams), 'proxies');
        }}
        toolBarRender={() => [
          <CreateButton key="create" onClick={() => setCreateModalVisible(true)}>
            新建访问
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
        title="新建访问"
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
          } else if (initialApplicationId) {
            setSelectedApplicationId(initialApplicationId);
          }
        }}
        onFinish={handleAdd}
        modalProps={{ destroyOnClose: true }}
        width={500}
      >
        <ProFormText
          name="name"
          label="访问名称"
          placeholder="请输入访问名称"
          rules={[{ required: true, message: '请输入访问名称' }]}
        />
        <ProFormSelect
          name="application_id"
          label="关联应用"
          placeholder="请选择要访问的应用"
          rules={[{ required: true, message: '请选择应用' }]}
          options={applicationOptions}
          fieldProps={{
            onChange: (value: number) => {
              setSelectedApplicationId(value);
            },
          }}
        />
        {selectedApplicationId && applicationMap.get(selectedApplicationId)?.application_type === 'http' && (
          <Alert
            message={<span style={{ fontSize: '11px', lineHeight: '16px', marginBottom: 0, display: 'block' }}>将开启 HTTPS</span>}
            description={<span style={{ fontSize: '10px', lineHeight: '14px', marginTop: 0, display: 'block' }}>HTTP 应用将默认使用 HTTPS 协议访问，使用系统配置的 TLS 证书</span>}
            type="info"
            icon={<CheckCircleOutlined style={{ color: '#52c41a', fontSize: '14px' }} />}
            style={{ marginBottom: 16, padding: '8px 12px' }}
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
        <ProFormTextArea
          name="description"
          label="描述"
          placeholder="请输入访问描述"
        />
      </ModalForm>

      <ModalForm
        title="编辑访问"
        open={editModalVisible}
        onOpenChange={setEditModalVisible}
        onFinish={handleEdit}
        modalProps={{ destroyOnClose: true }}
        initialValues={currentRow}
        width={500}
      >
        <ProFormText
          name="name"
          label="访问名称"
          placeholder="请输入访问名称"
          rules={[{ required: true, message: '请输入访问名称' }]}
        />
        <ProFormDigit
          name="port"
          label="公网端口"
          placeholder="请输入端口"
          min={1}
          max={65535}
        />
        <ProFormTextArea
          name="description"
          label="描述"
          placeholder="请输入访问描述"
        />
      </ModalForm>
    </PageContainer>
  );
};

export default ProxyPage;
