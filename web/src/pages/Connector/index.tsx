import {
  ActionType,
  PageContainer,
  ProColumns,
  ProTable,
  StepsForm,
  ProFormText,
  ProFormTextArea,
  ProFormSelect,
  ModalForm,
} from '@ant-design/pro-components';
import {
  Badge,
  Button,
  Drawer,
  List,
  App,
  Modal,
  Space,
  Tag,
  Typography,
  Spin,
  Result,
  Alert,
  Select,
  Tabs,
} from 'antd';
import {
  CopyOutlined,
  CheckCircleOutlined,
  LoadingOutlined,
  ReloadOutlined,
} from '@ant-design/icons';
import { useRef, useState } from 'react';
import { history } from '@umijs/max';
import {
  getEdgeList,
  createEdge,
  updateEdge,
  deleteEdge,
  getEdgeScanTask,
  createEdgeScanTask,
  createApplication,
  getDeviceList,
} from '@/services/api';
import { executeAction, tableRequest } from '@/utils/request';
import { CreateButton, DeleteLink } from '@/components/TableButtons';
import { defaultPagination, defaultSearch, buildSearchParams } from '@/utils/tableConfig';
import { copyToClipboard } from '@/utils/format';
import { useI18n } from '@/i18n';

const { Text, Paragraph } = Typography;

