import { PageContainer } from '@ant-design/pro-components';
import {
  Card,
  Tabs,
  Form,
  Input,
  InputNumber,
  Button,
  App,
  Descriptions,
  Avatar,
  Typography,
  Divider,
  Modal,
  Table,
  Popconfirm,
  Space,
  Alert,
} from 'antd';
import {
  UserOutlined,
  LockOutlined,
  SafetyOutlined,
  GithubOutlined,
  InfoCircleOutlined,
  KeyOutlined,
  CopyOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { useEffect, useState } from 'react';
import { useModel } from '@umijs/max';
import {
  changePassword,
  createAPIToken,
  listAPITokens,
  revokeAPIToken,
} from '@/services/api';
import { executeAction } from '@/utils/request';
import { APP_NAME } from '@/constants';
import { useI18n } from '@/i18n';
import './index.less';

const { Title, Text, Link } = Typography;
const GITHUB_URL = 'https://github.com/liaisonio/liaison';

const SettingsPage: React.FC = () => {
  const { message } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const { tr } = useI18n();
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [passwordForm] = Form.useForm();

  // ── PAT state ──────────────────────────────────────────────
  const [tokens, setTokens] = useState<API.APIToken[]>([]);
  const [tokensLoading, setTokensLoading] = useState(false);
  const [createOpen, setCreateOpen] = useState(false);
  const [createLoading, setCreateLoading] = useState(false);
  const [createForm] = Form.useForm();
  // plaintext of the just-created token — shown exactly once.
  const [revealed, setRevealed] = useState<string>('');

  const fetchTokens = async () => {
    setTokensLoading(true);
    try {
      const res = await listAPITokens();
      if (res.code === 200 && res.data) {
        setTokens(res.data.tokens || []);
      }
    } catch (err: any) {
      message.error(err?.message || tr('加载 Token 失败', 'Failed to load tokens'));
    } finally {
      setTokensLoading(false);
    }
  };

  useEffect(() => {
    fetchTokens();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleCreateToken = async (values: { name: string; expires_in_days?: number }) => {
    setCreateLoading(true);
    try {
      const res = await createAPIToken({
        name: values.name,
        expires_in_days: values.expires_in_days || 0,
      });
      if (res.code === 200 && res.data?.token) {
        setRevealed(res.data.token);
        setCreateOpen(false);
        createForm.resetFields();
        fetchTokens();
      } else {
        message.error(res.message || tr('创建失败', 'Failed to create'));
      }
    } catch (err: any) {
      message.error(err?.message || tr('创建失败', 'Failed to create'));
    } finally {
      setCreateLoading(false);
    }
  };

  const handleRevokeToken = async (id: number) => {
    await executeAction(() => revokeAPIToken(id), {
      successMessage: tr('Token 已撤销', 'Token revoked'),
      errorMessage: tr('撤销失败', 'Failed to revoke'),
      onSuccess: fetchTokens,
    });
  };

  const handleChangePassword = async (values: {
    oldPassword: string;
    newPassword: string;
    confirmPassword: string;
  }) => {
    if (values.newPassword !== values.confirmPassword) {
      message.error(tr('两次输入的新密码不一致', 'New passwords do not match'));
      return;
    }

    setPasswordLoading(true);
    await executeAction(
      () =>
        changePassword({
          old_password: values.oldPassword,
          new_password: values.newPassword,
        }),
      {
        successMessage: tr('密码修改成功', 'Password changed successfully'),
        errorMessage: tr('密码修改失败', 'Failed to change password'),
        onSuccess: () => passwordForm.resetFields(),
      },
    );
    setPasswordLoading(false);
  };

  const items = [
    {
      key: 'account',
      label: (
        <span>
          <UserOutlined />
          {tr('账户信息', 'Account')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <div className="user-profile">
              <Avatar
                size={80}
                icon={<UserOutlined />}
                src="/avatar.svg"
              />
              <div className="user-info">
                <Title level={4}>
                  {initialState?.currentUser?.name || 'Admin'}
                </Title>
                <Text type="secondary">
                  {initialState?.currentUser?.email || 'default@liaison.local'}
                </Text>
              </div>
            </div>
            
            <Divider />
            
            <Descriptions
              column={{ xs: 1, sm: 1, md: 2 }}
              styles={{ label: { fontWeight: 500 } }}
            >
              <Descriptions.Item label={tr('用户名', 'Username')}>
                {initialState?.currentUser?.name || 'Admin'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('邮箱', 'Email')}>
                {initialState?.currentUser?.email || 'default@liaison.local'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('角色', 'Role')}>
                {initialState?.currentUser?.role || tr('管理员', 'Administrator')}
              </Descriptions.Item>
              <Descriptions.Item label={tr('注册时间', 'Created At')}>
                {initialState?.currentUser?.created_at || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('最后登录', 'Last Login')}>
                {initialState?.currentUser?.last_login || '-'}
              </Descriptions.Item>
              <Descriptions.Item label={tr('登录IP', 'Login IP')}>
                {initialState?.currentUser?.login_ip || '-'}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </div>
      ),
    },
    {
      key: 'password',
      label: (
        <span>
          <LockOutlined />
          {tr('修改密码', 'Password')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <div className="password-tips">
              <SafetyOutlined className="text-blue-500 text-xl mr-2" />
              <div>
                <Text strong>{tr('密码安全提示', 'Password Security Tips')}</Text>
                <br />
                <Text type="secondary">
                  {tr('建议定期修改密码，密码长度至少8位，包含字母和数字', 'Use at least 8 characters and include letters and numbers')}
                </Text>
              </div>
            </div>
            
            <Divider />
            
            <Form
              form={passwordForm}
              layout="vertical"
              onFinish={handleChangePassword}
              className="password-form"
              requiredMark={false}
            >
              <Form.Item
                name="oldPassword"
                label={tr('当前密码', 'Current Password')}
                rules={[{ required: true, message: tr('请输入当前密码', 'Please input current password') }]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请输入当前密码', 'Please input current password')}
                />
              </Form.Item>

              <Form.Item
                name="newPassword"
                label={tr('新密码', 'New Password')}
                rules={[
                  { required: true, message: tr('请输入新密码', 'Please input new password') },
                  { min: 8, message: tr('密码长度至少8位', 'Password must be at least 8 characters') },
                  {
                    pattern: /^(?=.*[A-Za-z])(?=.*\d)/,
                    message: tr('密码必须包含字母和数字', 'Password must include letters and numbers'),
                  },
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请输入新密码', 'Please input new password')}
                />
              </Form.Item>

              <Form.Item
                name="confirmPassword"
                label={tr('确认新密码', 'Confirm New Password')}
                dependencies={['newPassword']}
                rules={[
                  { required: true, message: tr('请确认新密码', 'Please confirm new password') },
                  ({ getFieldValue }) => ({
                    validator(_, value) {
                      if (!value || getFieldValue('newPassword') === value) {
                        return Promise.resolve();
                      }
                      return Promise.reject(new Error(tr('两次输入的密码不一致', 'Passwords do not match')));
                    },
                  }),
                ]}
              >
                <Input.Password
                  prefix={<LockOutlined />}
                  placeholder={tr('请再次输入新密码', 'Please input password again')}
                />
              </Form.Item>

              <Form.Item>
                <Button
                  type="primary"
                  htmlType="submit"
                  loading={passwordLoading}
                >
                  {tr('修改密码', 'Change Password')}
                </Button>
              </Form.Item>
            </Form>
          </Card>
        </div>
      ),
    },
    {
      key: 'tokens',
      label: (
        <span>
          <KeyOutlined />
          {tr('API Token', 'API Tokens')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <div className="password-tips">
              <KeyOutlined className="text-blue-500 text-xl mr-2" />
              <div>
                <Text strong>{tr('个人访问令牌 (PAT)', 'Personal Access Tokens')}</Text>
                <br />
                <Text type="secondary">
                  {tr(
                    '用于 CLI / 脚本调用 API。每个 token 只会明文显示一次，请妥善保管。',
                    'For CLI / script API access. Each token is shown in plaintext once — copy it immediately.',
                  )}
                </Text>
              </div>
            </div>
            <Divider />
            <Space style={{ marginBottom: 16 }}>
              <Button
                type="primary"
                icon={<PlusOutlined />}
                onClick={() => setCreateOpen(true)}
              >
                {tr('新建 Token', 'Create token')}
              </Button>
            </Space>
            <Table<API.APIToken>
              rowKey="id"
              loading={tokensLoading}
              dataSource={tokens}
              pagination={false}
              columns={[
                { title: tr('名称', 'Name'), dataIndex: 'name', key: 'name' },
                {
                  title: tr('前缀', 'Prefix'),
                  dataIndex: 'token_prefix',
                  key: 'token_prefix',
                  render: (v: string) => <code>{v}…</code>,
                },
                {
                  title: tr('创建时间', 'Created'),
                  dataIndex: 'created_at',
                  key: 'created_at',
                },
                {
                  title: tr('最后使用', 'Last used'),
                  key: 'last_used',
                  render: (_: unknown, r) => (
                    <span>
                      {r.last_used_at || '-'}
                      {r.last_used_ip ? ` (${r.last_used_ip})` : ''}
                    </span>
                  ),
                },
                {
                  title: tr('过期时间', 'Expires'),
                  dataIndex: 'expires_at',
                  key: 'expires_at',
                  render: (v?: string) => v || tr('永不过期', 'Never'),
                },
                {
                  title: tr('操作', 'Actions'),
                  key: 'actions',
                  render: (_: unknown, r) => (
                    <Popconfirm
                      title={tr('撤销此 Token？', 'Revoke this token?')}
                      description={tr(
                        '撤销后使用此 Token 的客户端将立即失败。',
                        'Clients using this token will stop working immediately.',
                      )}
                      okText={tr('撤销', 'Revoke')}
                      cancelText={tr('取消', 'Cancel')}
                      okButtonProps={{ danger: true }}
                      onConfirm={() => handleRevokeToken(r.id)}
                    >
                      <Button danger size="small">
                        {tr('撤销', 'Revoke')}
                      </Button>
                    </Popconfirm>
                  ),
                },
              ]}
            />
          </Card>
        </div>
      ),
    },
    {
      key: 'about',
      label: (
        <span>
          <InfoCircleOutlined />
          {tr('关于', 'About')}
        </span>
      ),
      children: (
        <div className="settings-section">
          <Card variant="borderless">
            <Title level={4}>{tr('关于', 'About')} {APP_NAME}</Title>
            <Divider />
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
                <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>{tr('产品名称:', 'Product:')}</span>
                <span>{APP_NAME}</span>
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
                <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>GitHub:</span>
                <Link 
                  href={GITHUB_URL} 
                  target="_blank" 
                  rel="noopener noreferrer"
                  style={{ 
                    display: 'inline-flex',
                    alignItems: 'center',
                    wordBreak: 'break-all',
                    flex: 1
                  }}
                >
                  <GithubOutlined style={{ marginRight: 8, flexShrink: 0 }} />
                  <span>{GITHUB_URL}</span>
                </Link>
              </div>
              <div style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}>
                <span style={{ fontWeight: 500, minWidth: 'fit-content', whiteSpace: 'nowrap' }}>{tr('许可证:', 'License:')}</span>
                <span>Apache License 2.0</span>
              </div>
            </div>
          </Card>
        </div>
      ),
    },
  ];

  return (
    <PageContainer>
      <Card variant="borderless">
        <Tabs
          items={items}
          tabPosition="left"
          className="settings-tabs"
        />
      </Card>

      {/* Create-token modal */}
      <Modal
        title={tr('新建 API Token', 'Create API Token')}
        open={createOpen}
        onCancel={() => {
          setCreateOpen(false);
          createForm.resetFields();
        }}
        footer={null}
        destroyOnClose
      >
        <Form form={createForm} layout="vertical" onFinish={handleCreateToken} requiredMark={false}>
          <Form.Item
            name="name"
            label={tr('名称', 'Name')}
            rules={[
              { required: true, message: tr('请填写名称', 'Please enter a name') },
              { max: 64, message: tr('最长 64 个字符', 'At most 64 characters') },
            ]}
          >
            <Input placeholder={tr('例如: laptop-cli', 'e.g. laptop-cli')} />
          </Form.Item>
          <Form.Item
            name="expires_in_days"
            label={tr('过期天数（0 或留空表示永不过期）', 'Expires in days (0 or blank = never)')}
          >
            <InputNumber min={0} style={{ width: '100%' }} placeholder="0" />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={createLoading}>
                {tr('创建', 'Create')}
              </Button>
              <Button onClick={() => setCreateOpen(false)}>{tr('取消', 'Cancel')}</Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* One-time reveal modal */}
      <Modal
        title={tr('保管好你的 Token', 'Save this token now')}
        open={!!revealed}
        onCancel={() => setRevealed('')}
        okText={tr('我已保存', 'I have saved it')}
        cancelButtonProps={{ style: { display: 'none' } }}
        onOk={() => setRevealed('')}
        closable={false}
        maskClosable={false}
      >
        <Alert
          type="warning"
          showIcon
          message={tr(
            '此 Token 明文仅显示一次，关闭后无法再次查看。',
            'This plaintext token is shown only once and cannot be retrieved later.',
          )}
          style={{ marginBottom: 12 }}
        />
        <div
          style={{
            fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Consolas, monospace',
            fontSize: 13,
            background: 'rgba(0,0,0,0.04)',
            border: '1px solid rgba(0,0,0,0.08)',
            borderRadius: 6,
            padding: '10px 12px',
            wordBreak: 'break-all',
            userSelect: 'all',
          }}
        >
          {revealed}
        </div>
        <div style={{ marginTop: 12, textAlign: 'right' }}>
          <Button
            icon={<CopyOutlined />}
            onClick={() => {
              navigator.clipboard.writeText(revealed);
              message.success(tr('已复制', 'Copied'));
            }}
          >
            {tr('复制', 'Copy')}
          </Button>
        </div>
      </Modal>
    </PageContainer>
  );
};

export default SettingsPage;
