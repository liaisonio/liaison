import { APP_NAME } from '@/constants';
import { useI18n } from '@/i18n';
import { createAPIToken, listAPITokens, revokeAPIToken } from '@/services/api';
import { useThemeMode } from '@/store/theme';
import { executeAction } from '@/utils/request';
import {
  BgColorsOutlined,
  CopyOutlined,
  GithubOutlined,
  GlobalOutlined,
  InfoCircleOutlined,
  KeyOutlined,
  PlusOutlined,
} from '@ant-design/icons';
import { PageContainer } from '@ant-design/pro-components';
import {
  Alert,
  App,
  Button,
  Card,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Segmented,
  Space,
  Table,
  Tabs,
  Typography,
} from 'antd';
import { useEffect, useState } from 'react';
import './index.less';

const { Title, Text, Link } = Typography;
const GITHUB_URL = 'https://github.com/liaisonio/liaison';

const SettingsPage: React.FC = () => {
  const { message } = App.useApp();
  const { tr, locale, setLocale } = useI18n();
  const { preference, setPreference } = useThemeMode();

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
      message.error(
        err?.message || tr('加载 Token 失败', 'Failed to load tokens'),
      );
    } finally {
      setTokensLoading(false);
    }
  };

  useEffect(() => {
    fetchTokens();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleCreateToken = async (values: {
    name: string;
    expires_in_days?: number;
  }) => {
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

  const items = [
    {
      key: 'preferences',
      label: (
        <span>
          <BgColorsOutlined />
          {tr('界面偏好', 'Appearance')}
        </span>
      ),
      children: (
        <div className="settings-section settings-preferences">
          <div className="settings-preference-row">
            <div className="settings-preference-copy">
              <BgColorsOutlined />
              <div>
                <strong>{tr('主题', 'Theme')}</strong>
                <span>
                  {tr(
                    '选择界面外观，系统模式会跟随设备设置。',
                    'Choose an appearance. System follows your device setting.',
                  )}
                </span>
              </div>
            </div>
            <Segmented
              value={preference}
              onChange={(value) =>
                setPreference(value as 'system' | 'light' | 'dark')
              }
              options={[
                { label: tr('跟随系统', 'System'), value: 'system' },
                { label: tr('浅色', 'Light'), value: 'light' },
                { label: tr('深色', 'Dark'), value: 'dark' },
              ]}
            />
          </div>
          <div className="settings-preference-row">
            <div className="settings-preference-copy">
              <GlobalOutlined />
              <div>
                <strong>{tr('语言', 'Language')}</strong>
                <span>
                  {tr(
                    '切换控制台的显示语言。',
                    'Switch the language used by the console.',
                  )}
                </span>
              </div>
            </div>
            <Segmented
              value={locale}
              onChange={(value) => setLocale(value as 'zh-CN' | 'en-US')}
              options={[
                { label: '中文', value: 'zh-CN' },
                { label: 'English', value: 'en-US' },
              ]}
            />
          </div>
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
                <Text strong>
                  {tr('个人访问令牌 (PAT)', 'Personal Access Tokens')}
                </Text>
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
            <Title level={4}>
              {tr('关于', 'About')} {APP_NAME}
            </Title>
            <Divider />
            <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
              <div
                style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}
              >
                <span
                  style={{
                    fontWeight: 500,
                    minWidth: 'fit-content',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {tr('产品名称:', 'Product:')}
                </span>
                <span>{APP_NAME}</span>
              </div>
              <div
                style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}
              >
                <span
                  style={{
                    fontWeight: 500,
                    minWidth: 'fit-content',
                    whiteSpace: 'nowrap',
                  }}
                >
                  GitHub:
                </span>
                <Link
                  href={GITHUB_URL}
                  target="_blank"
                  rel="noopener noreferrer"
                  style={{
                    display: 'inline-flex',
                    alignItems: 'center',
                    wordBreak: 'break-all',
                    flex: 1,
                  }}
                >
                  <GithubOutlined style={{ marginRight: 8, flexShrink: 0 }} />
                  <span>{GITHUB_URL}</span>
                </Link>
              </div>
              <div
                style={{ display: 'flex', alignItems: 'flex-start', gap: 16 }}
              >
                <span
                  style={{
                    fontWeight: 500,
                    minWidth: 'fit-content',
                    whiteSpace: 'nowrap',
                  }}
                >
                  {tr('许可证:', 'License:')}
                </span>
                <span>Apache License 2.0</span>
              </div>
            </div>
          </Card>
        </div>
      ),
    },
  ];

  return (
    <PageContainer className="settings-page">
      <Card variant="borderless" className="settings-shell">
        <Tabs items={items} tabPosition="left" className="settings-tabs" />
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
        <Form
          form={createForm}
          layout="vertical"
          onFinish={handleCreateToken}
          requiredMark={false}
        >
          <Form.Item
            name="name"
            label={tr('名称', 'Name')}
            rules={[
              {
                required: true,
                message: tr('请填写名称', 'Please enter a name'),
              },
              {
                max: 64,
                message: tr('最长 64 个字符', 'At most 64 characters'),
              },
            ]}
          >
            <Input placeholder={tr('例如: laptop-cli', 'e.g. laptop-cli')} />
          </Form.Item>
          <Form.Item
            name="expires_in_days"
            label={tr(
              '过期天数（0 或留空表示永不过期）',
              'Expires in days (0 or blank = never)',
            )}
            rules={[
              {
                type: 'integer',
                min: 0,
                max: 3650,
                message: tr(
                  '请输入 0-3650 之间的整数',
                  'Enter an integer between 0 and 3650',
                ),
              },
            ]}
          >
            <InputNumber
              min={0}
              max={3650}
              precision={0}
              style={{ width: '100%' }}
              placeholder="0"
            />
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit" loading={createLoading}>
                {tr('创建', 'Create')}
              </Button>
              <Button onClick={() => setCreateOpen(false)}>
                {tr('取消', 'Cancel')}
              </Button>
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
        <div className="settings-token-reveal">{revealed}</div>
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