const ConnectorPage: React.FC = () => {
  const { tr } = useI18n();
  const { message } = App.useApp();
  const actionRef = useRef<ActionType>();
  const formRef = useRef<any>();
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [discoverDrawerVisible, setDiscoverDrawerVisible] = useState(false);
  const [currentRow, setCurrentRow] = useState<API.Edge>();
  const [accessKeys, setAccessKeys] = useState<API.EdgeCreateResult>();
  const [scanTask, setScanTask] = useState<API.EdgeScanApplicationTask>();
  const [scanning, setScanning] = useState(false);
  const [deviceOptions, setDeviceOptions] = useState<{ label: string; value: string }[]>([]);
  const [installOS, setInstallOS] = useState<'windows' | 'other'>('other');

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

  const handleOpenCreateModal = () => {
    setCreateModalVisible(true);
    setAccessKeys(undefined);
  };

  const handleDelete = async (id: number) => {
    await executeAction(() => deleteEdge(id), {
      successMessage: tr('删除成功', 'Deleted successfully'),
      errorMessage: tr('删除失败', 'Delete failed'),
      onSuccess: reload,
    });
  };

  const handleEdit = async (values: any) => {
    if (!currentRow?.id) return false;
    return executeAction(
      () =>
        updateEdge(currentRow.id, {
          name: values.name,
          description: values.description,
          status: values.status,
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

  const handleDiscoverApps = async (edge: API.Edge) => {
    if (edge.online !== 1) {
      message.warning(tr('连接器不在线，无法扫描应用', 'Edge is offline, cannot scan applications'));
      return;
    }

    setCurrentRow(edge);
    setDiscoverDrawerVisible(true);
    setScanning(true);
    setScanTask(undefined);

    try {
      // 先查找是否有已存在的任务
      const existingRes = await getEdgeScanTask(edge.id);
      if (existingRes.code === 200 && existingRes.data) {
        const task = existingRes.data;
        // 如果有 Pending 或 Running 任务，直接展示
        if (task.task_status === 'pending' || task.task_status === 'running') {
          setScanTask(task);
          setScanning(false);
          return;
        }
        // 如果有 Completed 或 Failed 任务，直接展示（用户可以选择重新扫描）
        if (task.task_status === 'completed' || task.task_status === 'failed') {
          setScanTask(task);
          setScanning(false);
          return;
        }
      }

      // 没有任务或任务状态允许创建新任务，则创建新的扫描任务
      const createRes = await createEdgeScanTask({ 
        edge_id: edge.id,
        protocol: 'tcp',
      });
      if (createRes.code !== 200) {
        message.error(createRes.message || tr('创建扫描任务失败', 'Failed to create scan task'));
        setScanning(false);
        return;
      }
      await new Promise<void>(resolve => { setTimeout(resolve, 1000); });
      const res = await getEdgeScanTask(edge.id);
      if (res.code === 200 && res.data) {
        setScanTask(res.data);
      }
    } catch (error: any) {
      message.error(error?.message || tr('扫描失败', 'Scan failed'));
    } finally {
      setScanning(false);
    }
  };

  // 重新扫描应用（强制创建新任务）
  const handleRescan = async () => {
    if (!currentRow?.id) return;
    setScanning(true);
    setScanTask(undefined);

    try {
      const createRes = await createEdgeScanTask({ 
        edge_id: currentRow.id,
        protocol: 'tcp',
      });
      if (createRes.code !== 200) {
        message.error(createRes.message || tr('创建扫描任务失败', 'Failed to create scan task'));
        setScanning(false);
        return;
      }
      await new Promise<void>(resolve => { setTimeout(resolve, 1000); });
      const res = await getEdgeScanTask(currentRow.id);
      if (res.code === 200 && res.data) {
        setScanTask(res.data);
      }
    } catch (error: any) {
      message.error(error?.message || tr('扫描失败', 'Scan failed'));
    } finally {
      setScanning(false);
    }
  };

  const handleRefreshScan = async () => {
    if (!currentRow?.id) return;
    setScanning(true);
    try {
      const res = await getEdgeScanTask(currentRow.id);
      if (res.code === 200 && res.data) {
        setScanTask(res.data);
      }
    } catch {
      message.error(tr('获取扫描结果失败', 'Failed to get scan result'));
    } finally {
      setScanning(false);
    }
  };

  const handleAddDiscoveredApp = async (appStr: string) => {
    if (!currentRow?.id) return;
    // 解析应用字符串，格式是 "ip:port:type"
    const parts = appStr.split(':');
    const ip = parts[0];
    const port = parseInt(parts[1], 10);
    // 如果后端已经提供了类型，使用后端的类型；否则根据端口推断
    const appType = parts[2] || (() => {
      const portToType: Record<number, string> = {
        22: 'ssh',
        80: 'http',
        443: 'http',
        3389: 'rdp',
        3306: 'mysql',
        5432: 'postgresql',
        6379: 'redis',
        27017: 'mongodb',
      };
      return portToType[port] || 'tcp';
    })();

    // 显示确认对话框
    const handleAddOnly = async () => {
      // 只添加应用
      await executeAction(
        () =>
          createApplication({
            name: `App-${ip}:${port}`,
            application_type: appType,
            ip,
            port,
            edge_id: currentRow.id,
          }),
        {
          successMessage: tr('添加应用成功', 'Application added'),
          errorMessage: tr('添加应用失败', 'Failed to add application'),
          onSuccess: () => {
            if (scanTask) {
              setScanTask({
                ...scanTask,
                applications: scanTask.applications.filter((a) => a !== appStr),
              });
            }
          },
        },
      );
    };

    const modalInstance = Modal.confirm({
      title: tr('添加应用', 'Add Application'),
      content: (
        <div>
          <div style={{ marginBottom: 8 }}>{tr('确定要添加应用', 'Add application')} <strong>{ip}:{port}</strong> {tr('吗？是否同时创建访问？', 'and create an entry at the same time?')}</div>
        </div>
      ),
      width: 450,
      centered: true,
      closable: true,
      maskClosable: false, // 禁止点击遮罩层关闭
      okText: tr('添加并设置访问', 'Add and Create Entry'),
      cancelText: tr('只添加应用', 'Add Only'),
      okButtonProps: { style: { marginRight: 80 } },
      footer: (_, { OkBtn }) => (
        <>
          <Button onClick={async () => {
            modalInstance.destroy();
            await handleAddOnly();
          }}>
            {tr('只添加应用', 'Add Only')}
          </Button>
          <OkBtn />
        </>
      ),
      onOk: async () => {
        // 添加应用并跳转到访问页面
        const result = await executeAction(
          () =>
            createApplication({
              name: `App-${ip}:${port}`,
              application_type: appType,
              ip,
              port,
              edge_id: currentRow.id,
            }),
          {
            successMessage: tr('添加应用成功', 'Application added'),
            errorMessage: tr('添加应用失败', 'Failed to add application'),
            onSuccess: (data?: API.Application) => {
              if (scanTask) {
                setScanTask({
                  ...scanTask,
                  applications: scanTask.applications.filter((a) => a !== appStr),
                });
              }
              // 跳转到访问页面，传递应用ID、名称和autoCreate参数
              if (data?.id) {
                const appName = encodeURIComponent(data.name || `App-${ip}:${port}`);
                history.push(`/proxy?application_id=${data.id}&application_name=${appName}&autoCreate=true`);
              } else {
                history.push('/proxy?autoCreate=true');
              }
            },
          },
        );
        return result;
      },
      onCancel: () => {
        // 点击关闭按钮时，不执行任何操作，只关闭对话框
        // 不做任何处理
      },
    });
  };

  const columns: ProColumns<API.Edge>[] = [
    {
      title: tr('连接器名称', 'Edge Name'),
      dataIndex: 'name',
      ellipsis: true,
      width: 150,
      fieldProps: {
        placeholder: tr('请输入连接器名称', 'Please input edge name'),
      },
    },
    {
      title: tr('所在设备', 'Device'),
      dataIndex: 'device_name',
      ellipsis: true,
      width: 150,
      render: (_, record) => record.device?.name || '-',
      renderFormItem: () => {
        return (
          <Select
            placeholder={tr('请选择设备', 'Please select device')}
            showSearch
            allowClear
            options={deviceOptions}
            filterOption={(input: string, option?: { label: string; value: string }) =>
              (option?.label ?? '').toLowerCase().includes(input.toLowerCase())
            }
            onFocus={loadDeviceOptions}
            onChange={(val) => {
              // 使用 formRef 获取表单实例并设置值
              if (formRef.current) {
                formRef.current.setFieldsValue({ device_name: val });
                // 触发表单提交
                formRef.current.submit();
              }
            }}
          />
        );
      },
    },
    {
      title: tr('描述', 'Description'),
      dataIndex: 'description',
      ellipsis: true,
      search: false,
      width: 200,
    },
    {
      title: tr('在线状态', 'Online Status'),
      dataIndex: 'online',
      width: 100,
      search: false,
      render: (_, record) => (
        <Badge
          status={record.online === 1 ? 'success' : 'default'}
          text={record.online === 1 ? tr('在线', 'Online') : tr('离线', 'Offline')}
        />
      ),
    },
    {
      title: tr('运行状态', 'Runtime Status'),
      dataIndex: 'status',
      width: 100,
      search: false,
      render: (_, record) => (
        <Tag color={record.status === 1 ? 'green' : 'default'}>
          {record.status === 1 ? tr('运行中', 'Running') : tr('已停止', 'Stopped')}
        </Tag>
      ),
    },
    {
      title: tr('创建时间', 'Created At'),
      dataIndex: 'created_at',
      valueType: 'dateTime',
      width: 170,
      search: false,
    },
    {
      title: tr('更新时间', 'Updated At'),
      dataIndex: 'updated_at',
      valueType: 'dateTime',
      width: 180,
      search: false,
      hideInTable: true, // 默认隐藏，可通过列设置显示
    },
    {
      title: tr('操作', 'Actions'),
      valueType: 'option',
      width: 180,
      fixed: 'right',
      align: 'center',
      render: (_, record) => (
        <Space>
          <a onClick={() => handleDiscoverApps(record)}>
            {tr('扫描应用', 'Scan Apps')}
          </a>
          <a onClick={() => {
            setCurrentRow(record);
            setEditModalVisible(true);
          }}>
            {tr('编辑', 'Edit')}
          </a>
          <DeleteLink
            title={tr('确定要删除这个连接器吗？', 'Delete this edge?')}
            description={tr('删除后，该连接器关联的所有应用和访问将失效', 'Related applications and entries will become invalid after deletion')}
            onConfirm={() => handleDelete(record.id)}
          />
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <div className="table-search-wrapper">
        <ProTable<API.Edge>
        headerTitle={tr('连接器列表', 'Edges')}
        actionRef={actionRef}
        formRef={formRef}
        rowKey="id"
        columns={columns}
        request={async (params) => {
          console.log('ProTable request params:', params);
          const searchParams = buildSearchParams<API.EdgeListParams>(params, ['name', 'device_name']);
          console.log('buildSearchParams result:', searchParams);
          return tableRequest(() => getEdgeList(searchParams), 'edges');
        }}
        onSubmit={(values) => {
          console.log('ProTable onSubmit:', values);
          // 触发表格刷新，此时会使用表单值
          actionRef.current?.reload();
        }}
        toolBarRender={() => [
          <CreateButton key="create" onClick={handleOpenCreateModal}>
            {tr('新建连接器', 'New Edge')}
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

      <StepsForm
        onFinish={async () => {
          setCreateModalVisible(false);
          setAccessKeys(undefined);
          reload();
          return true;
        }}
        stepsFormRender={(dom, submitter) => (
          <Modal
            title={tr('创建连接器', 'Create Edge')}
            open={createModalVisible}
            onCancel={() => {
              setCreateModalVisible(false);
              setAccessKeys(undefined);
            }}
            footer={submitter}
            width={650}
            destroyOnClose
          >
            {dom}
          </Modal>
        )}
        submitter={{
          render: (props, dom) => {
            // 最后一步时，提交按钮显示"完成"
            if (props.step === 2) {
              return dom.map((item: any) => {
                if (item.key === 'submit') {
                  return { ...item, props: { ...item.props, children: tr('完成', 'Done') } };
                }
                return item;
              });
            }
            return dom;
          },
        }}
      >
        <StepsForm.StepForm
          name="create"
          title={tr('创建连接器', 'Create Edge')}
          onFinish={async (values) => {
            // 如果已经创建了连接器，直接进入下一步，避免重复创建
            if (accessKeys) {
              return true;
            }
            try {
              const res = await createEdge({
                name: values.name,
                description: values.description,
              });
              if (res.code === 200 && res.data) {
                setAccessKeys(res.data);
                message.success(tr('连接器创建成功', 'Edge created successfully'));
                return true;
              }
              message.error(res.message || tr('创建失败', 'Create failed'));
              return false;
            } catch {
              message.error(tr('创建失败', 'Create failed'));
              return false;
            }
          }}
        >
          <ProFormText
            name="name"
            label={tr('连接器名称', 'Edge Name')}
            placeholder={tr('请输入连接器名称', 'Please input edge name')}
            rules={[{ required: true, message: tr('请输入连接器名称', 'Please input edge name') }]}
            extra={tr('名称用于标识这个连接器，建议使用有意义的名称', 'Use a meaningful name to identify this edge')}
          />
          <ProFormTextArea
            name="description"
            label={tr('描述', 'Description')}
            placeholder={tr('请输入连接器描述（可选）', 'Please input edge description (optional)')}
          />
        </StepsForm.StepForm>

        <StepsForm.StepForm
          name="install"
          title={tr('安装连接器', 'Install Edge')}
          onFinish={async () => true}
        >
          {accessKeys ? (
            <>
              <Alert
                message={tr('连接器已创建，请复制下面的安装命令在目标设备上执行', 'Edge created. Copy and run the command on target device')}
                type="success"
                showIcon
                icon={<CheckCircleOutlined />}
                className="mb-4"
              />
              <div className="space-y-4">
                <div>
                  <Text strong>Access Key:</Text>
                  <div className="bg-gray-100 p-3 rounded-lg mt-2 flex items-center justify-between">
                    <Text code className="break-all" style={{ flex: 1 }}>
                      {accessKeys.access_key}
                    </Text>
                    <Button
                      type="text"
                      icon={<CopyOutlined />}
                      onClick={() => copyToClipboard(accessKeys.access_key)}
                    />
                  </div>
                </div>
                <div>
                  <Text strong>Secret Key:</Text>
                  <div className="bg-gray-100 p-3 rounded-lg mt-2 flex items-center justify-between">
                    <Text code className="break-all" style={{ flex: 1 }}>
                      {accessKeys.secret_key}
                    </Text>
                    <Button
                      type="text"
                      icon={<CopyOutlined />}
                      onClick={() => copyToClipboard(accessKeys.secret_key)}
                    />
                  </div>
                </div>
                <div className="mt-4">
                  <Text strong>{tr('安装命令:', 'Install Command:')}</Text>
                  <Tabs
                    activeKey={installOS}
                    onChange={(key) => setInstallOS(key as 'windows' | 'other')}
                    items={[
                      {
                        key: 'other',
                        label: 'Linux / macOS',
                        children: (
                          <div className="bg-gray-100 p-3 rounded-lg mt-2">
                            <Paragraph
                              copyable
                              className="mb-0 text-sm"
                              style={{ marginBottom: 0, wordBreak: 'break-all' }}
                            >
                              {accessKeys.command || `curl -k -sSL https://49.232.250.11/install.sh | bash -s -- --access-key=${accessKeys.access_key} --secret-key=${accessKeys.secret_key} --server-http-addr=49.232.250.11 --server-edge-addr=49.232.250.11:30012`}
                            </Paragraph>
                          </div>
                        ),
                      },
                      {
                        key: 'windows',
                        label: 'Windows',
                        children: (
                          <div className="bg-gray-100 p-3 rounded-lg mt-2">
                            <Paragraph
                              copyable
                              className="mb-0 text-sm"
                              style={{ marginBottom: 0, wordBreak: 'break-all' }}
                            >
                              {(() => {
                                // 从后端命令中提取服务器地址，或使用默认值
                                let serverUrl = 'https://49.232.250.11';
                                let httpAddr = '49.232.250.11';
                                let edgeAddr = '49.232.250.11:30012';
                                
                                if (accessKeys.command) {
                                  // 从命令中提取 URL（例如：curl -k -sSL https://xxx/install.sh）
                                  const urlMatch = accessKeys.command.match(/https?:\/\/[^\s\/]+/);
                                  if (urlMatch) {
                                    serverUrl = urlMatch[0];
                                    httpAddr = serverUrl.replace(/^https?:\/\//, '');
                                    // 提取 edge 地址（--server-edge-addr=xxx）
                                    const edgeMatch = accessKeys.command.match(/--server-edge-addr=([^\s]+)/);
                                    if (edgeMatch) {
                                      edgeAddr = edgeMatch[1];
                                    }
                                  }
                                }
                                
                                // 使用 curl.exe 下载脚本，然后使用 PowerShell 执行（Windows 10+ 内置）
                                // 注意：PowerShell 中 curl 是 Invoke-WebRequest 的别名，需要使用 curl.exe
                                // 使用分号分隔命令，PowerShell 不支持 &&
                                const ps1Url = `${serverUrl}/install.ps1`;
                                return `curl.exe -fsSL "${ps1Url}" -o install.ps1; powershell -ExecutionPolicy Bypass -File install.ps1 -AccessKey "${accessKeys.access_key}" -SecretKey "${accessKeys.secret_key}" -ServerHttpAddr "${httpAddr}" -ServerEdgeAddr "${edgeAddr}"`;
                              })()}
                            </Paragraph>
                          </div>
                        ),
                      },
                    ]}
                  />
                </div>
                <Alert
                  message={tr('请妥善保管以上密钥信息，关闭后将无法再次查看', 'Keep keys safe. They cannot be viewed again after closing')}
                  type="warning"
                  showIcon
                  className="mt-4"
                />
                <div className="mt-4 text-gray-500 text-sm">
                  <p>{tr('支持的操作系统：Linux (x86_64, arm64)、Windows (x86_64)、macOS (x86_64, arm64)', 'Supported OS: Linux (x86_64, arm64), Windows (x86_64), macOS (x86_64, arm64)')}</p>
                </div>
              </div>
            </>
          ) : (
            <Result
              status="error"
              title={tr('未获取到密钥信息', 'Missing key information')}
              subTitle={tr('请返回上一步重新创建', 'Please go back and create again')}
            />
          )}
        </StepsForm.StepForm>

        <StepsForm.StepForm
          name="done"
          title={tr('完成', 'Done')}
        >
          <Result
            status="success"
            title={tr('连接器创建成功', 'Edge created successfully')}
            subTitle={tr('安装完成后，连接器将自动上线。您可以在连接器列表中查看状态。', 'After installation, edge will come online automatically. You can check status in edge list.')}
          />
        </StepsForm.StepForm>
      </StepsForm>

      <ModalForm
        title={tr('编辑连接器', 'Edit Edge')}
        open={editModalVisible}
        onOpenChange={setEditModalVisible}
        onFinish={handleEdit}
        initialValues={currentRow}
        modalProps={{ destroyOnClose: true }}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('连接器名称', 'Edge Name')}
          placeholder={tr('请输入连接器名称', 'Please input edge name')}
          rules={[{ required: true, message: tr('请输入连接器名称', 'Please input edge name') }]}
        />
        <ProFormTextArea
          name="description"
          label={tr('描述', 'Description')}
          placeholder={tr('请输入连接器描述', 'Please input description')}
        />
        <ProFormSelect
          name="status"
          label={tr('运行状态', 'Runtime Status')}
          options={[
            { label: tr('运行中', 'Running'), value: 1 },
            { label: tr('已停止', 'Stopped'), value: 2 },
          ]}
          placeholder={tr('请选择运行状态', 'Please select runtime status')}
        />
      </ModalForm>

      <Drawer
        title={`${tr('扫描应用', 'Scan Apps')} - ${currentRow?.name}`}
        width={500}
        open={discoverDrawerVisible}
        onClose={() => setDiscoverDrawerVisible(false)}
        extra={
          <Button
            icon={<ReloadOutlined />}
            onClick={handleRefreshScan}
            loading={scanning}
          >
            {tr('刷新', 'Refresh')}
          </Button>
        }
      >
        {scanning ? (
          <div className="text-center py-12">
            <Spin
              indicator={<LoadingOutlined style={{ fontSize: 32 }} spin />}
              tip={tr('正在扫描内网应用...', 'Scanning intranet applications...')}
            />
          </div>
        ) : scanTask ? (
          <>
            <div className="mb-4 flex justify-between items-center">
              <Text type="secondary">
                {tr('扫描状态', 'Scan Status')}: {scanTask.task_status === 'pending' ? tr('扫描中', 'Scanning') : scanTask.task_status === 'running' ? tr('扫描中', 'Scanning') : scanTask.task_status === 'completed' ? tr('已完成', 'Completed') : scanTask.task_status === 'failed' ? tr('失败', 'Failed') : scanTask.task_status}
                {scanTask.error && (
                  <Text type="danger" className="ml-2">{scanTask.error}</Text>
                )}
              </Text>
              {(scanTask.task_status === 'completed' || scanTask.task_status === 'failed') && (
                <Button size="small" onClick={handleRescan} loading={scanning}>
                  {tr('重新扫描', 'Rescan')}
                </Button>
              )}
            </div>
            {scanTask.applications && scanTask.applications.length > 0 ? (
              <List
                dataSource={scanTask.applications}
                renderItem={(app) => {
                  // 解析应用字符串，格式可能是 "ip:port" 或 "ip:port:protocol"
                  const parts = app.split(':');
                  const ip = parts[0];
                  const port = parseInt(parts[1], 10);
                  const protocol = parts[2] || 'tcp';
                  
                  // 根据端口推断应用类型
                  const detectApplicationTypeByPort = (port: number): string => {
                    const portToType: Record<number, string> = {
                      22: 'SSH',
                      80: 'HTTP',
                      443: 'HTTP',
                      3389: 'RDP',
                      3306: 'MySQL',
                      5432: 'PostgreSQL',
                      6379: 'Redis',
                      27017: 'MongoDB',
                    };
                    return portToType[port] || protocol.toUpperCase();
                  };
                  
                  const appType = detectApplicationTypeByPort(port);
                  const displayText = `${ip}:${port}`;
                  
                  return (
                    <List.Item
                      actions={[
                        <Button key="add" type="link" onClick={() => handleAddDiscoveredApp(app)}>
                          添加
                        </Button>,
                      ]}
                    >
                      <List.Item.Meta 
                        title={displayText} 
                        description={
                          <Space>
                            <Tag color="blue">{appType}</Tag>
                            <span>扫描到的内网服务</span>
                          </Space>
                        } 
                      />
                    </List.Item>
                  );
                }}
              />
            ) : (scanTask.task_status === 'pending' || scanTask.task_status === 'running') ? (
              <div className="text-center py-12 text-gray-400">扫描中...</div>
            ) : (
              <div className="text-center py-12 text-gray-400">未扫描到可用应用</div>
            )}
          </>
        ) : (
          <div className="text-center py-12 text-gray-400">点击刷新开始扫描</div>
        )}
      </Drawer>
    </PageContainer>
  );
};

export default ConnectorPage;
