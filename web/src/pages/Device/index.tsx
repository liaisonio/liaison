import {
  ActionType,
  ModalForm,
  PageContainer,
  ProColumns,
  ProDescriptions,
  ProFormText,
  ProFormTextArea,
  ProTable,
} from '@ant-design/pro-components';
import { Drawer, Space, Tag, Popconfirm, Badge, Tooltip, message } from 'antd';
import { DesktopOutlined } from '@ant-design/icons';
import { useRef, useState } from 'react';
import { getDeviceList, getDeviceDetail, updateDevice, deleteDevice } from '@/services/api';
import { executeAction, tableRequest } from '@/utils/request';
import { defaultPagination, defaultSearch, buildSearchParams } from '@/utils/tableConfig';
import { formatMBSize } from '@/utils/format';
import { useI18n } from '@/i18n';

const DevicePage: React.FC = () => {
  const { tr, locale } = useI18n();
  const actionRef = useRef<ActionType>();
  const [detailDrawerVisible, setDetailDrawerVisible] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [currentRow, setCurrentRow] = useState<API.Device>();
  const [detailLoading, setDetailLoading] = useState(false);

  const reload = () => actionRef.current?.reload();

  const handleViewDetail = async (record: API.Device) => {
    setDetailDrawerVisible(true);
    setDetailLoading(true);
    try {
      const res = await getDeviceDetail(record.id);
      if (res.code === 200 && res.data) {
        setCurrentRow(res.data);
      } else {
        setCurrentRow(record);
      }
    } catch {
      setCurrentRow(record);
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEdit = async (values: any) => {
    if (!currentRow?.id) return false;
    return executeAction(
      () => updateDevice(currentRow.id, {
        name: values.name,
        description: values.description,
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

  const handleDelete = async (id: number, online?: number) => {
    // 如果设备在线，不允许删除
    if (online === 1) {
      message.warning(tr('在线设备不允许删除，请先断开连接', 'Online devices cannot be deleted'));
      return;
    }
    await executeAction(() => deleteDevice(id), {
      successMessage: tr('删除成功', 'Deleted successfully'),
      errorMessage: tr('删除失败', 'Delete failed'),
      onSuccess: reload,
    });
  };

  const columns: ProColumns<API.Device>[] = [
    {
      title: tr('设备名称', 'Device Name'),
      dataIndex: 'name',
      width: 200,
      ellipsis: true,
      fieldProps: {
        placeholder: tr('请输入设备名称', 'Please input device name'),
      },
      render: (_, record) => (
        <Space>
          <DesktopOutlined />
          <span>{record.name || `${tr('设备', 'Device')}-${record.id}`}</span>
        </Space>
      ),
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
      title: tr('操作系统', 'OS'),
      dataIndex: 'os',
      width: 120,
      search: false,
    },
    {
      title: tr('版本', 'Version'),
      dataIndex: 'version',
      width: 180,
      search: false,
    },
    {
      title: 'CPU',
      dataIndex: 'cpu',
      width: 80,
      search: false,
      render: (cpu) => <Tag>{cpu} {tr('核', 'cores')}</Tag>,
    },
    {
      title: tr('内存', 'Memory'),
      dataIndex: 'memory',
      width: 100,
      search: false,
      render: (memory) => formatMBSize(memory as number),
    },
    {
      title: tr('磁盘', 'Disk'),
      dataIndex: 'disk',
      width: 100,
      search: false,
      render: (disk) => formatMBSize(disk as number),
    },
    {
      title: tr('网卡', 'Network Interfaces'),
      dataIndex: 'interfaces',
      search: false,
      width: 250,
      ellipsis: true,
      render: (_, record) => {
        const interfaces = record.interfaces;
        if (!interfaces || !Array.isArray(interfaces) || interfaces.length === 0) return '-';
        return (
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
            {interfaces.map((iface, index) => {
              if (!iface || !iface.name) return null;
              // 过滤出 IPv4 地址
              const ipv4 = Array.isArray(iface.ip)
                ? iface.ip.filter((ip: string) => ip && !ip.includes(':'))
                : [];
              return (
                <Tag key={index} style={{ margin: 0 }}>
                  {iface.name}: {ipv4.length > 0 ? ipv4.join(', ') : '-'}
                </Tag>
              );
            }).filter(Boolean)}
          </div>
        );
      },
    },
    {
      title: tr('网卡IP', 'NIC IP'),
      dataIndex: 'ip',
      hideInTable: true,
      tooltip: locale === 'en-US' ? 'Supports searching any NIC IP address' : '支持搜索设备的任意网卡IP地址',
      fieldProps: {
        placeholder: locale === 'en-US' ? 'Please input NIC IP' : '请输入网卡IP',
      },
    },
    {
      title: tr('描述', 'Description'),
      dataIndex: 'description',
      width: 200,
      ellipsis: true,
      search: false,
    },
    {
      title: tr('更新时间', 'Updated At'),
      dataIndex: 'updated_at',
      valueType: 'dateTime',
      width: 180,
      search: false,
    },
    {
      title: tr('操作', 'Actions'),
      valueType: 'option',
      width: 150,
      fixed: 'right',
      align: 'center',
      render: (_, record) => (
        <Space size="small">
          <a onClick={() => handleViewDetail(record)}>
            {tr('详情', 'Detail')}
          </a>
          <a onClick={() => {
            setCurrentRow(record);
            setEditModalVisible(true);
          }}>
            {tr('编辑', 'Edit')}
          </a>
          {record.online === 1 ? (
            <Tooltip title={tr('在线设备不允许删除，请先断开连接', 'Online devices cannot be deleted')}>
              <a style={{ color: '#d9d9d9', cursor: 'not-allowed' }}>
                {tr('删除', 'Delete')}
              </a>
            </Tooltip>
          ) : (
            <Popconfirm
              title={tr('确定要删除这个设备吗？', 'Delete this device?')}
              description={tr('删除后无法恢复，请谨慎操作', 'This action cannot be undone')}
              onConfirm={() => handleDelete(record.id, record.online)}
              okText={tr('确定', 'Confirm')}
              cancelText={tr('取消', 'Cancel')}
              okButtonProps={{ danger: true }}
            >
              <a style={{ color: '#ff4d4f' }}>
                {tr('删除', 'Delete')}
              </a>
            </Popconfirm>
          )}
        </Space>
      ),
    },
  ];

  return (
    <PageContainer>
      <div className="table-search-wrapper">
        <ProTable<API.Device>
        headerTitle={tr('设备列表', 'Devices')}
        actionRef={actionRef}
        rowKey="id"
        columns={columns}
        request={async (params) => {
          const searchParams = buildSearchParams<API.DeviceListParams>(params, ['name', 'ip']);
          return tableRequest(() => getDeviceList(searchParams), 'devices');
        }}
        pagination={defaultPagination}
        search={{
          ...defaultSearch,
          labelWidth: 'auto',
        }}
        scroll={{ x: 'max-content' }}
      />
      </div>

      <Drawer
        title={tr('设备详情', 'Device Detail')}
        width={600}
        open={detailDrawerVisible}
        onClose={() => setDetailDrawerVisible(false)}
        loading={detailLoading}
      >
        {currentRow && (
          <ProDescriptions<API.Device>
            column={1}
            dataSource={currentRow}
            columns={[
              { title: tr('设备 ID', 'Device ID'), dataIndex: 'id', copyable: true },
              { title: tr('设备名称', 'Device Name'), dataIndex: 'name' },
              { title: tr('操作系统', 'OS'), dataIndex: 'os' },
              { title: tr('版本', 'Version'), dataIndex: 'version' },
              { title: 'CPU', dataIndex: 'cpu', render: (cpu) => `${cpu} ${tr('核', 'cores')}` },
              { title: tr('内存', 'Memory'), dataIndex: 'memory', render: (memory) => formatMBSize(memory as number) },
              { title: tr('磁盘', 'Disk'), dataIndex: 'disk', render: (disk) => formatMBSize(disk as number) },
              {
                title: tr('网卡信息', 'NIC Information'),
                dataIndex: 'interfaces',
                render: (_, record) => {
                  const interfaces = record.interfaces;
                  if (!interfaces || !Array.isArray(interfaces) || interfaces.length === 0) return '-';
                  return (
                    <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                      {interfaces.map((iface, index) => {
                        if (!iface || !iface.name) return null;
                        // 过滤出 IPv4 地址
                        const ipv4 = Array.isArray(iface.ip)
                          ? iface.ip.filter((ip: string) => ip && !ip.includes(':'))
                          : [];
                        return (
                          <div key={index}>
                            <Tag>{iface.name}</Tag> {ipv4.length > 0 ? ipv4.join(', ') : '-'}
                            {iface.mac && <div style={{ fontSize: '12px', color: '#999' }}>MAC: {iface.mac}</div>}
                          </div>
                        );
                      }).filter(Boolean)}
                    </div>
                  );
                }
              },
              { title: tr('描述', 'Description'), dataIndex: 'description' },
              { title: tr('创建时间', 'Created At'), dataIndex: 'created_at', valueType: 'dateTime' },
              { title: tr('更新时间', 'Updated At'), dataIndex: 'updated_at', valueType: 'dateTime' },
            ]}
          />
        )}
      </Drawer>

      <ModalForm
        title={tr('编辑设备', 'Edit Device')}
        open={editModalVisible}
        onOpenChange={setEditModalVisible}
        onFinish={handleEdit}
        initialValues={currentRow}
        modalProps={{ destroyOnClose: true }}
        width={500}
      >
        <ProFormText
          name="name"
          label={tr('设备名称', 'Device Name')}
          placeholder={tr('请输入设备名称', 'Please input device name')}
          rules={[{ required: true, message: tr('请输入设备名称', 'Please input device name') }]}
        />
        <ProFormTextArea
          name="description"
          label={tr('描述', 'Description')}
          placeholder={tr('请输入设备描述', 'Please input description')}
        />
      </ModalForm>
    </PageContainer>
  );
};

export default DevicePage;
